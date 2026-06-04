//go:build ignore

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/setup/style"
)

//go:embed runtime
var embeddedFS fs.FS

func main() {
	workspaceDir := filepath.Join(os.TempDir(), "mindx-test-wizard")
	os.RemoveAll(workspaceDir)
	os.MkdirAll(workspaceDir, 0755)
	defer os.RemoveAll(workspaceDir)

	settingsDir := filepath.Join(workspaceDir, "settings")
	os.MkdirAll(settingsDir, 0755)

	// Write dummy providers.yml
	providersData := []byte(`providers:
  - name: openai
    title: OpenAI
    api_key: ""
    models:
      - gpt-4
`)
	os.WriteFile(filepath.Join(settingsDir, "providers.yml"), providersData, 0644)

	// Write dummy models.yml
	modelsData := []byte(`models:
  - id: gpt-4
    provider: openai
    name: GPT-4
`)
	os.WriteFile(filepath.Join(settingsDir, "models.yml"), modelsData, 0644)

	agentsDir := filepath.Join(workspaceDir, "agents")
	os.MkdirAll(agentsDir, 0644)

	cfg := &core.MindxConfig{}

	// Simulate what wizard.go View() outputs
	title := style.GradientTitle("")
	markdownContent := "选择提供商\n\n请选择一个 AI 提供商。"

	// Padded view simulation
	content := lipgloss.NewStyle().Padding(1, 2).Render(markdownContent)
	lines := strings.Count(content, "\n") + 1
	viewHeight := 24
	if viewHeight > lines+1 {
		content += strings.Repeat("\n", viewHeight-lines)
	} else {
		content += "\n"
	}

	finalView := tea.NewView(title + "\n\n" + content)

	fmt.Println("=== SETUP WIZARD VIEW OUTPUT ===")
	fmt.Print(finalView.Content)
	fmt.Println("=== END ===")
	fmt.Println()
	fmt.Printf("Raw title bytes: %q\n", title)
}
