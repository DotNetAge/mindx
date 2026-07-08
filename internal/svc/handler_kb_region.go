package svc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	goragcore "github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// handleKBCheckRegionHealth checks whether the Region graph for projectDir is
// complete.  Returns one of:
//
//	{ "health": "no_data" }       — zero chunks found, nothing to repair
//	{ "health": "healthy" }       — Region node exists with proper structure
//	{ "health": "needs_repair" }  — chunks exist but Region node is missing
func (d *Daemon) handleKBCheckRegionHealth(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.KBCheckRegionHealthParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	if d.graphIndexer == nil {
		return nil, fmt.Errorf("knowledge base not available")
	}

	absDir, err := filepath.Abs(p.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project dir: %w", err)
	}
	absDir = filepath.Clean(absDir)
	regionID := fmt.Sprintf("%x", sha256.Sum256([]byte(absDir)))

	// 1. Query vectorDB for chunks with this region_id
	vectors, _, err := d.graphIndexer.VectorDB().ListFiltered(context.Background(), 0, 1, []goragcore.FilterCondition{
		{Key: "region_id", Type: "exact", Value: regionID},
	})
	if err != nil {
		d.logger.Warn("kb.check_region_health: vectorDB query failed", "error", err)
		return nil, fmt.Errorf("query vectorDB: %w", err)
	}

	// No chunks at all → no data to repair
	if len(vectors) == 0 {
		return map[string]any{"health": "no_data"}, nil
	}

	// 2. Check if the project-level Region node exists in graphDB
	if d.graphStore == nil {
		return map[string]any{"health": "needs_repair"}, nil
	}

	allNodes, err := d.graphStore.ListNodes()
	if err != nil {
		d.logger.Warn("kb.check_region_health: listNodes failed", "error", err)
		return map[string]any{"health": "needs_repair"}, nil
	}

	projectDirPrefix := absDir
	hasProjectRegion := false
	for _, n := range allNodes {
		isRegion := false
		for _, l := range n.Labels {
			if l == "Region" {
				isRegion = true
				break
			}
		}
		if !isRegion {
			continue
		}
		dirV, ok := n.GetProperty("dir")
		if !ok {
			continue
		}
		dirStr, _ := dirV.InterfaceValue().(string)
		if dirStr == projectDirPrefix {
			hasProjectRegion = true
			break
		}
	}

	if !hasProjectRegion {
		return map[string]any{"health": "needs_repair"}, nil
	}

	return map[string]any{"health": "healthy"}, nil
}

// ---------------------------------------------------------------------------
// kb.repair_region — generate Region chunks + graph nodes for projectDir
// ---------------------------------------------------------------------------

// handleKBRepairRegion calls RegionIndexer.IndexRegion followed by
// GraphIndexer.AddFile to fill in missing Region-level chunks and graph nodes
// for the given projectDir.
func (d *Daemon) handleKBRepairRegion(ctx context.Context, params json.RawMessage) (any, error) {
	var p rpc.KBRepairRegionParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	if d.regionIndexer == nil {
		return nil, fmt.Errorf("region indexer not available")
	}
	if d.graphIndexer == nil {
		return nil, fmt.Errorf("knowledge base not available")
	}
	if d.graphStore == nil {
		return nil, fmt.Errorf("graph store not available")
	}

	absDir, err := filepath.Abs(p.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project dir: %w", err)
	}
	absDir = filepath.Clean(absDir)

	d.logger.Info("kb.repair_region: starting", "project_dir", absDir)

	// 1. Generate .README.md + Region graph nodes/edges
	result, riErr := d.regionIndexer.IndexRegion(ctx, absDir)
	if riErr != nil {
		d.logger.Error("kb.repair_region: IndexRegion failed", riErr, "project_dir", absDir)
		return nil, fmt.Errorf("index region: %w", riErr)
	}
	if result == nil || result.RegionFilePath == "" {
		d.logger.Info("kb.repair_region: no content to index", "project_dir", absDir)
		return map[string]any{
			"status":  "no_change",
			"message": "no content to index for this directory",
		}, nil
	}

	d.logger.Info("kb.repair_region: .README.md generated, now indexing it",
		"path", result.RegionFilePath)

	// 2. Index the generated .README.md through GraphIndexer
	regionID := fmt.Sprintf("%x", sha256.Sum256([]byte(absDir)))
	fileCtx := goragindexer.WithRegionID(ctx, regionID)

	indexStart := time.Now()
	chunks, idxErr := d.graphIndexer.AddFile(fileCtx, result.RegionFilePath)
	if idxErr != nil {
		d.logger.Error("kb.repair_region: AddFile failed", idxErr,
			"path", result.RegionFilePath)
		return nil, fmt.Errorf("index region file: %w", idxErr)
	}

	elapsed := time.Since(indexStart).Milliseconds()
	d.logger.Info("kb.repair_region: completed",
		"project_dir", absDir,
		"chunks", len(chunks),
		"elapsed_ms", elapsed)

	return map[string]any{
		"status":      "repaired",
		"chunks":      len(chunks),
		"region_file": result.RegionFilePath,
	}, nil
}
