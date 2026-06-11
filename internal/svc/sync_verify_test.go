package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestSyncBookProject(t *testing.T) {
	projectDir := "/Users/ray/Library/Mobile Documents/com~apple~CloudDocs/书稿/产品设计实践"

	d := &Daemon{}

	// Manually init sharedMemory same as daemon.go
	if err := d.initMemory(); err != nil {
		t.Fatalf("init memory: %v", err)
	}
	defer d.sharedMemory.Close()

	if err := d.initMemoryWatch(); err != nil {
		t.Fatalf("init watch: %v", err)
	}

	// Build params
	params, _ := json.Marshal(map[string]string{
		"project_dir": projectDir,
	})

	result, err := d.handleMemorySyncProject(context.Background(), params)
	if err != nil {
		t.Fatalf("sync project failed: %v", err)
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("Sync result: %s\n", string(b))

	// Also get stats
	statsResult, err := d.handleMemoryStats(context.Background(), params)
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	sb, _ := json.MarshalIndent(statsResult, "", "  ")
	fmt.Printf("Stats result: %s\n", string(sb))
}
