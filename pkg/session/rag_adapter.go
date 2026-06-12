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

func (a *RAGMemoryAdapter) Store(_ context.Context, sessionID, title, content string) error {
	if a.rag == nil {
		return nil
	}
	_, err := a.rag.Store(context.Background(), goharnessmemory.MemoryRecord{
		Type:      goharnessmemory.MemoryTypeSession,
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
		goharnessmemory.WithMemoryTypes(goharnessmemory.MemoryTypeSession),
		goharnessmemory.WithMemorySessionID(sessionID),
		goharnessmemory.WithMemoryLimit(limit),
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
