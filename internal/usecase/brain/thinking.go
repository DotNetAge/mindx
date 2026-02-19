package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type Thinking struct {
	modelConfig        *config.ModelConfig
	client             *openai.Client
	prompt             string
	systemPrompt       string
	logger             logging.Logger
	tokenUsageRepo     core.TokenUsageRepository
	tokenBudget        *config.TokenBudgetConfig
	tokenBudgetManager *TokenBudgetManager
	eventChan          chan<- ThinkingEvent
}

func NewThinking(
	modelConfig *config.ModelConfig,
	prompt string,
	logger logging.Logger,
	tokenUsageRepo core.TokenUsageRepository,
	tokenBudget *config.TokenBudgetConfig) *Thinking {

	clientConfig := openai.DefaultConfig(modelConfig.APIKey)
	if modelConfig.BaseURL != "" {
		clientConfig.BaseURL = modelConfig.BaseURL
	}

	budgetManager := NewTokenBudgetManager(
		modelConfig.MaxTokens,
		tokenBudget.ReservedOutputTokens,
		tokenBudget.MinHistoryRounds,
		tokenBudget.AvgTokensPerRound,
		logger,
	)

	return &Thinking{
		client:             openai.NewClientWithConfig(clientConfig),
		modelConfig:        modelConfig,
		prompt:             prompt,
		logger:             logger,
		tokenUsageRepo:     tokenUsageRepo,
		tokenBudget:        tokenBudget,
		tokenBudgetManager: budgetManager,
		eventChan:          nil,
	}
}

func (t *Thinking) SetEventChan(ch chan<- ThinkingEvent) {
	t.eventChan = ch
}

func (t *Thinking) sendEvent(event ThinkingEvent) {
	if t.eventChan != nil {
		select {
		case t.eventChan <- event:
		default:
		}
	}
}

func (t *Thinking) CalculateMaxHistoryCount() int {
	if t.tokenBudgetManager == nil {
		return t.calculateStaticMaxHistoryCount()
	}

	maxRounds := t.tokenBudgetManager.CalculateDynamicMaxHistoryCount()

	t.logger.Debug(i18n.T("brain.calc_history_dynamic"),
		logging.Int(i18n.T("brain.max_tokens"), t.modelConfig.MaxTokens),
		logging.Int(i18n.T("brain.max_rounds"), maxRounds),
		logging.Int(i18n.T("brain.avg_tokens_per_round"), t.tokenBudgetManager.GetAvgTokensPerRound()),
		logging.Int64(i18n.T("brain.total_rounds"), t.tokenBudgetManager.GetTotalRounds()))

	return maxRounds
}

func (t *Thinking) calculateStaticMaxHistoryCount() int {
	if t.modelConfig.MaxTokens <= 0 || t.tokenBudget == nil {
		return 4
	}

	availableTokens := t.modelConfig.MaxTokens - t.tokenBudget.ReservedOutputTokens

	if availableTokens <= 0 {
		return t.tokenBudget.MinHistoryRounds
	}

	maxRounds := availableTokens / t.tokenBudget.AvgTokensPerRound

	if maxRounds < t.tokenBudget.MinHistoryRounds {
		return t.tokenBudget.MinHistoryRounds
	}

	t.logger.Debug(i18n.T("brain.calc_history_static"),
		logging.Int(i18n.T("brain.max_tokens"), t.modelConfig.MaxTokens),
		logging.Int(i18n.T("brain.reserved_output_tokens"), t.tokenBudget.ReservedOutputTokens),
		logging.Int(i18n.T("brain.available_tokens"), availableTokens),
		logging.Int(i18n.T("brain.avg_tokens_per_round"), t.tokenBudget.AvgTokensPerRound),
		logging.Int(i18n.T("brain.max_rounds"), maxRounds))

	return maxRounds
}

