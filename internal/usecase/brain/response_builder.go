package brain

import (
	"mindx/internal/core"
	"fmt"
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

func (rb *ResponseBuilder) BuildLeftBrainPrompt(persona *core.Persona) string {
	return fmt.Sprintf(core.AssistantPrompt, persona.Name, persona.Gender, persona.Character, persona.UserContent)
}
