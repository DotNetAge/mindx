package brain

import (
	"context"
	"fmt"
	"mindx/internal/core"
	apperrors "mindx/internal/errors"
	"mindx/internal/usecase/skills"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
)

const maxToolCalls = 10

type ToolCaller struct {
	skillMgr *skills.SkillMgr
	logger   logging.Logger
}

func NewToolCaller(skillMgr *skills.SkillMgr, logger logging.Logger) *ToolCaller {
	return &ToolCaller{
		skillMgr: skillMgr,
		logger:   logger,
	}
}

func (tc *ToolCaller) ExecuteToolCall(
	ctx context.Context,
	thinking core.Thinking,
	question string,
	historyDialogue []*core.DialogueMessage,
	tools []*core.ToolSchema,
	customSystemPrompt ...string,
) (string, error) {
	tc.logger.Info(i18n.T("brain.use_tool_decision"), logging.Int(i18n.T("brain.tools_count"), len(tools)))

	currentHistory := make([]*core.DialogueMessage, len(historyDialogue))
	copy(currentHistory, historyDialogue)

	var finalAnswer string
	var callCount int

	// 第一步：让 LLM 决定调用哪些工具
	toolCallResult, err := thinking.ThinkWithTools(ctx, question, currentHistory, tools, customSystemPrompt...)
	if err != nil {
		return "", apperrors.Wrap(err, apperrors.ErrTypeModel, "tool call decision failed")
	}

	if toolCallResult.NoCall {
		tc.logger.Debug(i18n.T("brain.no_tool_call_decision"))
		return toolCallResult.Answer, nil
	}
	if toolCallResult.Function == nil {
		tc.logger.Debug("没有函数调用")
		return toolCallResult.Answer, nil
	}

	// 将首次结果转为待执行队列
	pendingCalls := toolCallResult.ToolCalls
	if len(pendingCalls) == 0 {
		// 兼容旧路径（FunctionCall / Ollama JSON 格式）
		pendingCalls = []core.ToolCallItem{{
			ToolCallID: toolCallResult.ToolCallID,
			Function:   toolCallResult.Function,
		}}
	}

	// 循环：执行 → 回传 → 如果模型要求继续则再执行
	for len(pendingCalls) > 0 && callCount < maxToolCalls {
		tc.logger.Info("批量执行工具",
			logging.Int("count", len(pendingCalls)),
			logging.Int("round_call_count", callCount))

		execResults := make([]core.ToolExecResult, 0, len(pendingCalls))
		for _, item := range pendingCalls {
			if callCount >= maxToolCalls {
				tc.logger.Warn("达到最大工具调用次数，跳过剩余工具")
				break
			}
			callCount++

			tc.logger.Info(i18n.T("brain.execute_skill"),
				logging.String(i18n.T("brain.function"), item.Function.Name),
				logging.String(i18n.T("brain.arguments"), fmt.Sprintf("%v", item.Function.Arguments)))

			funcResult, execErr := tc.skillMgr.ExecuteFunc(core.ToolCallFunction{
				Name:      item.Function.Name,
				Arguments: item.Function.Arguments,
			})

			er := core.ToolExecResult{
				ToolCallID:   item.ToolCallID,
				FunctionName: item.Function.Name,
				Arguments:    item.Function.Arguments,
			}
			if execErr != nil {
				tc.logger.Warn("工具执行失败",
					logging.String("function", item.Function.Name),
					logging.Err(execErr))
				er.Error = execErr.Error()
				er.Result = fmt.Sprintf("执行失败: %s", execErr.Error())
			} else {
				tc.logger.Info(i18n.T("brain.skill_exec_success"),
					logging.String(i18n.T("brain.result"), funcResult))
				er.Result = funcResult
			}
			execResults = append(execResults, er)
		}

		// 批量回传所有结果给 LLM
		batchResult, err := thinking.ReturnFuncResults(ctx, execResults, currentHistory, tools, question)
		if err != nil {
			return "", apperrors.Wrap(err, apperrors.ErrTypeModel, "return func results failed")
		}

		if batchResult.NoCall {
			finalAnswer = batchResult.Answer
			tc.logger.Info("工具调用完成", logging.String("answer", finalAnswer))
			break
		}

		// 模型要求继续调用 — 直接进入下一轮执行，不再调 ThinkWithTools
		pendingCalls = batchResult.ToolCalls
		if len(pendingCalls) == 0 && batchResult.Function != nil {
			pendingCalls = []core.ToolCallItem{{
				ToolCallID: batchResult.ToolCallID,
				Function:   batchResult.Function,
			}}
		}

		tc.logger.Info("模型要求继续调用工具", logging.Int("new_calls", len(pendingCalls)))
	}

	if callCount >= maxToolCalls {
		tc.logger.Warn("达到最大工具调用次数", logging.Int("max_calls", maxToolCalls))
	}

	return finalAnswer, nil
}

func (tc *ToolCaller) SearchTools(keywords []string) ([]core.ToolSchema, error) {
	skills, err := tc.skillMgr.SearchSkills(keywords...)
	if err != nil {
		return nil, err
	}

	schemas := make([]core.ToolSchema, 0, len(skills))
	for _, skill := range skills {
		name := skill.GetName()

		info, exists := tc.skillMgr.GetSkillInfo(name)
		if !exists {
			tc.logger.Warn(i18n.T("brain.skill_info_not_exist"), logging.String(i18n.T("brain.function"), name))
			schemas = append(schemas, core.ToolSchema{
				Name:        name,
				Description: "",
				Params:      map[string]any{},
			})
			continue
		}

		tc.logger.Debug("获取技能信息",
			logging.String("skill", name),
			logging.String("guidance_len", fmt.Sprintf("%d", len(info.Def.Guidance))),
			logging.String("output_format_len", fmt.Sprintf("%d", len(info.Def.OutputFormat))))

		params := make(map[string]interface{})
		if info.Def != nil && info.Def.Parameters != nil {
			properties := make(map[string]interface{})
			required := []string{}
			for paramName, paramDef := range info.Def.Parameters {
				desc := paramDef.Description
				if !paramDef.Required {
					desc += "（可选）"
				}
				properties[paramName] = map[string]interface{}{
					"type":        paramDef.Type,
					"description": desc,
				}
				if paramDef.Required {
					required = append(required, paramName)
				}
			}
			params = map[string]interface{}{
				"type":       "object",
				"properties": properties,
				"required":   required,
			}
		}

		schemas = append(schemas, core.ToolSchema{
			Name:         info.Def.Name,
			Description:  info.Def.Description,
			Params:       params,
			OutputFormat: info.Def.OutputFormat,
			Guidance:     info.Def.Guidance,
		})

		if info.Def.Guidance != "" || info.Def.OutputFormat != "" {
			tc.logger.Debug("技能包含引导信息",
				logging.String("skill", info.Def.Name),
				logging.String("guidance", info.Def.Guidance),
				logging.String("output_format", info.Def.OutputFormat))
		}
	}

	return schemas, nil
}
