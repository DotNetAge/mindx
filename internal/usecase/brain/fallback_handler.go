package brain

import (
	"mindx/internal/core"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
)

type FallbackHandler struct {
	rightBrain      core.Thinking
	toolCaller      *ToolCaller
	responseBuilder *ResponseBuilder
	logger          logging.Logger
}

func NewFallbackHandler(
	rightBrain core.Thinking,
	toolCaller *ToolCaller,
	responseBuilder *ResponseBuilder,
	logger logging.Logger,
) *FallbackHandler {
	return &FallbackHandler{
		rightBrain:      rightBrain,
		toolCaller:      toolCaller,
		responseBuilder: responseBuilder,
		logger:          logger,
	}
}

func (fh *FallbackHandler) Handle(
	question string,
	thinkResult *core.ThinkingResult,
	historyDialogue []*core.DialogueMessage,
	leftBrainSearchedTools []*core.ToolSchema,
) (*core.ThinkingResponse, error) {
	fh.logger.Info(i18n.T("brain.fallback_try_right"))

	if len(leftBrainSearchedTools) > 0 {
		answer, err := fh.toolCaller.ExecuteToolCall(
			fh.rightBrain,
			question,
			historyDialogue,
			leftBrainSearchedTools,
		)
		if err == nil && answer != "" {
			return fh.responseBuilder.BuildToolCallResponse(answer, leftBrainSearchedTools, thinkResult.SendTo), nil
		}
	}

	fh.logger.Warn(i18n.T("brain.fallback_right_failed"))
	return fh.responseBuilder.BuildLeftBrainResponse(thinkResult, nil), nil
}
