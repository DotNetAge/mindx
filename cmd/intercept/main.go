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

	"github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/goharness/session"
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
		fmt.Fprintf(w, `{
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

	app, err := core.DefaultApp(nil)
	if err != nil {
		fmt.Printf("Error creating app: %v\n", err)
		os.Exit(1)
	}

	for _, m := range app.Models().List() {
		m.BaseURL = server.URL
	}

	agentList := app.Agents().List()
	if len(agentList) == 0 {
		fmt.Println("Error: no agents defined")
		os.Exit(1)
	}

	var agentCfg *config.AgentConfig
	var modelCfg *config.ModelConfig
	for _, a := range agentList {
		modelCfg = app.Models().Get(a.Model)
		if modelCfg != nil {
			agentCfg = a
			break
		}
	}

	if agentCfg == nil {
		fmt.Println("Error: no agent has a model matching the registry")
		os.Exit(1)
	}

	// Isolate session storage — intercept is a one-shot debug tool and must not
	// pollute the production session directory with stale sessions.
	tmpDir, err := os.MkdirTemp("", "intercept-session-*")
	if err != nil {
		fmt.Printf("Error creating temp session dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)
	tmpStore, err := mindxses.NewFileSessionStore(tmpDir)
	if err != nil {
		fmt.Printf("Error creating temp session store: %v\n", err)
		os.Exit(1)
	}

	rt, err := app.ResolveRuntime(agentCfg.Name)
	if err != nil {
		fmt.Printf("Error resolving runtime: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Agent: %s (%s), Model: %s\n", agentCfg.Name, agentCfg.Role, modelCfg.Name)
	fmt.Println("Sending: '我想开发一个AI系统'")

	s := session.NewSession("test-session", agentCfg.Name,
		session.WithStore(tmpStore),
	)

	done := make(chan struct{})
	go func() {
		result, err := rt.Ask(agentCfg.Name, "我想开发一个AI系统", s).Run()
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

	os.WriteFile(outPath, data, 0644)
	fmt.Printf("\r  [SAVED] %s (%d requests)    \n", outPath, len(capturedRequests))
}

type capturedRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}
