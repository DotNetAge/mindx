package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
)

// mockGateway implements gwCaller for testing.
type mockGateway struct {
	mu      sync.Mutex
	callFn  func(ctx context.Context, method string, params any) (json.RawMessage, error)
	closeFn func() error
	calls   []methodCall
}

type methodCall struct {
	Method string `json:"method"`
	Params any    `json:"params"`
}

func newMockGateway() *mockGateway {
	return &mockGateway{
		callFn: func(_ context.Context, _ string, _ any) (json.RawMessage, error) {
			return json.RawMessage(`"ok"`), nil
		},
		closeFn: func() error { return nil },
	}
}

func (m *mockGateway) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	m.mu.Lock()
	m.calls = append(m.calls, methodCall{Method: method, Params: params})
	fn := m.callFn
	m.mu.Unlock()
	return fn(ctx, method, params)
}

func (m *mockGateway) Close() error { return m.closeFn() }

func (m *mockGateway) lastCall() (method string, params any, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return "", nil, false
	}
	c := m.calls[len(m.calls)-1]
	return c.Method, c.Params, true
}

// testRPC is a helper to test a typed RPC method with params.
// It calls fn, verifies the RPC method name, unmarshals the params into wantParams,
// and returns the result.
func testRPC[P any](t *testing.T, c *Client, m *mockGateway, wantMethod string, wantParams P, fn func() (json.RawMessage, error)) json.RawMessage {
	t.Helper()
	result, err := fn()
	if err != nil {
		t.Fatalf("%s error = %v", wantMethod, err)
	}

	method, raw, ok := m.lastCall()
	if !ok {
		t.Fatalf("%s: no RPC call made", wantMethod)
	}
	if method != wantMethod {
		t.Errorf("method = %q, want %q", method, wantMethod)
	}

	// Unmarshal captured params and compare
	var got P
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("%s: marshal captured params: %v", wantMethod, err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("%s: unmarshal captured params: %v", wantMethod, err)
	}
	if !reflect.DeepEqual(got, wantParams) {
		t.Errorf("%s: params = %+v, want %+v", wantMethod, got, wantParams)
	}

	return result
}

// testRPCNoParams tests a method that takes no parameters (passes nil to the RPC call).
func testRPCNoParams(t *testing.T, c *Client, m *mockGateway, wantMethod string, fn func() (json.RawMessage, error)) json.RawMessage {
	t.Helper()
	result, err := fn()
	if err != nil {
		t.Fatalf("%s error = %v", wantMethod, err)
	}

	method, _, ok := m.lastCall()
	if !ok {
		t.Fatalf("%s: no RPC call made", wantMethod)
	}
	if method != wantMethod {
		t.Errorf("method = %q, want %q", method, wantMethod)
	}

	return result
}

// ============================================================================
// Client core tests
// ============================================================================

func TestClose_Idempotent(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	var closeCount int
	m.closeFn = func() error {
		closeCount++
		return nil
	}

	if err := c.Close(); err != nil {
		t.Errorf("first Close() error = %v", err)
	}
	if closeCount != 1 {
		t.Errorf("expected gw.Close() called once, got %d calls", closeCount)
	}

	if err := c.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
	if closeCount != 1 {
		t.Errorf("expected gw.Close() still called once after second Close(), got %d calls", closeCount)
	}
}

func TestClose_PropagatesError(t *testing.T) {
	m := newMockGateway()
	want := errors.New("close error")
	m.closeFn = func() error { return want }

	c := &Client{gw: m}
	if err := c.Close(); err != want {
		t.Errorf("Close() error = %v, want %v", err, want)
	}
}

func TestCallWithTimeout_ReturnsResult(t *testing.T) {
	m := newMockGateway()
	m.callFn = func(_ context.Context, _ string, _ any) (json.RawMessage, error) {
		return json.RawMessage(`{"result":42}`), nil
	}

	c := &Client{gw: m}
	result, err := c.CallWithTimeout("test.method", "hello")
	if err != nil {
		t.Fatalf("CallWithTimeout error = %v", err)
	}
	if string(result) != `{"result":42}` {
		t.Errorf("result = %s, want %s", result, `{"result":42}`)
	}
}

