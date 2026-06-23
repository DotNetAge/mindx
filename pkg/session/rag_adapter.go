package session

import (
	"context"

	goharnessmemory "github.com/DotNetAge/goharness/memory"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/pkg/memory"
)

var _ goharnesssession.MemoryStore = (*RAGMemoryAdapter)(nil)

type RAGMemoryAdapter struct {
	rag *memory.RAGMemory
}

func NewRAGMemoryAdapter(rag *memory.RAGMemory) *RAGMemoryAdapter {
	return &RAGMemoryAdapter{rag: rag}
}

func (a *RAGMemoryAdapter) StoreChunks(ctx context.Context, sessionID string, chunks []goharnessmemory.MemoryChunk) error {
	if a.rag == nil || len(chunks) == 0 {
		return nil
	}
	// Ensure each chunk gets the session ID
	for i := range chunks {
		chunks[i].SessionID = sessionID
	}
	return a.rag.StoreMemoryChunks(ctx, chunks)
}

func (a *RAGMemoryAdapter) Retrieve(ctx context.Context, query, sessionID string, limit int) ([]goharnessmemory.MemoryChunk, error) {
	if a.rag == nil {
		return nil, nil
	}

	opts := []goharnessmemory.RetrieveOption{
		goharnessmemory.WithMemorySessionID(sessionID),
		goharnessmemory.WithMemoryLimit(limit),
	}
	chunks, err := a.rag.Retrieve(ctx, query, opts...)
	if err != nil {
		return nil, err
	}
	return chunks, nil
}
