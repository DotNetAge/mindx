package brain

import (
	"fmt"
	"mindx/internal/core"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"strings"
)

type ContextPreparer struct {
	memory         core.Memory
	historyRequest core.OnHistoryRequest
	logger         logging.Logger
}

func NewContextPreparer(memory core.Memory, historyRequest core.OnHistoryRequest, logger logging.Logger) *ContextPreparer {
	return &ContextPreparer{
		memory:         memory,
		historyRequest: historyRequest,
		logger:         logger,
	}
}

func (cp *ContextPreparer) Prepare(question string, leftBrain core.Thinking) (*processingContext, error) {
	ctx := &processingContext{
		historyDialogue: make([]*core.DialogueMessage, 0),
	}

	memories, err := cp.memory.Search(question)
	if err != nil {
		cp.logger.Warn(i18n.T("brain.get_memory_failed"), logging.Err(err))
	} else {
		cp.logger.Debug(i18n.T("brain.get_memory_success"), logging.String("count", fmt.Sprintf("%d", len(memories))))
	}

	ctx.refs = cp.buildReferencePrompt(memories)

	if cp.historyRequest != nil {
		maxRounds := leftBrain.CalculateMaxHistoryCount()
		ctx.historyDialogue, err = cp.historyRequest(maxRounds)
		if err != nil {
			cp.logger.Warn(i18n.T("brain.get_history_failed"), logging.Err(err))
		} else {
			cp.logger.Debug(i18n.T("brain.get_history_success"),
				logging.Int("max_rounds", maxRounds),
				logging.Int("actual_count", len(ctx.historyDialogue)))
		}
	}

	return ctx, nil
}

func (cp *ContextPreparer) buildReferencePrompt(memories []core.MemoryPoint) string {
	if len(memories) == 0 {
		return ""
	}

	var context strings.Builder
	fmt.Fprintf(&context, "# 参考\n")

	for i, mem := range memories {
		if i > 0 {
			fmt.Fprintf(&context, "\n")
		}
		fmt.Fprintf(&context, "- %s", mem.Summary)
	}

	return context.String()
}