func (t *Thinking) Think(question string, history []*core.DialogueMessage, references string, jsonResult bool) (*core.ThinkingResult, error) {
	t.logger.Debug(i18n.T("brain.start_think"),
		logging.String(i18n.T("brain.model"), t.modelConfig.Name),
		logging.String(i18n.T("brain.domain"), t.modelConfig.Domain))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	systemPrompt := t.prompt
	if references != "" {
		systemPrompt += "\n" + references
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	for _, msg := range history {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: question,
	})

	respFormat := openai.ChatCompletionResponseFormatTypeText
	if jsonResult {
		respFormat = openai.ChatCompletionResponseFormatTypeJSONObject
	}

	req := openai.ChatCompletionRequest{
		Model:    t.modelConfig.Name,
		Messages: messages,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: respFormat,
		},
		Stream: true,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}

	if t.modelConfig.Temperature > 0 {
		temperature := float32(t.modelConfig.Temperature)
		req.Temperature = temperature
	}

	if t.modelConfig.MaxTokens > 0 {
		req.MaxTokens = t.modelConfig.MaxTokens
	}

	req.ChatTemplateKwargs = map[string]any{
		"enable_thinking": true,
	}

	startTime := time.Now()

	t.sendEvent(NewThinkingEvent(ThinkingEventStart, i18n.T("brain.start_thinking")))

	stream, err := t.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		t.logger.Error(i18n.T("brain.think_failed"), logging.Err(err))
		t.sendEvent(NewThinkingEvent(ThinkingEventError, err.Error()))
		return nil, fmt.Errorf("think failed: %w", err)
	}
	defer stream.Close()

	var fullContent strings.Builder
	var thinkingContent strings.Builder
	var contentContent strings.Builder
	inThinking := false
	var usage openai.Usage

	for {
		response, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.logger.Error(i18n.T("brain.stream_recv_failed"), logging.Err(err))
			t.sendEvent(NewThinkingEvent(ThinkingEventError, err.Error()))
			return nil, fmt.Errorf("stream receive failed: %w", err)
		}

		if response.Usage != nil && response.Usage.TotalTokens > 0 {
			usage = *response.Usage
		}

		if len(response.Choices) == 0 {
			continue
		}

		delta := response.Choices[0].Delta
		if delta.Content == "" {
			continue
		}

		chunk := delta.Content
		fullContent.WriteString(chunk)

		if strings.Contains(chunk, "<think") || strings.Contains(chunk, "<thinking") {
			inThinking = true
		}

		if inThinking {
			thinkingContent.WriteString(chunk)
			t.sendEvent(NewThinkingEvent(ThinkingEventChunk, chunk))
			if strings.Contains(chunk, "</think") || strings.Contains(chunk, "</thinking") {
				inThinking = false
			}
		} else {
			contentContent.WriteString(chunk)
			t.sendEvent(NewThinkingEvent(ThinkingEventChunk, chunk))
		}
	}

	duration := time.Since(startTime).Milliseconds()

	content := strings.TrimSpace(fullContent.String())

	if strings.Contains(content, "<thinking>") {
		t.logger.Info(i18n.T("brain.think_process"),
			logging.String(i18n.T("brain.content"), content))
	}

	t.logger.Info("[左脑] 模型返回原始内容",
		logging.String("content", content),
		logging.Int("content_length", len(content)))

	var result core.ThinkingResult
	jsonContent := extractJSON(content)
	if err := json.Unmarshal([]byte(jsonContent), &result); err != nil {
		t.logger.Warn(i18n.T("brain.parse_result_failed"),
			logging.Err(err),
			logging.String(i18n.T("brain.raw_content"), content))
		result = core.ThinkingResult{
			Answer:    content,
			Intent:    "",
			Keywords:  []string{},
			CanAnswer: true,
		}
	}

	t.logger.Info("[左脑] 思考结果解析",
		logging.String("intent", result.Intent),
		logging.String("keywords", fmt.Sprintf("%v", result.Keywords)),
		logging.Bool("useless", result.Useless),
		logging.Bool("can_answer", result.CanAnswer),
		logging.String("answer", result.Answer))

	if t.tokenUsageRepo != nil {
		tokenUsage := &entity.TokenUsage{
			Model:            t.modelConfig.Name,
			Duration:         duration,
			CompletionTokens: int(usage.CompletionTokens),
			TotalTokens:      int(usage.TotalTokens),
			PromptTokens:     int(usage.PromptTokens),
			CreatedAt:        time.Now(),
		}
		if err := t.tokenUsageRepo.Save(tokenUsage); err != nil {
			t.logger.Warn(i18n.T("brain.save_token_failed"), logging.Err(err))
		} else {
			t.logger.Debug(i18n.T("brain.token_saved"),
				logging.String(i18n.T("brain.model"), tokenUsage.Model),
				logging.Int(i18n.T("brain.total_tokens"), tokenUsage.TotalTokens),
				logging.Int64("duration_ms", tokenUsage.Duration))
		}
	}

	if t.tokenBudgetManager != nil {
		t.tokenBudgetManager.RecordUsage(
			int(usage.PromptTokens),
			int(usage.CompletionTokens),
		)
	}

	t.logger.Debug(i18n.T("brain.think_complete"),
		logging.String(i18n.T("brain.intent"), result.Intent),
		logging.String(i18n.T("brain.keywords_count"), fmt.Sprintf("%d", len(result.Keywords))),
		logging.String(i18n.T("brain.can_answer"), fmt.Sprintf("%v", result.CanAnswer)))

	t.sendEvent(NewThinkingEventWithProgress(ThinkingEventComplete, result.Answer, 100))

	return &result, nil
}

