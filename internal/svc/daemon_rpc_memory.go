package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goreactcore "github.com/DotNetAge/goreact/core"
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

	opts := []goreactcore.RetrieveOption{}
	if p.Limit > 0 {
		opts = append(opts, goreactcore.WithMemoryLimit(p.Limit))
	}
	if p.MinScore > 0 {
		opts = append(opts, goreactcore.WithMinScore(p.MinScore))
	}
	if p.Type != "" {
		switch p.Type {
		case "longterm":
			opts = append(opts, goreactcore.WithMemoryTypes(goreactcore.MemoryTypeLongTerm))
		case "session":
			opts = append(opts, goreactcore.WithMemoryTypes(goreactcore.MemoryTypeSession))
		}
	}

	records, err := mem.Retrieve(context.Background(), p.Query, opts...)
	if err != nil {
		return nil, fmt.Errorf("memory query failed: %w", err)
	}

	if records == nil {
		return []goreactcore.MemoryRecord{}, nil
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

	record := goreactcore.MemoryRecord{
		Title:     p.Title,
		Content:   p.Content,
		Tags:      p.Tags,
		CreatedAt: time.Now(),
	}
	if p.Type == "session" {
		record.Type = goreactcore.MemoryTypeSession
	} else {
		record.Type = goreactcore.MemoryTypeLongTerm
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
