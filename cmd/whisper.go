package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DotNetAge/mindx/internal/svc"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

var whisperCmd = &cobra.Command{
	Use:   "whisper [message]",
	Short: "向Master Agent发送任务指令后立即退出 (fire-and-forget)",
	Long: `Whisper 是一个轻量级的CLI客户端，用于向正在运行的 MindX Master Agent 发送消息。

它设计用于自动化场景（如cron定时任务），发送消息后立即退出，不阻塞。

典型用法:
  # Cron 定时触发每日任务
  0 9 * * * mindx whisper "检查并完成今天的任务"

  # 手动触发紧急任务
  mindx whisper "紧急处理线上bug: 用户无法登录"

  # 发送带标签的消息
  mindx whisper --tag daily --priority high "开始Q2发布准备"

选项:
  --wait     等待Agent的首次响应再退出 (默认: false, 立即退出)
  --timeout  等待超时时间(秒) (默认: 10, 仅在 --wait 时生效)
  --tag      消息标签 (可选, 用于分类和过滤)
  --priority 消息优先级: low|normal|high|urgent (默认: normal)
  --silent   静默模式, 只输出错误信息 (默认: false)
  --verbose  详细模式, 输出连接和发送详情 (默认: false)`,
	Args: cobra.ExactArgs(1),
	RunE: runWhisper,
}

var (
	whisperWait     bool
	whisperTimeout  int
	whisperTag      string
	whisperPriority string
	whisperSilent   bool
	whisperVerbose  bool
)

func init() {
	rootCmd.AddCommand(whisperCmd)

	whisperCmd.Flags().BoolVar(&whisperWait, "wait", false, "等待Agent的首次响应")
	whisperCmd.Flags().IntVar(&whisperTimeout, "timeout", 10, "等待超时时间(秒)")
	whisperCmd.Flags().StringVar(&whisperTag, "tag", "", "消息标签")
	whisperCmd.Flags().StringVar(&whisperPriority, "priority", "normal", "消息优先级: low|normal|high|urgent")
	whisperCmd.Flags().BoolVar(&whisperSilent, "silent", false, "静默模式")
	whisperCmd.Flags().BoolVar(&whisperVerbose, "verbose", false, "详细模式")
}

func runWhisper(cmd *cobra.Command, args []string) error {
	message := args[0]

	if !whisperSilent {
		fmt.Printf("🤫 Whisper: 发送消息给 Master Agent...\n")
	}

	settings := loadWhisperSettings()

	wsURL := fmt.Sprintf("ws://%s%s", settings.Addr, settings.WSPath)
	if whisperVerbose {
		log.Printf("📡 连接到: %s", wsURL)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("❌ 连接失败: %w\n请确保 MindX 服务已启动 (运行 'mindx start')", err)
	}
	defer conn.Close()

	if whisperVerbose {
		log.Printf("✅ 已连接到 Master Agent")
	}

	enhancedMessage := enhanceMessage(message)

	if err := conn.WriteMessage(websocket.TextMessage, []byte(enhancedMessage)); err != nil {
		return fmt.Errorf("❌ 发送失败: %w", err)
	}

	if !whisperSilent {
		fmt.Printf("✅ 消息已发送: %s\n", truncate(message, 50))
	}

	if !whisperWait {
		if !whisperSilent {
			fmt.Printf("⚡ Whisper: 任务已触发, 即时退出\n")
			fmt.Printf("💡 使用 'mindx tui' 查看工作进度\n")
		}
		return nil
	}

	if !whisperSilent {
		fmt.Printf("⏳ 等待 Agent 首次响应 (超时: %ds)...\n", whisperTimeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(whisperTimeout)*time.Second)
	defer cancel()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, msg, err := conn.ReadMessage()
				if err != nil {
					if whisperVerbose {
						log.Printf("读取响应出错: %v", err)
					}
					return
				}

				response := string(msg)
				if isAgentResponse(response) {
					if !whisperSilent {
						fmt.Printf("\n📨 Agent 响应:\n%s\n", truncate(response, 500))
						fmt.Printf("✅ Whisper: 收到响应, 退出\n")
					}
					return
				}
			}
		}
	}()

	<-done

	return nil
}

type whisperSettings struct {
	Addr   string
	WSPath string
}

func loadWhisperSettings() whisperSettings {
	addr := os.Getenv("MINDX_WS_ADDR")
	if addr == "" {
		addr = svc.DefaultPort
	}

	wsPath := os.Getenv("MINDX_WS_PATH")
	if wsPath == "" {
		wsPath = "/ws"
	}

	return whisperSettings{
		Addr:   addr,
		WSPath: wsPath,
	}
}

func enhanceMessage(rawMsg string) string {
	metadata := map[string]interface{}{
		"source":    "whisper",
		"timestamp": time.Now().Format(time.RFC3339),
		"priority":  whisperPriority,
	}

	if whisperTag != "" {
		metadata["tag"] = whisperTag
	}

	message := WhisperMessage{
		Text:     rawMsg,
		Metadata: metadata,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return rawMsg
	}
	return string(data)
}

func isAgentResponse(msg string) bool {
	return len(msg) > 10 && !containsThinkingMarker(msg)
}

func containsThinkingMarker(s string) bool {
	thinkingMarkers := []string{"thinking:", "🤔", "...", "  "}
	for _, marker := range thinkingMarkers {
		if len(s) < 30 && strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen]) + "..."
}

// WhisperMessage 定义了从 whisper 发送给 MA 的消息格式
type WhisperMessage struct {
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata"`
}

func (m *WhisperMessage) MarshalJSON() ([]byte, error) {
	type Alias WhisperMessage
	aux := struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(m),
		Type:  "whisper_command",
	}

	return json.Marshal(aux)
}
