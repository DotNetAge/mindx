package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	graphapi "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/gorag/v2/embedder"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	goraggograph "github.com/DotNetAge/gorag/v2/store/graph/gograph"
	govector "github.com/DotNetAge/gorag/v2/store/vector/govector"
	"github.com/DotNetAge/mindx/internal/core"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
)

var (
	mu               sync.Mutex
	capturedRequests []capturedRequest
)

func main() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		req := capturedRequest{
			URL:     r.URL.String(),
			Method:  r.Method,
			Headers: map[string]string{},
			Body:    json.RawMessage(body),
		}
		for k, v := range r.Header {
			if k == "Authorization" {
				v = []string{"Bearer ***REDACTED***"}
			}
			req.Headers[k] = strings.Join(v, ", ")
		}

		mu.Lock()
		capturedRequests = append(capturedRequests, req)
		saveFile()
		mu.Unlock()

		fmt.Printf("  [CAPTURED #%d] %s %s (%d bytes)\n", len(capturedRequests), r.Method, r.URL.Path, len(body))

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"id": "chatcmpl-intercept",
			"object": "chat.completion",
			"created": %d,
			"model": "intercepted",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "好的，我来帮你。"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150}
		}`, time.Now().Unix())
	}))
	defer server.Close()

	cfg, err := core.LoadMindxConfig(core.DefaultUserPrefsDir())
	if err != nil {
		fmt.Printf("Warning: failed to load config: %v\n", err)
	}
	app, err := core.DefaultApp(cfg)
	if err != nil {
		fmt.Printf("Error creating app: %v\n", err)
		os.Exit(1)
	}

	for _, m := range app.Models().ListRaw() {
		m.BaseURL = server.URL
	}

	agentList := app.Agents().List()

	// Pick a specific agent for testing
	agentCfg := app.Agents().Get("executive-assistant")
	if agentCfg == nil {
		if len(agentList) == 0 {
			fmt.Println("Error: no agents defined")
			os.Exit(1)
		}
		agentCfg = agentList[0]
	}

	// Isolate session storage — intercept is a one-shot debug tool and must not
	// pollute the production session directory with stale sessions.
	tmpDir, err := os.MkdirTemp("", "intercept-session-*")
	if err != nil {
		fmt.Printf("Error creating temp session dir: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	tmpStore, err := mindxses.NewFileSessionStore(tmpDir)
	if err != nil {
		fmt.Printf("Error creating temp session store: %v\n", err)
		os.Exit(1)
	}

	// Initialize GraphIndexer from real data so LocalSearch is registered
	if gi := initGraphIndexer(); gi != nil {
		app.SetGraphIndexer(gi)
		fmt.Println("LocalSearch tool enabled (GraphIndexer initialized)")
	} else {
		fmt.Println("LocalSearch tool disabled (GraphIndexer not available)")
	}

	rt, err := app.ResolveRuntime(agentCfg.Name)
	if err != nil {
		fmt.Printf("Error resolving runtime: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Agent: %s (%s)\n", agentCfg.Name, agentCfg.Role)
	fmt.Println("Sending: '帮我审查下面这个Go代码，检查安全问题和性能瓶颈'")

	s := session.NewSession("test-session", agentCfg.Name,
		session.WithStore(tmpStore),
	)

	done := make(chan struct{})
	go func() {
		result, err := rt.Ask(agentCfg.Name, "帮我审查下面这个Go代码，检查安全问题和性能瓶颈", s).Run()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Response: %s\n", result.Answer)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		fmt.Println("(Agent timed out)")
	}
}

func saveFile() {
	cwd, _ := os.Getwd()
	outPath := filepath.Join(cwd, ".tmp", "llm_requests.json")

	data, _ := json.MarshalIndent(map[string]any{
		"total":    len(capturedRequests),
		"requests": capturedRequests,
	}, "", "  ")

	_ = os.WriteFile(outPath, data, 0644)
	fmt.Printf("\r  [SAVED] %s (%d requests)    \n", outPath, len(capturedRequests))
}

type capturedRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

// initGraphIndexer initializes a GraphIndexer from the real knowledge base data
// at ~/.mindx/data. Returns nil if data is unavailable or locked (daemon running).
func initGraphIndexer() *goragindexer.GraphIndexer {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".mindx", "data")

	dbPath := filepath.Join(dataDir, "kb.db")
	if _, err := os.Stat(dbPath); err != nil {
		fmt.Printf("  [KB] graph database not found at %s\n", dbPath)
		return nil
	}

	db, err := graphapi.Open(dbPath)
	if err != nil {
		fmt.Printf("  [KB] cannot open graph database (daemon running?): %v\n", err)
		return nil
	}

	gs := graphapi.NewGraphStore(db)
	coreGS := goraggograph.WrapGraphStore(db, gs)

	modelPath := filepath.Join(dataDir, "models", "model_q4.onnx")
	if _, err := os.Stat(modelPath); err != nil {
		fmt.Printf("  [KB] embedder model not found at %s\n", modelPath)
		_ = db.Close()
		return nil
	}

	emb, err := embedder.NewChineseClipEmbedder(embedder.WithModelFile(modelPath))
	if err != nil {
		fmt.Printf("  [KB] cannot initialize embedder: %v\n", err)
		_ = db.Close()
		return nil
	}

	kbVecDir := filepath.Join(dataDir, "kb-vectors")
	kbVS, err := govector.NewStore(
		govector.WithCollection("kb_sem"),
		govector.WithDimension(emb.Dim()),
		govector.WithDBPath(filepath.Join(kbVecDir, "kb.db")),
		govector.WithHNSW(true),
	)
	if err != nil {
		fmt.Printf("  [KB] cannot open vector store: %v\n", err)
		_ = db.Close()
		return nil
	}

	llmModelCfg := goragindexer.ModelConfig{
		APIKey: "intercept-skip",
		Model:  "placeholder",
	}

	gi := goragindexer.New(llmModelCfg, emb, kbVS, coreGS)
	fmt.Printf("  [KB] GraphIndexer initialized (dim=%d, db=%s)\n", emb.Dim(), dbPath)
	return gi
}
