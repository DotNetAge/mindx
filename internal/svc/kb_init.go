package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goharnesssession "github.com/DotNetAge/goharness/session"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	govector "github.com/DotNetAge/gorag/v2/store/vector/govector"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/indexing"
	"github.com/DotNetAge/mindx/pkg/logging"
)

// newKBStack creates the full knowledge-base stack: GraphIndexer, RegionIndexer,
// FileWatchService, and wires the VersionRecorder. Called from both NewDaemon
// and ensureGraphIndexer to eliminate duplicate initialization logic.
func newKBStack(
	emb goragcore.Embedder,
	coreGS goragcore.GraphStore,
	llmModelCfg *goragindexer.ModelConfig,
	dataDir string,
	logger logging.Logger,
	tokenUsageStore goharnesssession.TokenUsageStore,
	watchListStore *indexing.WatchListStore,
	indexStateStore *indexing.IndexStateStore,
	app *core.App,
) (graphIndexer *goragindexer.GraphIndexer, kbWatch *indexing.FileWatchService, err error) {

	// ── 1. Load entity-defs.json ──────────────────────────────────
	var entityDefs []goragindexer.EntityDef
	entityDefsPath := filepath.Join(dataDir, "entity-defs.json")
	if etData, etErr := os.ReadFile(entityDefsPath); etErr == nil {
		var etFile struct {
			Types []struct {
				Name   string `json:"name"`
				Desc   string `json:"desc"`
				Prompt string `json:"prompt,omitempty"`
				Schema string `json:"schema,omitempty"`
			} `json:"types"`
		}
		if json.Unmarshal(etData, &etFile) == nil {
			for _, t := range etFile.Types {
				if t.Name != "" {
					prompt := t.Prompt
					if prompt == "" {
						prompt = "**" + t.Name + "** — " + t.Desc
					}
					entityDefs = append(entityDefs, goragindexer.EntityDef{
						Prompt: prompt,
						Schema: t.Schema,
					})
				}
			}
		}
		logger.Info("memory: loaded saved entity tags from file", "path", entityDefsPath, "count", len(entityDefs))
	}

	// ── 2. Create KB vector store ─────────────────────────────────
	kbVecDir := filepath.Join(dataDir, "kb-vectors")
	if mkErr := os.MkdirAll(kbVecDir, 0755); mkErr != nil {
		return nil, nil, fmt.Errorf("KB vector directory creation failed: %w", mkErr)
	}

	kbVS, vsErr := govector.NewStore(
		govector.WithCollection("kb_sem"),
		govector.WithDimension(emb.Dim()),
		govector.WithDBPath(filepath.Join(kbVecDir, "kb.db")),
		govector.WithHNSW(true),
	)
	if vsErr != nil {
		return nil, nil, fmt.Errorf("KB vector store creation failed: %w", vsErr)
	}

	// ── 3. Create GraphIndexer ─────────────────────────────────────
	var graphOpts []goragindexer.GraphOption
	graphOpts = append(graphOpts, goragindexer.WithLogger(logger))
	if len(entityDefs) > 0 {
		graphOpts = append(graphOpts, goragindexer.WithSchemas(entityDefs...))
	}

	gi := goragindexer.New(
		*llmModelCfg,
		emb,
		kbVS,
		coreGS,
		graphOpts...,
	)
	logger.Info("GraphIndexer initialized for knowledge base",
		"vector_dim", emb.Dim(),
		"vec_db", filepath.Join(kbVecDir, "kb.db"),
	)

	// ── 4. Create RegionIndexer ────────────────────────────────────
	regionIndexer := goragindexer.NewRegionIndexer(
		*llmModelCfg,
		emb,
		kbVS,
		goragindexer.RegionWithLogger(logger),
		goragindexer.RegionWithGraphStore(coreGS),
	)
	logger.Info("RegionIndexer initialized for knowledge base")

	// ── 5. Create FileWatchService ─────────────────────────────────
	kw := indexing.NewFileWatchService(
		gi,
		regionIndexer,
		watchListStore,
		indexStateStore,
		filepath.Join(dataDir, "kb-cache"),
		logger,
		tokenUsageStore,
		llmModelCfg.Model,
	)
	logger.Info("FileWatchService configured (backed by GraphIndexer)",
		"cache_dir", filepath.Join(dataDir, "kb-cache"),
	)

	// ── 6. Wire VersionRecorder ────────────────────────────────────
	kw.VersionRecorder = func(absPath string) {
		if app.SessDB() == nil || app.FileVersions() == nil {
			return
		}
		sessions, listErr := app.SessDB().ListSessions(context.Background())
		if listErr != nil {
			return
		}
		for _, s := range sessions {
			if s.ProjectDir == "" || !strings.HasPrefix(absPath, s.ProjectDir) {
				continue
			}
			if s.SessionDir != "" {
				_ = app.FileVersions().Record(s.SessionDir, absPath)
			}
		}
	}

	return gi, kw, nil
}