func TestCallWithTimeout_PropagatesError(t *testing.T) {
	m := newMockGateway()
	want := errors.New("rpc failed")
	m.callFn = func(_ context.Context, _ string, _ any) (json.RawMessage, error) {
		return nil, want
	}

	c := &Client{gw: m}
	_, err := c.CallWithTimeout("test.method", nil)
	if err != want {
		t.Errorf("error = %v, want %v", err, want)
	}
}

// ============================================================================
// Session domain
// ============================================================================

func TestSessionMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Create", func(t *testing.T) {
		testRPC(t, c, m, "session.create", SessionCreateParams{
			Agent: "my-agent", ProjectDir: "/tmp/proj",
		}, func() (json.RawMessage, error) {
			return c.SessionCreate("my-agent", "/tmp/proj")
		})
	})

	t.Run("List", func(t *testing.T) {
		testRPC(t, c, m, "session.list", SessionListParams{Agent: "agent-x"}, func() (json.RawMessage, error) {
			return c.SessionList("agent-x")
		})
	})

	t.Run("Get", func(t *testing.T) {
		testRPC(t, c, m, "session.get", SessionGetParams{SessionID: "sess_123"}, func() (json.RawMessage, error) {
			return c.SessionGet("sess_123")
		})
	})

	t.Run("Delete", func(t *testing.T) {
		testRPC(t, c, m, "session.delete", SessionDeleteParams{SessionID: "sess_123"}, func() (json.RawMessage, error) {
			return c.SessionDelete("sess_123")
		})
	})

	t.Run("Meta", func(t *testing.T) {
		testRPC(t, c, m, "session.meta", SessionMetaParams{SessionID: "sess_123"}, func() (json.RawMessage, error) {
			return c.SessionMeta("sess_123")
		})
	})

	t.Run("ConfirmFiles", func(t *testing.T) {
		testRPC(t, c, m, "session.confirm_files", SessionFileActionParams{
			SessionID: "sess_123", Files: []string{"a.go", "b.go"},
		}, func() (json.RawMessage, error) {
			return c.SessionConfirmFiles("sess_123", []string{"a.go", "b.go"})
		})
	})

	t.Run("RollbackFiles", func(t *testing.T) {
		testRPC(t, c, m, "session.rollback_files", SessionFileActionParams{
			SessionID: "sess_123", Files: []string{"a.go"},
		}, func() (json.RawMessage, error) {
			return c.SessionRollbackFiles("sess_123", []string{"a.go"})
		})
	})

	t.Run("Truncate", func(t *testing.T) {
		testRPC(t, c, m, "session.truncate", SessionTruncateParams{SessionID: "sess_123"}, func() (json.RawMessage, error) {
			return c.SessionTruncate("sess_123")
		})
	})
}

// ============================================================================
// Agent domain
// ============================================================================

func TestAgentMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("List", func(t *testing.T) {
		testRPCNoParams(t, c, m, "agent.list", func() (json.RawMessage, error) {
			return c.AgentList()
		})
	})

	t.Run("Get", func(t *testing.T) {
		testRPC(t, c, m, "agent.get", AgentGetParams{Name: "agent-x"}, func() (json.RawMessage, error) {
			return c.AgentGet("agent-x")
		})
	})

	t.Run("Create", func(t *testing.T) {
		params := AgentCreateParams{
			Name: "new-agent", Role: "helper", Description: "desc",
			Model: "gpt-4", Skills: []string{"code"},
		}
		testRPC(t, c, m, "agent.create", params, func() (json.RawMessage, error) {
			return c.AgentCreate(params)
		})
	})

	t.Run("Update", func(t *testing.T) {
		params := AgentUpdateParams{Name: "agent-x", Description: "new desc"}
		testRPC(t, c, m, "agent.update", params, func() (json.RawMessage, error) {
			return c.AgentUpdate(params)
		})
	})

	t.Run("Score", func(t *testing.T) {
		params := AgentScoreParams{AgentName: "agent-x", Task: "test", Score: 85, Notes: "good"}
		testRPC(t, c, m, "agent.score", params, func() (json.RawMessage, error) {
			return c.AgentScore(params)
		})
	})

	t.Run("Reload", func(t *testing.T) {
		testRPCNoParams(t, c, m, "agent.reload", func() (json.RawMessage, error) {
			return c.AgentReload()
		})
	})
}

