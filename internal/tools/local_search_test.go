package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	graphapi "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gorag/v2/embedder"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	goraggograph "github.com/DotNetAge/gorag/v2/store/graph/gograph"
	govector "github.com/DotNetAge/gorag/v2/store/vector/govector"
)

// ── Assertion helpers ─────────────────────────────────────────────────────────────

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain:\n  %q\n\ngot:\n%s", needle, haystack)
	}
}

// ── Real-data integration test ────────────────────────────────────────────────────

// TestQuickSearchWithRealData initializes a GraphIndexer from the real knowledge base
// at ~/.mindx/data and verifies that Search produces correctly formatted output.
//
// Skips if:
//   - the daemon is running (Pebble DB locked)
//   - ONNX Runtime is not available
//   - the model file is missing
//
// Run:  GOROOT=/usr/local/Cellar/go/1.26.4/libexec go test ./internal/tools/ -v -run TestQuickSearchWithRealData
// Stop daemon first: kill $(lsof -ti:8765) or mindx stop
func TestQuickSearchWithRealData(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal("cannot determine home directory:", err)
	}
	dataDir := filepath.Join(home, ".mindx", "data")

	// ── 1. Open graph DB (Pebble) ─────────────────────────────────────
	dbPath := filepath.Join(dataDir, "kb.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("graph database not found at %s (is the KB initialized?)", dbPath)
	}

	db, err := graphapi.Open(dbPath)
	if err != nil {
		t.Skipf("cannot open graph database — daemon may be running: %v\nStop the daemon first then re-run this test.", err)
	}
	gs := graphapi.NewGraphStore(db)
	coreGS := goraggograph.WrapGraphStore(db, gs)

	// ── 2. Load embedder (Chinese CLIP ONNX) ──────────────────────────
	modelPath := filepath.Join(dataDir, "models", "model_q4.onnx")
	if _, err := os.Stat(modelPath); err != nil {
		_ = db.Close()
		t.Skipf("embedder model not found at %s", modelPath)
	}

	emb, err := embedder.NewChineseClipEmbedder(embedder.WithModelFile(modelPath))
	if err != nil {
		_ = db.Close()
		t.Skipf("cannot initialize embedder (ONNX Runtime missing or incompatible): %v", err)
	}

	t.Logf("embedder dimension: %d", emb.Dim())

	// ── 3. Open vector store (read-only so test can run alongside daemon) ──
	kbVecDir := filepath.Join(dataDir, "kb-vectors")
	vecDBPath := filepath.Join(kbVecDir, "kb.db")

	kbVS, err := govector.NewStore(
		govector.WithCollection("kb_sem"),
		govector.WithDimension(emb.Dim()),
		govector.WithDBPath(vecDBPath),
		govector.WithHNSW(true),
	)
	if err != nil {
		_ = db.Close()
		t.Skipf("vector store open failed: %v", err)
	}

	// ── 4. Create GraphIndexer ────────────────────────────────────────
	// ModelConfig is required by the constructor but not used for Search
	// when TextQuery is set to "" (prevents LLM text→Cypher path).
	llmModelCfg := goragindexer.ModelConfig{
		APIKey: "test-skip",
		Model:  "placeholder",
	}

	gi := goragindexer.New(llmModelCfg, emb, kbVS, coreGS)

	// Cleanup
	defer func() { _ = db.Close() }()
	// kbVS is backed by read-only bbolt; no explicit Close needed

	// ── 5. Create QuickSearch and query ───────────────────────────────
	qs := &QuickSearch{indexer: gi}

	ctx := context.Background()

	// Try a few queries to get results
	queries := []string{
		"需求说明书",
		"系统设计",
		"电子系统",
		"产品",
	}

	var output string
	var found bool
	for _, q := range queries {
		result, err := qs.Execute(ctx, map[string]any{
			"query": q,
			"limit": float64(5),
		})
		if err != nil {
			t.Logf("query %q failed: %v", q, err)
			continue
		}
		if result == "" || result == nil {
			t.Logf("query %q returned empty results", q)
			continue
		}
		output = result.(string)
		if strings.Contains(output, "[ID:") {
			found = true
			t.Logf("query %q returned: %.120s...", q, output)
			break
		}
	}

	if !found {
		t.Fatal("all queries returned empty or no-ID results — check that the KB is properly indexed")
	}

	// ── 6. Verify output format structure ──────────────────────────────

	// Root header
	assertContains(t, output, "## Search Result")

	// Each result line contains [summary], [file:], [ID:]
	lines := strings.Split(output, "\n")
	idCount := 0
	fileCount := 0
	for _, line := range lines {
		if strings.Contains(line, "[ID:") {
			idCount++
		}
		if strings.Contains(line, "[file:") {
			fileCount++
		}
	}

	t.Logf("results: %d ID markers, %d file markers", idCount, fileCount)

	if idCount == 0 {
		t.Error("no [ID:xxx] markers found in output — format may be broken")
	}
	if fileCount == 0 {
		t.Error("no [file:xxx] markers found in output — format may be broken")
	}

	// Verify line format: [summary] - [file:path][POS:...][ID:xxx][TAGS:...]
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		// Must start with [ and contain ] - [
		if !strings.Contains(line, "] - [") {
			continue
		}
		// Must contain [ID:
		if !strings.Contains(line, "[ID:") {
			t.Errorf("result line missing [ID:]: %s", line)
		}
	}

	// Footer
	assertContains(t, output, "QuickSearch clue result")

	// If there are entity tables, verify table structure
	if strings.Contains(output, "### Relevant Nodes") {
		assertContains(t, output, "| ID | Name | Type")
	}

	t.Logf("--- Full output ---\n%s\n--- End ---", output)
}
