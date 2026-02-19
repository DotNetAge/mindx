package brain

import (
	"mindx/internal/core"
)

type ResponseBuilder struct{}

func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{}
}

func (rb *ResponseBuilder) BuildLeftBrainResponse(thinkResult *core.ThinkingResult, tools []*core.ToolSchema) *core.ThinkingResponse {
	if tools == nil {
		tools = make([]*core.ToolSchema, 0)
	}
	return &core.ThinkingResponse{
		Answer: thinkResult.Answer,
		Tools:  tools,
		SendTo: thinkResult.SendTo,
	}
}

func (rb *ResponseBuilder) BuildToolCallResponse(answer string, tools []*core.ToolSchema, sendTo string) *core.ThinkingResponse {
	if tools == nil {
		tools = make([]*core.ToolSchema, 0)
	}
	return &core.ThinkingResponse{
		Answer: answer,
		Tools:  tools,
		SendTo: sendTo,
	}
}
