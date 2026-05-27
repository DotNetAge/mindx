package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goreactmemory "github.com/DotNetAge/goreact/memory"
)

type memoryQueryParams struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit,omitempty"`
	Type     string  `json:"type,omitempty"`
	MinScore float64 `json:"min_score,omitempty"`
}

func (d *Daemon) handleMemoryQuery(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryQueryParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	opts := []goreactmemory.RetrieveOption{}
	if p.Limit > 0 {
		opts = append(opts, goreactmemory.WithMemoryLimit(p.Limit))
	}
	if p.MinScore > 0 {
		opts = append(opts, goreactmemory.WithMinScore(p.MinScore))
	}
	if p.Type != "" {
		switch p.Type {
		case "longterm":
			opts = append(opts, goreactmemory.WithMemoryTypes(goreactmemory.MemoryTypeLongTerm))
		case "session":
			opts = append(opts, goreactmemory.WithMemoryTypes(goreactmemory.MemoryTypeSession))
		}
	}

	records, err := mem.Retrieve(context.Background(), p.Query, opts...)
	if err != nil {
		return nil, fmt.Errorf("memory query failed: %w", err)
	}

	if records == nil {
		return []goreactmemory.MemoryRecord{}, nil
	}
	return records, nil
}

type memoryStoreParams struct {
	Title   string   `json:"title,omitempty"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
	Type    string   `json:"type,omitempty"`
}

func (d *Daemon) handleMemoryStore(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryStoreParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	record := goreactmemory.MemoryRecord{
		Title:     p.Title,
		Content:   p.Content,
		Tags:      p.Tags,
		CreatedAt: time.Now(),
	}
	if p.Type == "session" {
		record.Type = goreactmemory.MemoryTypeSession
	} else {
		record.Type = goreactmemory.MemoryTypeLongTerm
	}

	id, err := mem.Store(context.Background(), record)
	if err != nil {
		return nil, fmt.Errorf("memory store failed: %w", err)
	}

	return map[string]string{"id": id}, nil
}

type memoryDeleteParams struct {
	ID string `json:"id"`
}

func (d *Daemon) handleMemoryDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p memoryDeleteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	mem := d.sharedMemory
	if mem == nil {
		return nil, fmt.Errorf("memory service not available (embedder not configured)")
	}

	if err := mem.Delete(context.Background(), p.ID); err != nil {
		return nil, fmt.Errorf("memory delete failed: %w", err)
	}

	return map[string]string{"status": "ok", "deleted_id": p.ID}, nil
}
