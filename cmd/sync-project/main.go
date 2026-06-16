package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/DotNetAge/gort/pkg/gateway"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <project-dir>\n", os.Args[0])
		os.Exit(1)
	}
	projectDir := os.Args[1]

	addr := "ws://localhost:1314/ws"
	c := gateway.NewClient(addr)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 连接失败: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = c.Close() }()
	fmt.Fprintf(os.Stderr, "✅ 已连接 %s\n\n", addr)

	// 1. Sync
	params := map[string]string{"project_dir": projectDir}
	result, err := c.Call(ctx, "memory.sync_project", params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ memory.sync_project 失败: %v\n", err)
		os.Exit(1)
	}
	var res map[string]any
	_ = json.Unmarshal(result, &res)
	fmt.Fprintf(os.Stderr, "✅ memory.sync_project 完成\n")
	for k, v := range res {
		fmt.Printf("  %s: %v\n", k, v)
	}

	// 2. Stats
	statsResult, err := c.Call(ctx, "memory.stats", params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ memory.stats 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\n📊 索引统计:\n%s\n\n", string(statsResult))

	// 3. List first page of chunks (with chunk_meta)
	chunksParams := map[string]any{"page": 1, "page_size": 5}
	chunksResult, err := c.Call(ctx, "memory.chunks", chunksParams)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ memory.chunks 失败: %v\n", err)
		os.Exit(1)
	}

	var chunksRes struct {
		Chunks []struct {
			ID        string `json:"id"`
			DocID     string `json:"doc_id"`
			ParentID  string `json:"parent_id"`
			MIMEType  string `json:"mime_type"`
			Content   string `json:"content"`
			ChunkMeta struct {
				Index        int      `json:"index"`
				StartPos     int      `json:"start_pos"`
				EndPos       int      `json:"end_pos"`
				HeadingLevel int      `json:"heading_level"`
				HeadingPath  []string `json:"heading_path,omitempty"`
			} `json:"chunk_meta"`
			Metadata map[string]any `json:"metadata"`
		} `json:"chunks"`
		Total   int  `json:"total"`
		Page    int  `json:"page"`
		HasMore bool `json:"has_more"`
	}
	if err := json.Unmarshal(chunksResult, &chunksRes); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ 解析chunks失败: %v\n原始: %s\n", err, string(chunksResult))
		os.Exit(1)
	}
	fmt.Printf("✅ 成功获取 %d 条chunks (共 %d 条)\n\n", len(chunksRes.Chunks), chunksRes.Total)
	for i, c := range chunksRes.Chunks {
		fmt.Printf("=== Chunk #%d ===\n", i+1)
		fmt.Printf("  ID:           %s\n", truncate(c.ID, 48))
		fmt.Printf("  DocID:        %s\n", truncate(c.DocID, 48))
		fmt.Printf("  ParentID:     %s\n", c.ParentID)
		fmt.Printf("  MimeType:     %s\n", c.MIMEType)
		fmt.Printf("  Content:      %s\n", truncate(c.Content, 100))
		fmt.Printf("  ChunkMeta:\n")
		fmt.Printf("    Index:        %d\n", c.ChunkMeta.Index)
		fmt.Printf("    StartPos:     %d\n", c.ChunkMeta.StartPos)
		fmt.Printf("    EndPos:       %d\n", c.ChunkMeta.EndPos)
		fmt.Printf("    HeadingLevel: %d\n", c.ChunkMeta.HeadingLevel)
		if len(c.ChunkMeta.HeadingPath) > 0 {
			fmt.Printf("    HeadingPath:  %v\n", c.ChunkMeta.HeadingPath)
		}
		fmt.Printf("  Metadata:\n")
		for k, v := range c.Metadata {
			fmt.Printf("    %s: %v\n", k, truncate(fmt.Sprintf("%v", v), 60))
		}
		fmt.Println()
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
