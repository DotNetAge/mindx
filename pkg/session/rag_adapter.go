package session

import (
	"context"

	goharnessmemory "github.com/DotNetAge/goharness/memory"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/pkg/memory"
)

var _ goharnesssession.MemoryStore = (*RAGMemoryAdapter)(nil)

type RAGMemoryAdapter struct {
	rag        *memory.RAGMemory
	agentName  string
	projectDir string
}

func NewRAGMemoryAdapter(rag *memory.RAGMemory, agentName, projectDir string) *RAGMemoryAdapter {
	return &RAGMemoryAdapter{rag: rag, agentName: agentName, projectDir: projectDir}
}

func (a *RAGMemoryAdapter) StoreChunks(ctx context.Context, sessionID string, chunks []goharnessmemory.MemoryChunk) error {
	if a.rag == nil || len(chunks) == 0 {
		return nil
	}
	// Ensure each chunk gets all required metadata fields.
	for i := range chunks {
		if chunks[i].SessionID == "" {
			chunks[i].SessionID = sessionID
		}
		if chunks[i].AgentName == "" {
			chunks[i].AgentName = a.agentName
		}
		if chunks[i].ProjectDir == "" {
			chunks[i].ProjectDir = a.projectDir
		}
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