// ============================================================================
// Model domain
// ============================================================================

func TestModelMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("List", func(t *testing.T) {
		testRPCNoParams(t, c, m, "model.list", func() (json.RawMessage, error) {
			return c.ModelList()
		})
	})

	t.Run("Get", func(t *testing.T) {
		testRPC(t, c, m, "model.get", ModelGetParams{Name: "gpt-4"}, func() (json.RawMessage, error) {
			return c.ModelGet("gpt-4")
		})
	})

	t.Run("Switch", func(t *testing.T) {
		testRPC(t, c, m, "model.switch", ModelSwitchParams{Name: "gpt-4", Provider: "openai"}, func() (json.RawMessage, error) {
			return c.ModelSwitch("gpt-4", "openai")
		})
	})

	t.Run("Create", func(t *testing.T) {
		params := ModelCreateParams{Name: "my-model", Title: "My Model", Provider: "openai"}
		testRPC(t, c, m, "model.create", params, func() (json.RawMessage, error) {
			return c.ModelCreate(params)
		})
	})

	t.Run("Update", func(t *testing.T) {
		params := ModelUpdateParams{Name: "my-model", Title: "Updated"}
		testRPC(t, c, m, "model.update", params, func() (json.RawMessage, error) {
			return c.ModelUpdate(params)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		testRPC(t, c, m, "model.delete", ModelDeleteParams{Name: "gpt-4"}, func() (json.RawMessage, error) {
			return c.ModelDelete("gpt-4")
		})
	})
}

// ============================================================================
// Provider domain
// ============================================================================

func TestProviderMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("List", func(t *testing.T) {
		testRPCNoParams(t, c, m, "provider.list", func() (json.RawMessage, error) {
			return c.ProviderList()
		})
	})

	t.Run("Create", func(t *testing.T) {
		params := ProviderCreateParams{Name: "p1", Title: "P1", BaseURL: "https://example.com", APIKey: "key123"}
		testRPC(t, c, m, "provider.create", params, func() (json.RawMessage, error) {
			return c.ProviderCreate(params)
		})
	})

	t.Run("Update", func(t *testing.T) {
		params := ProviderUpdateParams{Name: "p1", Title: "Updated"}
		testRPC(t, c, m, "provider.update", params, func() (json.RawMessage, error) {
			return c.ProviderUpdate(params)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		testRPC(t, c, m, "provider.delete", ProviderDeleteParams{Name: "p1"}, func() (json.RawMessage, error) {
			return c.ProviderDelete("p1")
		})
	})
}

// ============================================================================
// Memory domain
// ============================================================================

func TestMemoryMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Query", func(t *testing.T) {
		testRPC(t, c, m, "memory.query", MemoryQueryParams{Query: "test", Limit: 10, MinScore: 0.5}, func() (json.RawMessage, error) {
			return c.MemoryQuery("test", 10, 0.5)
		})
	})

	t.Run("Store", func(t *testing.T) {
		testRPC(t, c, m, "memory.store", MemoryStoreParams{
			Content: "hello", Title: "t", Description: "d", Source: "s",
		}, func() (json.RawMessage, error) {
			return c.MemoryStore("hello", "t", "d", "s")
		})
	})

	t.Run("Delete", func(t *testing.T) {
		testRPC(t, c, m, "memory.delete", MemoryDeleteParams{ID: "mem_1"}, func() (json.RawMessage, error) {
			return c.MemoryDelete("mem_1")
		})
	})

	t.Run("Chunks", func(t *testing.T) {
		testRPC(t, c, m, "memory.chunks", MemoryChunksParams{Page: 1, PageSize: 20, DocID: "doc_1"}, func() (json.RawMessage, error) {
			return c.MemoryChunks(1, 20, "doc_1")
		})
	})

	t.Run("GetChunks", func(t *testing.T) {
		testRPC(t, c, m, "memory.get_chunks", MemoryGetChunksParams{DocID: "doc_1"}, func() (json.RawMessage, error) {
			return c.MemoryGetChunks("doc_1")
		})
	})

	t.Run("Count", func(t *testing.T) {
		testRPCNoParams(t, c, m, "memory.count", func() (json.RawMessage, error) {
			return c.MemoryCount()
		})
	})
}

