package usecase

import (
	"mindx/internal/usecase/tools"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// BenchmarkToolManagerLoad 测试工具加载性能
func BenchmarkToolManagerLoad(b *testing.B) {
	// 创建临时目录
	tmpDir := b.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")

	// 创建 10 个测试工具
	for i := 0; i < 10; i++ {
		toolDir := filepath.Join(toolsDir, "tool"+string(rune(i)))
		require.NoError(b, os.MkdirAll(toolDir, 0755))

		toolJSON := `{
			"name": "tool` + string(rune(i)) + `",
			"description": "测试工具",
			"version": "1.0.0",
			"type": "shell",
			"command": "echo"
		}`

		require.NoError(b, os.WriteFile(
			filepath.Join(toolDir, "tool.json"),
			[]byte(toolJSON),
			0644,
		))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		toolManager := tools.NewToolManager(toolsDir)
		toolManager.LoadTools()
	}
}

// BenchmarkToolAssemble 测试工具组装性能
func BenchmarkToolAssemble(b *testing.B) {
	// TODO: 实现工具组装性能测试
	b.Skip("待实现")
}

// BenchmarkToolExecution 测试工具执行性能
func BenchmarkToolExecution(b *testing.B) {
	// TODO: 实现工具执行性能测试
	b.Skip("待实现")
}
