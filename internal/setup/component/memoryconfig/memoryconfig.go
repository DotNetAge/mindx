package memoryconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	goragutils "github.com/DotNetAge/gorag/utils"

	"github.com/DotNetAge/mindx/internal/i18n"
	setupmsg "github.com/DotNetAge/mindx/internal/setup/msg"
	"github.com/DotNetAge/mindx/internal/setup/style"
)

const minContentWidth = 60

type Model struct {
	state          int
	choice         bool
	progressBar    progress.Model
	downloadCh     chan setupmsg.DownloadProgressMsg
	downloadErr    error
	downloadStatus string
	embedderModel  string
	workspaceDir   string
	width          int
	height         int
	renderer       *glamour.TermRenderer
}

func New(workspaceDir string, alreadyDownloaded bool) *Model {
	m := &Model{
		state:        0,
		choice:       true,
		progressBar:  progress.New(progress.WithWidth(minContentWidth - 12)),
		workspaceDir: workspaceDir,
		width:        80,
		height:       24,
		renderer:     initGlamour(minContentWidth),
	}

	if alreadyDownloaded {
		m.state = 2
		m.embedderModel = "model_q4.onnx"
	}

	return m
}

func (m *Model) EmbedderModel() string { return m.embedderModel }

func initGlamour(width int) *glamour.TermRenderer {
	if width < 40 {
		width = 40
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	return r
}

func renderMarkdown(r *glamour.TermRenderer, src string) string {
	if r == nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return out
}

func yesNoIndicator(yes bool) string {
	if yes {
		return "**> Yes**  \n  No"
	}
	return "  Yes  \n**> No**"
}

func contentWidth(w int) int {
	if w > minContentWidth {
		cw := w - 4
		return cw
	}
	return minContentWidth
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		cw := contentWidth(m.width)
		m.progressBar = progress.New(progress.WithWidth(cw - 12))
		m.renderer = initGlamour(cw)

	case tea.KeyPressMsg:
		switch m.state {
		case 0:
			switch msg.String() {
			case "q", "ctrl+c", "esc":
				return m, func() tea.Msg { return setupmsg.WizardQuitMsg{} }
			case "left", "right":
				m.choice = !m.choice
			case "enter":
				if m.choice {
					m.state = 1
					m.downloadCh = make(chan setupmsg.DownloadProgressMsg, 100)
					cacheDir := filepath.Join(m.workspaceDir, "data", "models")
					m.embedderModel = "model_q4.onnx"
					go runModelDownload(cacheDir, m.downloadCh)
					return m, m.listenDownloadCmd()
				}
				return m, func() tea.Msg {
					return setupmsg.MemoryDecisionMsg{Download: false}
				}
			}
		case 1:
			if msg.String() == "q" || msg.String() == "ctrl+c" || msg.String() == "esc" {
				return m, func() tea.Msg { return setupmsg.WizardQuitMsg{} }
			}
		case 2:
			if msg.String() == "enter" || msg.String() == " " ||
				msg.String() == "s" || msg.String() == "S" {
				return m, func() tea.Msg {
					return setupmsg.MemoryDecisionMsg{Download: true}
				}
			}
		}

	case setupmsg.DownloadProgressMsg:
		if m.state == 1 {
			if msg.Err != nil {
				m.downloadErr = msg.Err
				m.embedderModel = ""
				m.downloadStatus = i18n.T("setup.memory.download.failed")
				m.state = 2
				return m, nil
			}
			if msg.Done {
				m.downloadStatus = i18n.T("setup.memory.processing")
				m.state = 2
				return m, nil
			}
			if msg.Status != "" {
				m.downloadStatus = msg.Status
			}
			var progCmd tea.Cmd
			if msg.Total > 0 {
				progCmd = m.progressBar.SetPercent(float64(msg.Current) / float64(msg.Total))
			} else if msg.Current > 0 {
				progCmd = m.progressBar.SetPercent(0.5)
			}
			return m, tea.Batch(progCmd, m.listenDownloadCmd())
		}
	}

	return m, nil
}

func (m *Model) listenDownloadCmd() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.downloadCh
		if !ok {
			return nil
		}
		return msg
	}
}

type downloadObserver struct {
	ch chan<- setupmsg.DownloadProgressMsg
}