// ============================================================================
// KB domain
// ============================================================================

func TestKBMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Search", func(t *testing.T) {
		testRPC(t, c, m, "kb.search", KBSearchParams{Query: "q", Limit: 5, MinScore: 0.7}, func() (json.RawMessage, error) {
			return c.KBSearch("q", 5, 0.7, "")
		})
	})

	t.Run("Count", func(t *testing.T) {
		testRPC(t, c, m, "kb.count", KBCountParams{Region: "/p"}, func() (json.RawMessage, error) {
			return c.KBCount("/p")
		})
	})

	t.Run("CountAll", func(t *testing.T) {
		testRPC(t, c, m, "kb.count", KBCountParams{}, func() (json.RawMessage, error) {
			return c.KBCount("")
		})
	})

	t.Run("Chunks", func(t *testing.T) {
		testRPC(t, c, m, "kb.chunks", KBChunksParams{Page: 1, PageSize: 10}, func() (json.RawMessage, error) {
			return c.KBChunks(1, 10)
		})
	})

	t.Run("Stats", func(t *testing.T) {
		testRPC(t, c, m, "kb.stats", KBStatsParams{ProjectDir: "/p"}, func() (json.RawMessage, error) {
			return c.KBStats("/p")
		})
	})

	t.Run("SyncProject", func(t *testing.T) {
		testRPC(t, c, m, "kb.sync_project", KBSyncProjectParams{ProjectDir: "/p"}, func() (json.RawMessage, error) {
			return c.KBSyncProject("/p")
		})
	})

	t.Run("FileStates", func(t *testing.T) {
		testRPC(t, c, m, "kb.file_states", KBFileStatesParams{ProjectDir: "/p"}, func() (json.RawMessage, error) {
			return c.KBFileStates("/p")
		})
	})
}

// ============================================================================
// Graph domain
// ============================================================================

func TestGraphMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Query", func(t *testing.T) {
		params := map[string]interface{}{"key": "val"}
		testRPC(t, c, m, "graph.query", GraphQueryParams{Query: "match (n)", Params: params}, func() (json.RawMessage, error) {
			return c.GraphQuery("match (n)", params)
		})
	})

	t.Run("Exec", func(t *testing.T) {
		testRPC(t, c, m, "graph.exec", GraphQueryParams{Query: "create (n)"}, func() (json.RawMessage, error) {
			return c.GraphExec("create (n)", nil)
		})
	})

	t.Run("UpsertNodes", func(t *testing.T) {
		nodes := []GraphNodeParam{{ID: "n1", Labels: []string{"Person"}}}
		testRPC(t, c, m, "graph.upsert_nodes", GraphUpsertNodesParams{Nodes: nodes}, func() (json.RawMessage, error) {
			return c.GraphUpsertNodes(nodes)
		})
	})

	t.Run("UpsertEdges", func(t *testing.T) {
		edges := []GraphEdgeParam{{FromNodeID: "n1", ToNodeID: "n2", Type: "KNOWS"}}
		testRPC(t, c, m, "graph.upsert_edges", GraphUpsertEdgesParams{Edges: edges}, func() (json.RawMessage, error) {
			return c.GraphUpsertEdges(edges)
		})
	})

	t.Run("GetNode", func(t *testing.T) {
		testRPC(t, c, m, "graph.get_node", GraphGetNodeParams{ID: "n1"}, func() (json.RawMessage, error) {
			return c.GraphGetNode("n1")
		})
	})

	t.Run("GetNeighbors", func(t *testing.T) {
		testRPC(t, c, m, "graph.get_neighbors", GraphGetNeighborsParams{
			ID: "n1", Depth: 2, Limit: 10, Types: []string{"KNOWS"},
		}, func() (json.RawMessage, error) {
			return c.GraphGetNeighbors("n1", 2, 10, []string{"KNOWS"})
		})
	})
}