func (t *Thinking) GetSystemPrompt() string {
	if t.prompt != "" {
		return t.prompt
	}
	return t.systemPrompt
}

func (t *Thinking) ThinkWithTools(question string, history []*core.DialogueMessage, tools []*core.ToolSchema, customSystemPrompt ...string) (*core.ToolCallResult, error) {
	t.logger.Info(i18n.T("brain.right_prepare_skill"),
		logging.String(i18n.T("brain.question"), question),
		logging.Int(i18n.T("brain.tools_count"), len(tools)))

	t.sendEvent(NewThinkingEvent(ThinkingEventStart, i18n.T("brain.right_prepare_skill")))

	if len(tools) == 0 {
		t.logger.Warn(i18n.T("brain.no_skill"))
		return &core.ToolCallResult{Answer: i18n.T("brain.no_available_skill")}, nil
	}

	for i, tool := range tools {
		t.logger.Info(i18n.T("brain.tool_detail"),
			logging.Int(i18n.T("brain.index"), i),
			logging.String(i18n.T("brain.function"), tool.Name),
			logging.String(i18n.T("brain.description"), tool.Description),
			logging.Any(i18n.T("brain.params"), tool.Params))
	}

	ollamaTools := make([]openai.Tool, 0, len(tools))
	for _, tool := range tools {
		t.logger.Info("[右脑] 工具参数详情",
			logging.String("name", tool.Name),
			logging.Any("params", tool.Params))

		description := tool.Description
		if tool.Guidance != "" {
			description += "\n\n## 使用指南\n" + tool.Guidance
		}
		if tool.OutputFormat != "" {
			description += "\n\n## 输出格式\n" + tool.OutputFormat
		}

		ollamaTools = append(ollamaTools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: description,
				Parameters:  tool.Params,
			},
		})
	}

	ctx := context.Background()

	var systemPrompt string
	if len(customSystemPrompt) > 0 && customSystemPrompt[0] != "" {
		systemPrompt = customSystemPrompt[0] + `

## 工具调用指南

重要规则：
1. 如果用户的请求可以通过现有工具满足，请调用相应的工具
2. 如果用户的请求无法通过现有工具满足，请直接回答用户，不要调用工具
3. 调用工具时，确保传递正确的参数
4. 不要编造工具，只能使用提供的工具
5. 仔细阅读每个工具的使用指南和输出格式说明`
	} else {
		systemPrompt = `你是一个工具调用助手。你的职责是根据用户的请求，从可用的工具中选择合适的工具并调用。

重要规则：
1. 如果用户的请求可以通过现有工具满足，请调用相应的工具
2. 如果用户的请求无法通过现有工具满足，请直接回答用户，不要调用工具
3. 调用工具时，确保传递正确的参数
4. 不要编造工具，只能使用提供的工具
5. 仔细阅读每个工具的使用指南和输出格式说明`
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}

	for _, msg := range history {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: question,
	})

	startTime := time.Now()
	req := openai.ChatCompletionRequest{
		Model:      t.modelConfig.Name,
		Messages:   messages,
		Tools:      ollamaTools,
		ToolChoice: "auto",
	}

	t.logger.Info("[右脑] 请求详情",
		logging.String("model", t.modelConfig.Name),
		logging.Int("tools_count", len(ollamaTools)),
		logging.Any("tools", ollamaTools))

	resp, err := t.client.CreateChatCompletion(ctx, req)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		t.logger.Error(i18n.T("brain.right_skill_call_failed"), logging.Err(err))
		t.sendEvent(NewThinkingEvent(ThinkingEventError, err.Error()))
		return nil, fmt.Errorf("skill call failed: %w", err)
	}

	if t.tokenUsageRepo != nil {
		usage := &entity.TokenUsage{
			Model:            t.modelConfig.Name,
			Duration:         duration,
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
			PromptTokens:     int(resp.Usage.PromptTokens),
			CreatedAt:        time.Now(),
		}
		_ = t.tokenUsageRepo.Save(usage)
	}

	if t.tokenBudgetManager != nil {
		t.tokenBudgetManager.RecordUsage(
			int(resp.Usage.PromptTokens),
			int(resp.Usage.CompletionTokens),
		)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no think result")
	}

	choice := resp.Choices[0]

	t.logger.Info("[右脑] 模型响应",
		logging.String("content", choice.Message.Content),
		logging.Int("tool_calls_count", len(choice.Message.ToolCalls)),
		logging.Bool("has_function_call", choice.Message.FunctionCall != nil))

	if len(choice.Message.ToolCalls) > 0 {
		toolCall := choice.Message.ToolCalls[0]
		t.logger.Info(i18n.T("brain.right_will_call_tool"),
			logging.String(i18n.T("brain.function"), toolCall.Function.Name),
			logging.String(i18n.T("brain.arguments"), toolCall.Function.Arguments),
			logging.String("tool_call_id", toolCall.ID))

		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			t.logger.Error(i18n.T("brain.parse_func_params_failed"), logging.Err(err))
			t.sendEvent(NewThinkingEvent(ThinkingEventError, err.Error()))
			return nil, fmt.Errorf("parse func params failed: %w", err)
		}

		t.sendEvent(NewToolCallEvent(toolCall.Function.Name, args))

		return &core.ToolCallResult{
			Function: &core.ToolCallFunction{
				Name:      toolCall.Function.Name,
				Arguments: args,
			},
			ToolCallID: toolCall.ID,
			NoCall:     false,
		}, nil
	}

	if choice.Message.FunctionCall != nil {
		funcCall := choice.Message.FunctionCall
		t.logger.Info(i18n.T("brain.right_will_call_tool"),
			logging.String(i18n.T("brain.function"), funcCall.Name),
			logging.String(i18n.T("brain.arguments"), funcCall.Arguments))

		var args map[string]interface{}
		if err := json.Unmarshal([]byte(funcCall.Arguments), &args); err != nil {
			t.logger.Error(i18n.T("brain.parse_func_params_failed"), logging.Err(err))
			t.sendEvent(NewThinkingEvent(ThinkingEventError, err.Error()))
			return nil, fmt.Errorf("parse func params failed: %w", err)
		}

		t.sendEvent(NewToolCallEvent(funcCall.Name, args))

		return &core.ToolCallResult{
			Function: &core.ToolCallFunction{
				Name:      funcCall.Name,
				Arguments: args,
			},
			NoCall: false,
		}, nil
	}

	content := strings.TrimSpace(choice.Message.Content)
	if content == "" {
		t.logger.Info(i18n.T("brain.right_no_tool_call"))
		return &core.ToolCallResult{Answer: "", NoCall: true}, nil
	}

	var ollamaToolCall struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(content), &ollamaToolCall); err == nil && ollamaToolCall.Name != "" {
		t.logger.Info("[右脑] Ollama 格式工具调用",
			logging.String("function", ollamaToolCall.Name),
			logging.Any("arguments", ollamaToolCall.Arguments))
		return &core.ToolCallResult{
			Function: &core.ToolCallFunction{
				Name:      ollamaToolCall.Name,
				Arguments: ollamaToolCall.Arguments,
			},
			NoCall: false,
		}, nil
	}

	t.logger.Info(i18n.T("brain.right_no_tool_call"))
	return &core.ToolCallResult{
		Answer: content,
		NoCall: true,
	}, nil
}

