package core

import (
	"fmt"

	goragcore "github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/embedder"
)

// NewEmbedderFromConfig 创建 BGE 本地嵌入模型（ONNX）。
//
// 2026-10-06 记忆剥离重构：Agent 记忆线已退役（压缩摘要改写 .agents/memory/ 文件），
// embedder 仅剩 rag.seg 条目语义索引一个消费者，随 RAG 产品线剥离（阶段三）整体退役。
// 本文件是从 pkg/memory 迁出的过渡放置，届时与 native.SegIndexer 一并删除。
func NewEmbedderFromConfig(modelPath string) (goragcore.Embedder, error) {
	if modelPath == "" {
		return nil, nil
	}
	emb, err := embedder.NewBGEEmbedder(
		embedder.WithBGEModelFile(modelPath),
		embedder.WithBGEDimension(512),
	)
	if err != nil {
		return nil, fmt.Errorf("embedder: 创建 embedder 失败: %w", err)
	}
	return emb, nil
}
