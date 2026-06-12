package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	gochat "github.com/DotNetAge/gochat"
	gochatcore "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/goharness/config"
	goharnesssession "github.com/DotNetAge/goharness/session"
)

var _ goharnesssession.Summarizer = (*LLMSummarizer)(nil)

type LLMSummarizer struct {
	model config.ModelConfig
}

func NewLLMSummarizer(model config.ModelConfig) *LLMSummarizer {
	return &LLMSummarizer{model: model}
}

func (s *LLMSummarizer) Summarize(ctx context.Context, msgs []goharnesssession.Message) (string, error) {
	if len(msgs) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("Summarize the following conversation excerpt to preserve key information, decisions, and context. ")
	sb.WriteString("Focus on facts, conclusions, and action items. Output only the summary text.\n\n")
	for _, m := range msgs {
		switch m.Role {
		case "user":
			sb.WriteString("[User] ")
		case "assistant":
			sb.WriteString("[Assistant] ")
		case "system":
			sb.WriteString("[System] ")
		default:
			fmt.Fprintf(&sb, "[%s] ", m.Role)
		}
		sb.WriteString(m.Content)
		sb.WriteString("\n\n")
	}

	resp, err := gochat.Client().Config(
		gochat.WithAPIKey(s.model.APIKey),
		gochat.WithBaseURL(s.model.BaseURL),
		gochat.WithModel(s.model.Name),
		gochat.WithAuthToken(s.model.AuthToken),
		gochat.WithTimeout(60*time.Second),
	).
		Messages(gochatcore.NewUserMessage(sb.String())).
		MaxTokens(1024).
		Temperature(0.3).
		GetResponse()
	if err != nil {
		return "", fmt.Errorf("summarize: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}