func (t *Thinking) ReturnFuncResult(toolCallID string, name string, result string, originalArgs map[string]interface{}, history []*core.DialogueMessage, tools []*core.ToolSchema, question string) (string, error) {
	t.logger.Debug(i18n.T("brain.return_func_result"),
		logging.String(i18n.T("brain.function"), name),
		logging.String(i18n.T("brain.result"), result),
		logging.String("tool_call_id", toolCallID))

	t.sendEvent(NewToolResultEvent(name, result))

	ctx := context.Background()

	systemPrompt := `你是一个工具调用助手。你的职责是根据用户的请求，从可用的工具中选择合适的工具并调用。

重要规则：
1. 如果用户的请求可以通过现有工具满足，请调用相应的工具
2. 如果用户的请求无法通过现有工具满足，请直接回答用户，不要调用工具
3. 调用工具时，确保传递正确的参数
4. 不要编造工具，只能使用提供的工具`

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}

	for _, msg := range history {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: question,
	})

	argsBytes, err := json.Marshal(originalArgs)
	if err != nil {
		t.logger.Error(i18n.T("brain.serialize_params_failed"), logging.Err(err))
		return "", fmt.Errorf("serialize params failed: %w", err)
	}

	toolCalls := []openai.ToolCall{
		{
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      name,
				Arguments: string(argsBytes),
			},
		},
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:      openai.ChatMessageRoleAssistant,
		ToolCalls: toolCalls,
	})

	messages = append(messages, openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCallID,
		Content:    result,
	})

	startTime := time.Now()
	req := openai.ChatCompletionRequest{
		Model:    t.modelConfig.Name,
		Messages: messages,
	}

	req.ChatTemplateKwargs = map[string]any{
		"enable_thinking": true,
	}

	resp, err := t.client.CreateChatCompletion(ctx, req)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		t.logger.Error(i18n.T("brain.return_func_result_failed"), logging.Err(err))
		t.sendEvent(NewThinkingEvent(ThinkingEventError, err.Error()))
		return "", fmt.Errorf("return func result failed: %w", err)
	}

	if t.tokenUsageRepo != nil {
		usage := &entity.TokenUsage{
			Model:            t.modelConfig.Name,
			Duration:         duration,
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
			PromptTokens:     int(resp.Usage.PromptTokens),
			CreatedAt:        time.Now(),
		}
		_ = t.tokenUsageRepo.Save(usage)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response result")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	t.logger.Info(i18n.T("brain.get_final_response"), logging.String(i18n.T("brain.content"), content))

	t.sendEvent(NewThinkingEventWithProgress(ThinkingEventComplete, content, 100))

	return content, nil
}

func extractJSON(content string) string {
	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
	}

	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
	}

	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	if start == -1 {
		return content
	}

	braceCount := 0
	end := -1
	for i, ch := range content[start:] {
		if ch == '{' {
			braceCount++
		} else if ch == '}' {
			braceCount--
			if braceCount == 0 {
				end = start + i + 1
				break
			}
		}
	}

	if end > start {
		return content[start:end]
	}

	return content
}
