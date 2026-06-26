package indexing

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/DotNetAge/goharness/session"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
)

// indexFile reads and indexes a single file, returning all chunk IDs.
func (p *IndexService) indexFile(ctx context.Context, absPath string) ([]chunkInfo, error) {
	// If no indexer is configured (e.g. embedder not available), skip indexing
	// but do NOT mark the file as processed — it will be picked up when an
	// indexer becomes available (e.g. after model.switch).
	if p.indexer == nil {
		return nil, nil
	}

	// Content quality gate: skip binary / garbage files before they reach the
	// chunker & embedder pipeline.
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if !isValidFileContent(raw) {
		if p.logger != nil {
			p.logger.Warn("index-service: content quality check failed, skipped",
				"path", absPath,
				"bytes", len(raw),
			)
		}
		return nil, nil
	}

	chunks, err := p.indexer.AddFile(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("add file: %w", err)
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	// Record LLM token usage if a TokenUsageStore is configured
	p.recordTokenUsage(ctx)

	infos := make([]chunkInfo, len(chunks))
	for i, c := range chunks {
		infos[i] = chunkInfo{ID: c.ID}
	}
	return infos, nil
}

// removeChunks removes all tracked chunks for a previously indexed file.
func (p *IndexService) removeChunks(ctx context.Context, chunks []chunkInfo) {
	if p.indexer == nil {
		return
	}
	for _, ci := range chunks {
		if err := p.indexer.Remove(ctx, ci.ID); err != nil && p.logger != nil {
			p.logger.Warn("index-service: failed to remove chunk", "id", ci.ID, "error", err)
		}
	}
}

// recordTokenUsage extracts LLM token usage from the GraphIndexer (if available)
// and writes it to the configured TokenUsageStore.
func (p *IndexService) recordTokenUsage(ctx context.Context) {
	if p.usageStore == nil {
		return
	}
	gi, ok := p.indexer.(*goragindexer.GraphIndexer)
	if !ok {
		return
	}
	tu := gi.LastTokenUsage()
	if tu == nil {
		return
	}

	record := session.TokenUsageRecord{
		ID:               session.NewRecordID(),
		ModelName:        p.modelName,
		PromptTokens:     tu.PromptTokens,
		CompletionTokens: tu.CompletionTokens,
		TotalTokens:      tu.TotalTokens,
		Timestamp:        time.Now(),
	}
	// Attempt to write with source annotation; fall back to plain Append if unavailable.
	if sws, ok := p.usageStore.(interface {
		AppendWithSource(context.Context, session.TokenUsageRecord, string) error
	}); ok {
		if err := sws.AppendWithSource(ctx, record, "indexing"); err != nil && p.logger != nil {
			p.logger.Warn("index-service: failed to record token usage", "error", err)
		}
	} else {
		if err := p.usageStore.Append(ctx, record); err != nil && p.logger != nil {
			p.logger.Warn("index-service: failed to record token usage", "error", err)
		}
	}
}
