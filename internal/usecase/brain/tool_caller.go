package brain

import (
	"mindx/internal/core"
	"mindx/internal/usecase/skills"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"fmt"
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
	thinking core.Thinking,
	question string,
	historyDialogue []*core.DialogueMessage,
	tools []*core.ToolSchema,
) (string, error) {
	tc.logger.Info(i18n.T("brain.use_tool_decision"), logging.Int(i18n.T("brain.tools_count"), len(tools)))

	currentHistory := make([]*core.DialogueMessage, len(historyDialogue))
	copy(currentHistory, historyDialogue)

	var finalAnswer string
	var callCount int

	for callCount < maxToolCalls {
		callCount++
		tc.logger.Info("工具调用", logging.Int("round", callCount))

		toolCallResult, err := thinking.ThinkWithTools(question, currentHistory, tools)
		if err != nil {
			return "", fmt.Errorf(i18n.T("brain.tool_call_decision_failed")+": %w", err)
		}

		if toolCallResult.NoCall {
			tc.logger.Debug(i18n.T("brain.no_tool_call_decision"))
			finalAnswer = toolCallResult.Answer
			break
		}

		if toolCallResult.Function == nil {
			tc.logger.Debug("没有函数调用")
			finalAnswer = toolCallResult.Answer
			break
		}

		tc.logger.Info(i18n.T("brain.execute_skill"),
			logging.String(i18n.T("brain.function"), toolCallResult.Function.Name),
			logging.String(i18n.T("brain.arguments"), fmt.Sprintf("%v", toolCallResult.Function.Arguments)))

		functionCallResult, err := tc.skillMgr.ExecuteFunc(*toolCallResult.Function)
		if err != nil {
			return "", fmt.Errorf(i18n.T("brain.skill_exec_failed")+": %w", err)
		}

		tc.logger.Info(i18n.T("brain.skill_exec_success"), logging.String(i18n.T("brain.result"), functionCallResult))

		answer, err := thinking.ReturnFuncResult(
			toolCallResult.ToolCallID,
			toolCallResult.Function.Name,
			functionCallResult,
			toolCallResult.Function.Arguments,
			currentHistory,
			tools,
			question,
		)
		if err != nil {
			return "", fmt.Errorf(i18n.T("brain.return_func_result_failed")+": %w", err)
		}

		finalAnswer = answer

		currentHistory = append(currentHistory, &core.DialogueMessage{
			Role:    "assistant",
			Content: answer,
		})

		tc.logger.Info("工具调用完成", logging.String("answer", answer))
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
			for paramName, paramDef := range info.Def.Parameters {
				params[paramName] = map[string]interface{}{
					"type":        paramDef.Type,
					"description": paramDef.Description,
					"required":    paramDef.Required,
				}
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