// ============================================================================
// FS domain
// ============================================================================

func TestFSMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("List", func(t *testing.T) {
		testRPC(t, c, m, "fs.list", FSListParams{Path: "/tmp"}, func() (json.RawMessage, error) {
			return c.FSList("/tmp")
		})
	})

	t.Run("Read", func(t *testing.T) {
		testRPC(t, c, m, "fs.read", FSReadParams{Path: "/tmp/f"}, func() (json.RawMessage, error) {
			return c.FSRead("/tmp/f")
		})
	})

	t.Run("Write", func(t *testing.T) {
		testRPC(t, c, m, "fs.write", FSWriteParams{Path: "/tmp/f", Content: "data"}, func() (json.RawMessage, error) {
			return c.FSWrite("/tmp/f", "data")
		})
	})

	t.Run("Mkdir", func(t *testing.T) {
		testRPC(t, c, m, "fs.mkdir", FSMkdirParams{Path: "/tmp/d", All: true}, func() (json.RawMessage, error) {
			return c.FSMkdir("/tmp/d", true)
		})
	})

	t.Run("Rm", func(t *testing.T) {
		testRPC(t, c, m, "fs.rm", FSRmParams{Path: "/tmp/f", Recurse: true, Force: false}, func() (json.RawMessage, error) {
			return c.FSRm("/tmp/f", true, false)
		})
	})

	t.Run("Mv", func(t *testing.T) {
		testRPC(t, c, m, "fs.mv", FSMvParams{Src: "/tmp/a", Dst: "/tmp/b"}, func() (json.RawMessage, error) {
			return c.FSMv("/tmp/a", "/tmp/b")
		})
	})

	t.Run("Reveal", func(t *testing.T) {
		testRPC(t, c, m, "fs.reveal", FSRevealParams{Path: "/tmp"}, func() (json.RawMessage, error) {
			return c.FSReveal("/tmp")
		})
	})

	t.Run("Home", func(t *testing.T) {
		testRPCNoParams(t, c, m, "fs.home", func() (json.RawMessage, error) {
			return c.FSHome()
		})
	})
}

// ============================================================================
// Filewatch domain
// ============================================================================

func TestFilewatchMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Start", func(t *testing.T) {
		testRPCNoParams(t, c, m, "filewatch.start", func() (json.RawMessage, error) {
			return c.FilewatchStart()
		})
	})

	t.Run("Stop", func(t *testing.T) {
		testRPCNoParams(t, c, m, "filewatch.stop", func() (json.RawMessage, error) {
			return c.FilewatchStop()
		})
	})

	t.Run("Status", func(t *testing.T) {
		testRPCNoParams(t, c, m, "filewatch.status", func() (json.RawMessage, error) {
			return c.FilewatchStatus()
		})
	})
}

// ============================================================================
// Server domain
// ============================================================================

func TestServerMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Version", func(t *testing.T) {
		testRPCNoParams(t, c, m, "server.version", func() (json.RawMessage, error) {
			return c.ServerVersion()
		})
	})

	t.Run("CheckUpdate", func(t *testing.T) {
		testRPCNoParams(t, c, m, "server.check_update", func() (json.RawMessage, error) {
			return c.ServerCheckUpdate()
		})
	})

	t.Run("ApplyUpdate", func(t *testing.T) {
		testRPCNoParams(t, c, m, "server.apply_update", func() (json.RawMessage, error) {
			return c.ServerApplyUpdate()
		})
	})

	t.Run("RestartDaemon", func(t *testing.T) {
		testRPCNoParams(t, c, m, "server.restart_daemon", func() (json.RawMessage, error) {
			return c.ServerRestartDaemon()
		})
	})
}

// ============================================================================
// Token Usage domain
// ============================================================================

func TestTokenUsageMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Overview", func(t *testing.T) {
		testRPCNoParams(t, c, m, "token.usage.overview", func() (json.RawMessage, error) {
			return c.TokenUsageOverview()
		})
	})

	t.Run("Total", func(t *testing.T) {
		testRPCNoParams(t, c, m, "token.usage.total", func() (json.RawMessage, error) {
			return c.TokenUsageTotal()
		})
	})

	t.Run("Monthly", func(t *testing.T) {
		testRPC(t, c, m, "token.usage.monthly", TokenUsageMonthlyParams{Year: 2026, Month: 6}, func() (json.RawMessage, error) {
			return c.TokenUsageMonthly(2026, 6)
		})
	})

	t.Run("ByModel", func(t *testing.T) {
		testRPC(t, c, m, "token.usage.by_model", TokenUsageByModelParams{Model: "gpt-4", Year: 2026, Month: 6}, func() (json.RawMessage, error) {
			return c.TokenUsageByModel("gpt-4", 2026, 6)
		})
	})

	t.Run("Session", func(t *testing.T) {
		testRPC(t, c, m, "token.usage.session", TokenUsageSessionParams{SessionID: "sess_1"}, func() (json.RawMessage, error) {
			return c.TokenUsageSession("sess_1")
		})
	})
}

// ============================================================================
// Schedule domain
// ============================================================================

func TestScheduleMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("List", func(t *testing.T) {
		testRPCNoParams(t, c, m, "schedule.list", func() (json.RawMessage, error) {
			return c.ScheduleList()
		})
	})

	t.Run("Add", func(t *testing.T) {
		params := ScheduleAddParams{
			Agent: "a", SessionID: "s", Content: "do it", CronExpr: "* * * * *",
		}
		testRPC(t, c, m, "schedule.add", params, func() (json.RawMessage, error) {
			return c.ScheduleAdd(params)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		testRPC(t, c, m, "schedule.del", ScheduleDeleteParams{ID: "task_1"}, func() (json.RawMessage, error) {
			return c.ScheduleDelete("task_1")
		})
	})
}

// ============================================================================
// Log domain
// ============================================================================

func TestLogMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Read", func(t *testing.T) {
		testRPC(t, c, m, "log.read", LogReadParams{Offset: 0, Limit: 100, Stream: "daemon"}, func() (json.RawMessage, error) {
			return c.LogRead(0, 100, "daemon")
		})
	})

	t.Run("Clear", func(t *testing.T) {
		testRPC(t, c, m, "log.clear", LogClearParams{Confirmed: true}, func() (json.RawMessage, error) {
			return c.LogClear(true)
		})
	})

	t.Run("Count", func(t *testing.T) {
		testRPCNoParams(t, c, m, "log.count", func() (json.RawMessage, error) {
			return c.LogCount()
		})
	})
}

// ============================================================================
// I18n domain
// ============================================================================

func TestI18nMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Get", func(t *testing.T) {
		testRPCNoParams(t, c, m, "i18n.get", func() (json.RawMessage, error) {
			return c.I18nGet()
		})
	})

	t.Run("Switch", func(t *testing.T) {
		testRPC(t, c, m, "i18n.switch", I18nSwitchParams{Lang: "zh-CN"}, func() (json.RawMessage, error) {
			return c.I18nSwitch("zh-CN")
		})
	})

	t.Run("List", func(t *testing.T) {
		testRPCNoParams(t, c, m, "i18n.list", func() (json.RawMessage, error) {
			return c.I18nList()
		})
	})
}

// ============================================================================
// Rule domain
// ============================================================================

func TestRuleMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("List", func(t *testing.T) {
		testRPCNoParams(t, c, m, "rule.list", func() (json.RawMessage, error) {
			return c.RuleList()
		})
	})

	t.Run("Get", func(t *testing.T) {
		testRPC(t, c, m, "rule.get", RuleGetParams{ID: "rule_1"}, func() (json.RawMessage, error) {
			return c.RuleGet("rule_1")
		})
	})

	t.Run("Create", func(t *testing.T) {
		params := RuleCreateParams{ID: "r1", Intro: "test rule", Scope: "global", Priority: 1, Enabled: true}
		testRPC(t, c, m, "rule.create", params, func() (json.RawMessage, error) {
			return c.RuleCreate(params)
		})
	})

	t.Run("Update", func(t *testing.T) {
		intro := "updated"
		params := RuleUpdateParams{ID: "r1", Intro: &intro}
		testRPC(t, c, m, "rule.update", params, func() (json.RawMessage, error) {
			return c.RuleUpdate(params)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		testRPC(t, c, m, "rule.delete", RuleDeleteParams{ID: "rule_1"}, func() (json.RawMessage, error) {
			return c.RuleDelete("rule_1")
		})
	})
}

// ============================================================================
// KV Store domain
// ============================================================================

func TestKVStoreMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Get", func(t *testing.T) {
		testRPC(t, c, m, "kvstore.get", KVGetParams{Key: "k1"}, func() (json.RawMessage, error) {
			return c.KVGet("k1")
		})
	})

	t.Run("Set", func(t *testing.T) {
		testRPC(t, c, m, "kvstore.set", KVSetParams{Key: "k1", Value: "v1", TTL: 60}, func() (json.RawMessage, error) {
			return c.KVSet("k1", "v1", 60)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		testRPC(t, c, m, "kvstore.delete", KVDeleteParams{Key: "k1"}, func() (json.RawMessage, error) {
			return c.KVDelete("k1")
		})
	})

	t.Run("List", func(t *testing.T) {
		testRPC(t, c, m, "kvstore.list", KVListParams{Prefix: "p", Limit: 10, WithValues: true}, func() (json.RawMessage, error) {
			return c.KVList("p", 10, true)
		})
	})

	t.Run("BatchSet", func(t *testing.T) {
		entries := []KVBatchSetEntry{{Key: "k1", Value: "v1"}}
		testRPC(t, c, m, "kvstore.batch_set", KVBatchSetParams{Entries: entries}, func() (json.RawMessage, error) {
			return c.KVBatchSet(entries)
		})
	})

	t.Run("Clear", func(t *testing.T) {
		testRPC(t, c, m, "kvstore.clear", KVClearParams{Prefix: "tmp"}, func() (json.RawMessage, error) {
			return c.KVClear("tmp")
		})
	})
}

// ============================================================================
// Entity Tags domain
// ============================================================================

func TestEntityTagsMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Get", func(t *testing.T) {
		testRPCNoParams(t, c, m, "entity_tags.get", func() (json.RawMessage, error) {
			return c.EntityTagsGet()
		})
	})

	t.Run("Save", func(t *testing.T) {
		defs := []EntityTagDef{{Name: "t1", Title: "Tag 1", Desc: "desc"}}
		testRPC(t, c, m, "entity_tags.save", EntityTagsSaveParams{Types: defs}, func() (json.RawMessage, error) {
			return c.EntityTagsSave(defs)
		})
	})
}

// ============================================================================
// Skill domain
// ============================================================================

func TestSkillMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("List", func(t *testing.T) {
		testRPC(t, c, m, "skill.list", SkillListParams{AgentName: "a"}, func() (json.RawMessage, error) {
			return c.SkillList("a")
		})
	})

	t.Run("Get", func(t *testing.T) {
		testRPC(t, c, m, "skill.get", SkillGetParams{Name: "s1", AgentName: "a"}, func() (json.RawMessage, error) {
			return c.SkillGet("s1", "a")
		})
	})

	t.Run("Reload", func(t *testing.T) {
		testRPCNoParams(t, c, m, "skill.reload", func() (json.RawMessage, error) {
			return c.SkillReload()
		})
	})
}

// ============================================================================
// Translate / Optimize domain
// ============================================================================

func TestTranslateOptimizeMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Translate", func(t *testing.T) {
		testRPC(t, c, m, "translate.rpc", TranslateParams{Text: "hello", Lang: "zh"}, func() (json.RawMessage, error) {
			return c.Translate("hello", "zh")
		})
	})
}

// ============================================================================
// User domain
// ============================================================================

func TestUserMethods(t *testing.T) {
	m := newMockGateway()
	c := &Client{gw: m}

	t.Run("Config", func(t *testing.T) {
		testRPCNoParams(t, c, m, "user.config", func() (json.RawMessage, error) {
			return c.UserConfig()
		})
	})
}
