package session

import (
	"context"

	goreactmemory "github.com/DotNetAge/goreact/memory"
	goreactsession "github.com/DotNetAge/goreact/session"
	"github.com/DotNetAge/mindx/pkg/memory"
)

var _ goreactsession.MemoryStore = (*RAGMemoryAdapter)(nil)

type RAGMemoryAdapter struct {
	rag *memory.RAGMemory
}

func NewRAGMemoryAdapter(rag *memory.RAGMemory) *RAGMemoryAdapter {
	return &RAGMemoryAdapter{rag: rag}
}

func (a *RAGMemoryAdapter) Store(_ context.Context, sessionID, title, content string) error {
	if a.rag == nil {
		return nil
	}
	_, err := a.rag.Store(context.Background(), goreactmemory.MemoryRecord{
		Type:      goreactmemory.MemoryTypeSession,
		SessionID: sessionID,
		Title:     title,
		Content:   content,
	})
	return err
}

func (a *RAGMemoryAdapter) Retrieve(ctx context.Context, query, sessionID string, limit int) ([]string, error) {
	if a.rag == nil {
		return nil, nil
	}
	records, err := a.rag.Retrieve(ctx, query,
		goreactmemory.WithMemoryTypes(goreactmemory.MemoryTypeSession),
		goreactmemory.WithMemorySessionID(sessionID),
		goreactmemory.WithMemoryLimit(limit),
	)
	if err != nil || len(records) == 0 {
		return nil, err
	}
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.Content
	}
	return out, nil
}
