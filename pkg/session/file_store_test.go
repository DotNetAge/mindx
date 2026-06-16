package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
)

func TestFileStoreWithComplexContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session-test")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	sessionID := "test-session-123"
	agentName := "test-agent"
	ctx := context.Background()

	testCases := []struct {
		name    string
		content string
	}{
		{
			name:    "plain text",
			content: "Hello, this is plain text",
		},
		{
			name:    "markdown content",
			content: "# Title\n\nThis is **bold** and *italic* text.\n\n## Code Example\n\n```go\nfunc hello() {\n    fmt.Println(\"Hello, World!\")\n}\n```\n\n### List\n- Item 1\n- Item 2\n  - Nested item\n\n> This is a blockquote\n\n| Column 1 | Column 2 |\n|----------|----------|\n| Data 1   | Data 2   |",
		},
		{
			name:    "xml content",
			content: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<root>\n  <person id=\"123\">\n    <name>John Doe</name>\n    <email>john@example.com</email>\n    <roles>\n      <role>admin</role>\n      <role>user</role>\n    </roles>\n  </person>\n  <data><![CDATA[This contains <special> & characters]]></data>\n</root>",
		},
		{
			name:    "json content",
			content: "{\n  \"name\": \"test\",\n  \"version\": \"1.0.0\",\n  \"config\": {\n    \"debug\": true,\n    \"ports\": [8080, 8081],\n    \"features\": {\n      \"auth\": {\n        \"enabled\": true,\n        \"providers\": [\"oauth\", \"jwt\"]\n      }\n    }\n  },\n  \"nested\": [[1, 2], [3, 4]]\n}",
		},
		{
			name:    "yaml-like content (should not conflict)",
			content: "key: value\nanother-key: another-value\nlist:\n  - item1\n  - item2",
		},
		{
			name:    "special characters",
			content: "Special chars: # @ $ % ^ & * ( ) _ + = { } [ ] | \\ : ; \" ' < > , . ? /\nNewlines:\nLine 1\nLine 2\nLine 3\n\nTabs:\tTabbed\tContent\n\nEmoji: 🎉🚀💻🔥",
		},
		{
			name:    "multiline code with indentation",
			content: "function complexExample() {\n    const data = {\n        items: [\n            { id: 1, name: \"first\" },\n            { id: 2, name: \"second\" }\n        ],\n        config: {\n            nested: {\n                deep: {\n                    value: \"test\"\n                }\n            }\n        }\n    };\n    \n    return data.items.map(item => ({\n        ...item,\n        processed: true\n    }));\n}",
		},
	}

	t.Run("append and retrieve all formats", func(t *testing.T) {
		for i, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				msg := goharnesssession.Message{
					Role:      "user",
					Content:   tc.content,
					Timestamp: time.Now().UnixMilli() + int64(i),
				}

				err := store.Append(ctx, sessionID, agentName, msg)
				if err != nil {
					t.Errorf("Append failed for %s: %v", tc.name, err)
					return
				}
			})
		}

		retrievedMsgs, err := store.Get(ctx, sessionID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if len(retrievedMsgs) != len(testCases) {
			t.Fatalf("expected %d messages, got %d", len(testCases), len(retrievedMsgs))
		}

		for i, tc := range testCases {
			if retrievedMsgs[i].Content != tc.content {
				t.Errorf("Test case '%s' content mismatch:\nExpected:\n%s\n\nGot:\n%s",
					tc.name, tc.content, retrievedMsgs[i].Content)
			}
			if retrievedMsgs[i].Role != "user" {
				t.Errorf("Test case '%s' role mismatch: expected 'user', got '%s'",
					tc.name, retrievedMsgs[i].Role)
			}
		}
	})

	t.Run("delete specific message", func(t *testing.T) {
		if len(testCases) == 0 {
			return
		}

		retrievedMsgs, err := store.Get(ctx, sessionID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if len(retrievedMsgs) > 0 {
			targetTimestamp := retrievedMsgs[0].Timestamp
			err := store.Delete(ctx, targetTimestamp, sessionID)
			if err != nil {
				t.Errorf("Delete failed: %v", err)
			}
		}
	})

	t.Run("clear session", func(t *testing.T) {
		err := store.Clear(ctx, sessionID)
		if err != nil {
			t.Errorf("Clear failed: %v", err)
		}

		msgs, err := store.Get(ctx, sessionID)
		if err != nil {
			t.Errorf("Get after clear failed: %v", err)
			return
		}

		if len(msgs) != 0 {
			t.Errorf("expected 0 messages after clear, got %d", len(msgs))
		}
	})
}

func TestFileStoreYAMLFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session-yaml-test")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	sessionID := "yaml-format-test"
	agentName := "test-agent"

	markdownContent := "# Complex Markdown\n\n## Features\n- **Bold** and *italic*\n- `code inline`\n- [links](http://example.com)\n\n```json\n{ \"key\": \"value\" }\n```\n\n> Blockquote with \"quotes\" and 'apostrophes'"

	msg := goharnesssession.Message{
		Role:      "assistant",
		Content:   markdownContent,
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli(),
	}

	err = store.Append(ctx, sessionID, agentName, msg)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	yamlPath := filepath.Join(tmpDir, agentName, sessionID, "session.yml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read yaml file failed: %v", err)
	}

	t.Logf("Generated YAML file content:\n%s", string(data))

	retrieved, err := store.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(retrieved) != 1 {
		t.Fatalf("expected 1 message, got %d", len(retrieved))
	}

	if retrieved[0].Content != markdownContent {
		t.Errorf("content mismatch\nExpected:\n%s\n\nGot:\n%s",
			markdownContent, retrieved[0].Content)
	}
}

// TestFileStoreConcurrentAppend 验证并发 Append 不会导致数据损坏（ioMu 保护）。
func TestFileStoreConcurrentAppend(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	sessionID := "concurrent-test"
	agentName := "test-agent"

	const goroutines = 10
	const msgsPerGoroutine = 20

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < msgsPerGoroutine; j++ {
				msg := goharnesssession.Message{
					Role:      "user",
					Content:   fmt.Sprintf("goroutine-%d-msg-%d", id, j),
					Timestamp: time.Now().UnixMilli() + int64(id*msgsPerGoroutine+j),
				}
				if err := store.Append(ctx, sessionID, agentName, msg); err != nil {
					t.Errorf("Append failed: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	// 验证所有消息都正确写入
	msgs, err := store.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	expectedCount := goroutines * msgsPerGoroutine
	if len(msgs) != expectedCount {
		t.Errorf("expected %d messages, got %d", expectedCount, len(msgs))
	}

	// 验证所有消息内容完整
	seen := make(map[string]int)
	for _, m := range msgs {
		seen[m.Content]++
	}
	for i := 0; i < goroutines; i++ {
		for j := 0; j < msgsPerGoroutine; j++ {
			key := fmt.Sprintf("goroutine-%d-msg-%d", i, j)
			if seen[key] != 1 {
				t.Errorf("message %q: expected 1 occurrence, got %d", key, seen[key])
			}
		}
	}
}

// TestFileStoreDeleteSession 验证 DeleteSession 正确删除并返回错误。
func TestFileStoreDeleteSession(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	agentName := "test-agent"

	// 创建 session
	info, err := store.Create(ctx, agentName)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 存入一条消息
	msg := goharnesssession.Message{
		Role:      "user",
		Content:   "hello",
		Timestamp: time.Now().UnixMilli(),
	}
	if err := store.Append(ctx, info.SessionID, agentName, msg); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// 删除 session
	if err := store.DeleteSession(ctx, info.SessionID); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// 确认 session 已删除
	msgs, err := store.Get(ctx, info.SessionID)
	if err != nil {
		t.Fatalf("Get after DeleteSession failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after DeleteSession, got %d", len(msgs))
	}
}

// TestFileStoreDeleteSessionNotFound 验证删除不存在的 session 返回 ErrSessionNotFound。
func TestFileStoreDeleteSessionNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	err = store.DeleteSession(context.Background(), "nonexistent-session")
	if err != goharnesssession.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}
