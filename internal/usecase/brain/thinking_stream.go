package brain

import (
	"fmt"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"time"
)

type ThinkingEventType = entity.ThinkingEventType

const (
	ThinkingEventStart      = entity.ThinkingEventStart
	ThinkingEventProgress   = entity.ThinkingEventProgress
	ThinkingEventChunk      = entity.ThinkingEventChunk
	ThinkingEventToolCall   = entity.ThinkingEventToolCall
	ThinkingEventToolResult = entity.ThinkingEventToolResult
	ThinkingEventComplete   = entity.ThinkingEventComplete
	ThinkingEventError      = entity.ThinkingEventError
)

type ThinkingEvent = entity.ThinkingEvent

func NewThinkingEvent(eventType ThinkingEventType, content string) ThinkingEvent {
	return ThinkingEvent{
		Type:      eventType,
		Content:   content,
		Timestamp: time.Now(),
	}
}

func NewThinkingEventWithProgress(eventType ThinkingEventType, content string, progress float64) ThinkingEvent {
	return ThinkingEvent{
		Type:      eventType,
		Content:   content,
		Progress:  progress,
		Timestamp: time.Now(),
	}
}

func NewThinkingEventWithMetadata(eventType ThinkingEventType, content string, metadata map[string]any) ThinkingEvent {
	return ThinkingEvent{
		Type:      eventType,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
}

func NewToolCallEvent(toolName string, args map[string]any) ThinkingEvent {
	return ThinkingEvent{
		Type:    ThinkingEventToolCall,
		Content: fmt.Sprintf(i18n.T("brain.call_tool"), toolName),
		Metadata: map[string]any{
			"tool_name": toolName,
			"arguments": args,
		},
		Timestamp: time.Now(),
	}
}

func NewToolResultEvent(toolName string, result string) ThinkingEvent {
	return ThinkingEvent{
		Type:    ThinkingEventToolResult,
		Content: fmt.Sprintf(i18n.T("brain.tool_return_result"), toolName),
		Metadata: map[string]any{
			"tool_name": toolName,
			"result":    result,
		},
		Timestamp: time.Now(),
	}
}
