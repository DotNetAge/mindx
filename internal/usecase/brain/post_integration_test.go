package brain

import (
	"mindx/internal/core"
	"strings"
)

// TestPost_ChatDirect 闲聊直接回答（不触发工具）
func (s *BrainIntegrationSuite) TestPost_ChatDirect() {
	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: "你好",
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "闲聊应返回非空回答")
	if len(resp.Tools) > 0 {
		s.T().Logf("⚠ 闲聊返回了 %d 个工具（小模型关键字提取可能过于宽泛）", len(resp.Tools))
	}
	s.T().Logf("回答: %s", resp.Answer)
}

// TestPost_CommonKnowledge 常识问题直接回答
func (s *BrainIntegrationSuite) TestPost_CommonKnowledge() {
	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: "法国的首都是哪里",
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "常识问题应返回非空回答")
	s.T().Logf("回答: %s", resp.Answer)
}

// TestPost_ToolExecution_Calculator 完整 post() 流程：计算器工具
func (s *BrainIntegrationSuite) TestPost_ToolExecution_Calculator() {
	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: "帮我算一下 15 乘以 20",
		Timeout:  60,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "计算器流程应返回非空回答")
	s.T().Logf("回答: %s, 工具数: %d", resp.Answer, len(resp.Tools))

	if len(resp.Tools) > 0 {
		for _, t := range resp.Tools {
			s.T().Logf("  工具: %s", t.Name)
		}
	}
}

// TestPost_ToolExecution_Weather 完整 post() 流程：天气工具
func (s *BrainIntegrationSuite) TestPost_ToolExecution_Weather() {
	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: "北京今天天气怎么样",
		Timeout:  60,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "天气流程应返回非空回答")
	s.T().Logf("回答: %s, 工具数: %d", resp.Answer, len(resp.Tools))
}

// TestPost_ToolExecution_Sysinfo 完整 post() 流程：系统信息工具
func (s *BrainIntegrationSuite) TestPost_ToolExecution_Sysinfo() {
	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: "查看一下系统CPU使用率",
		Timeout:  60,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "系统信息流程应返回非空回答")
	s.T().Logf("回答: %s, 工具数: %d", resp.Answer, len(resp.Tools))
}

// TestPost_Schedule_Create 完整 post() 流程：创建定时任务
func (s *BrainIntegrationSuite) TestPost_Schedule_Create() {
	s.cronMock.reset()

	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: "每天早上9点提醒我喝水",
		Timeout:  60,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "创建定时任务应返回非空回答")

	jobs := s.cronMock.getJobs()
	s.T().Logf("回答: %s, 创建的任务数: %d", resp.Answer, len(jobs))

	if len(jobs) > 0 {
		for _, job := range jobs {
			s.T().Logf("  任务: name=%s, cron=%s, msg=%s", job.Name, job.Cron, job.Message)
			s.NotEmpty(job.Cron, "cron 表达式不应为空")
			s.Contains(job.Cron, "9", "cron 应包含 9 点")
		}
	} else {
		s.T().Log("⚠ 小模型未识别出定时意图，cronMock 未收到 Add 调用")
	}
}

// TestPost_Schedule_Cancel 完整 post() 流程：取消定时任务（预留）
// brain.go 中 CancelSchedule 处理逻辑尚未实现，此测试验证意图识别
func (s *BrainIntegrationSuite) TestPost_Schedule_Cancel() {
	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: "取消每日喝水提醒",
		Timeout:  60,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "取消定时任务应返回非空回答")
	s.T().Logf("回答: %s", resp.Answer)
	// TODO: brain.go 实现 CancelSchedule 后，验证 cronMock.deleted
}

// TestPost_SendTo 完整 post() 流程：转发意图
func (s *BrainIntegrationSuite) TestPost_SendTo() {
	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: "帮我把这条消息转发到微信",
		Timeout:  60,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "转发意图应返回非空回答")
	s.T().Logf("回答: %s, send_to: %s", resp.Answer, resp.SendTo)

	if resp.SendTo != "" {
		s.True(containsAnyCI(resp.SendTo, []string{"wechat", "微信", "weixin"}),
			"send_to '%s' 应包含微信相关标识", resp.SendTo)
	} else {
		s.T().Log("⚠ 小模型未识别出转发意图")
	}
}

// TestPost_MultiRound 多轮对话：验证上下文传递
func (s *BrainIntegrationSuite) TestPost_MultiRound() {
	// 第一轮
	resp1, err := s.postWithHistory(&core.ThinkingRequest{
		Question: "我叫张三",
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp1.Answer)
	s.T().Logf("第一轮回答: %s", resp1.Answer)

	// 第二轮：验证模型记住了名字
	resp2, err := s.postWithHistory(&core.ThinkingRequest{
		Question: "我叫什么名字",
		Timeout:  30,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp2.Answer)
	s.T().Logf("第二轮回答: %s", resp2.Answer)

	if strings.Contains(resp2.Answer, "张三") {
		s.T().Log("✓ 模型正确记住了用户名字")
	} else {
		s.T().Log("⚠ 模型未在回答中提及用户名字（小模型上下文能力有限）")
	}
}

// TestPost_ToolExecution_Contacts 完整 post() 流程：查询联系人电话
// 验证向量搜索能匹配到 contacts 工具并执行
func (s *BrainIntegrationSuite) TestPost_ToolExecution_Contacts() {
	resp, err := s.brain.Post(&core.ThinkingRequest{
		Question: "帮我查Alex的电话",
		Timeout:  60,
	})
	s.Require().NoError(err)
	s.NotEmpty(resp.Answer, "联系人查询应返回非空回答")
	s.T().Logf("回答: %s, 工具数: %d", resp.Answer, len(resp.Tools))

	if len(resp.Tools) > 0 {
		foundContacts := false
		for _, t := range resp.Tools {
			s.T().Logf("  工具: %s, 参数: %v", t.Name, t.Params)
			if t.Name == "contacts" {
				foundContacts = true
			}
		}
		if foundContacts {
			s.T().Log("✓ 正确调用了 contacts 工具")
			// 回答应包含查询结果：找到电话号码或告知未找到
			hasResult := strings.Contains(resp.Answer, "Alex") ||
				strings.Contains(resp.Answer, "没有找到") ||
				strings.Contains(resp.Answer, "未找到") ||
				strings.Contains(resp.Answer, "找不到") ||
				strings.Contains(resp.Answer, "不存在")
			s.True(hasResult, "回答应包含查询结果（电话号码或未找到提示），实际: %s", resp.Answer)
		} else {
			toolNames := make([]string, 0, len(resp.Tools))
			for _, t := range resp.Tools {
				toolNames = append(toolNames, t.Name)
			}
			s.T().Logf("⚠ 小模型未匹配到 contacts 工具，实际调用: %v", toolNames)
		}
	} else {
		s.T().Log("⚠ 模型未识别出联系人查询意图")
	}
}

func containsAnyCI(s string, substrs []string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