func (o *downloadObserver) OnEvent(event goragutils.DownloadEvent) {
	msg := setupmsg.DownloadProgressMsg{
		Current: event.Current,
		Total:   event.Total,
		File:    event.File,
	}
	switch event.Type {
	case goragutils.EventStart:
		msg.Status = fmt.Sprintf(i18n.T("setup.memory.downloading"), event.File)
	case goragutils.EventProgress:
		downloadedMB := float64(event.Current) / (1024 * 1024)
		if event.Total > 0 {
			totalMB := float64(event.Total) / (1024 * 1024)
			msg.Status = fmt.Sprintf(i18n.T("setup.memory.download.progress"), downloadedMB, totalMB)
		} else {
			msg.Status = fmt.Sprintf(i18n.T("setup.memory.download.progress.mb"), downloadedMB)
		}
	case goragutils.EventComplete:
		msg.Status = fmt.Sprintf(i18n.T("setup.memory.download.complete"), event.File)
	}
	o.ch <- msg
}

func runModelDownload(cacheDir string, ch chan<- setupmsg.DownloadProgressMsg) {
	defer close(ch)

	modelID := "Xenova/chinese-clip-vit-base-patch16"
	modelFile := "onnx/model_q4.onnx"
	dstPath := filepath.Join(cacheDir, filepath.Base(modelFile))

	if _, err := os.Stat(dstPath); err == nil {
		ch <- setupmsg.DownloadProgressMsg{
			Done: true,
		}
		return
	}

	observer := &downloadObserver{ch: ch}

	downloader, err := goragutils.NewModelDownloader(cacheDir)
	if err != nil {
		ch <- setupmsg.DownloadProgressMsg{Done: true, Err: fmt.Errorf(i18n.T("setup.memory.downloader.create.failed"), err)}
		return
	}
	downloader.WithObserver(observer)

	files := []string{modelFile}
	if _, err := downloader.Download(modelID, files); err != nil {
		ch <- setupmsg.DownloadProgressMsg{Done: true, Err: fmt.Errorf(i18n.T("setup.memory.download.failed.err"), err)}
		return
	}

	srcPath := filepath.Join(cacheDir, strings.ReplaceAll(modelID, "/", string(filepath.Separator)), modelFile)

	src, err := os.Open(srcPath)
	if err != nil {
		ch <- setupmsg.DownloadProgressMsg{Done: true, Err: fmt.Errorf(i18n.T("setup.memory.model.open.failed"), err)}
		return
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		ch <- setupmsg.DownloadProgressMsg{Done: true, Err: fmt.Errorf(i18n.T("setup.memory.model.create.failed"), err)}
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		ch <- setupmsg.DownloadProgressMsg{Done: true, Err: fmt.Errorf(i18n.T("setup.memory.model.copy.failed"), err)}
		return
	}

	srcDir := filepath.Join(cacheDir, strings.ReplaceAll(modelID, "/", string(filepath.Separator)))
	os.RemoveAll(srcDir)

	ch <- setupmsg.DownloadProgressMsg{
		Done:   true,
		Status: i18n.T("setup.memory.model.download.complete"),
	}
}

func (m *Model) View() string {
	var b strings.Builder
	switch m.state {
	case 0:
		md := `💾 记忆体配置

🔴 **Embedder 模型未下载**

Embedder 模型用于将文本向量化，实现语义搜索和 RAG 记忆。

下载 Chinese-CLIP 模型后，Agent 可以跨会话检索历史知识。
不下载则仅有基础会话记忆。

是否下载 Embedder 模型?

` + yesNoIndicator(m.choice) + `

← → 切换  **Enter** 确认  **Esc** 退出`
		b.WriteString(renderMarkdown(m.renderer, md))

	case 1:
		b.WriteString(renderMarkdown(m.renderer, i18n.T("setup.memory.view.downloading")))
		if m.downloadStatus != "" {
			b.WriteString(renderMarkdown(m.renderer, m.downloadStatus))
			b.WriteString("\n\n")
		}
		b.WriteString(m.progressBar.View())
		b.WriteString("\n\n")
		b.WriteString(renderMarkdown(m.renderer, i18n.T("setup.memory.view.waiting")))

	case 2:
		if m.downloadErr != nil {
			b.WriteString(renderMarkdown(m.renderer, fmt.Sprintf(
				i18n.T("setup.memory.view.error"),
				m.downloadErr.Error(),
			)))
		} else {
			b.WriteString(renderMarkdown(m.renderer, i18n.T("setup.memory.view.success")))
		}
	}
	content := style.Border.Render(b.String())
	return lipgloss.JoinVertical(
		lipgloss.Left,
		style.GradientTitle(""),
		"",
		content,
	) + "\n"
}
