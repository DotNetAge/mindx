package brain

import (
	"encoding/json"
	"mindx/internal/core"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "干净JSON",
			input:    `{"answer":"hi","intent":"greet"}`,
			expected: `{"answer":"hi","intent":"greet"}`,
		},
		{
			name:     "markdown json代码块",
			input:    "```json\n{\"answer\":\"hi\"}\n```",
			expected: `{"answer":"hi"}`,
		},
		{
			name:     "markdown无标签代码块",
			input:    "```\n{\"answer\":\"hi\"}\n```",
			expected: `{"answer":"hi"}`,
		},
		{
			name:     "JSON前有文字",
			input:    `Here is the result: {"answer":"hi"}`,
			expected: `{"answer":"hi"}`,
		},
		{
			name:     "JSON后有文字",
			input:    `{"answer":"hi"} Hope this helps!`,
			expected: `{"answer":"hi"}`,
		},
		{
			name:     "嵌套花括号在字符串值中",
			input:    `{"answer":"a {b} c","intent":"test"}`,
			expected: `{"answer":"a {b} c","intent":"test"}`,
		},
		{
			name:     "深层嵌套",
			input:    `{"a":{"b":{"c":1}}}`,
			expected: `{"a":{"b":{"c":1}}}`,
		},
		{
			name:     "无JSON纯文本",
			input:    "I don't know what to say",
			expected: "I don't know what to say",
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
		{
			name:     "只有空白",
			input:    "   ",
			expected: "",
		},
		{
			name:     "未闭合花括号",
			input:    `{"answer":"hi"`,
			expected: `{"answer":"hi"`,
		},
		{
			name:     "多个JSON对象取第一个",
			input:    `{"a":1} {"b":2}`,
			expected: `{"a":1}`,
		},
		{
			name:     "think标签包裹JSON",
			input:    `<think>reasoning about intent</think>{"answer":"","intent":"天气"}`,
			expected: `{"answer":"","intent":"天气"}`,
		},
		{
			name:     "thinking标签包裹JSON",
			input:    "<thinking>some reasoning</thinking>\n{\"intent\":\"stock\"}",
			expected: `{"intent":"stock"}`,
		},
		{
			name:     "代码块带额外空白",
			input:    "```json  \n  {\"answer\":\"hi\"}  \n  ```",
			expected: `{"answer":"hi"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractJSON(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractJSONThenUnmarshal(t *testing.T) {
	tests := []struct {
		name           string
		rawOutput      string
		expectIntent   string
		expectKeywords []string
		expectCanAnswer bool
		expectParseOK  bool // false 表示 JSON 解析失败，走 fallback
	}{
		{
			name:           "干净模型输出",
			rawOutput:      `{"answer":"","intent":"查询天气","useless":false,"keywords":["天气","北京"],"can_answer":false}`,
			expectIntent:   "查询天气",
			expectKeywords: []string{"天气", "北京"},
			expectCanAnswer: false,
			expectParseOK:  true,
		},
		{
			name:           "markdown包裹",
			rawOutput:      "```json\n{\"answer\":\"\",\"intent\":\"计算\",\"keywords\":[\"计算\"],\"can_answer\":false}\n```",
			expectIntent:   "计算",
			expectKeywords: []string{"计算"},
			expectCanAnswer: false,
			expectParseOK:  true,
		},
		{
			name:           "thinking标签包裹",
			rawOutput:      "<think>user wants weather</think>{\"answer\":\"\",\"intent\":\"weather\",\"keywords\":[\"weather\"],\"can_answer\":false}",
			expectIntent:   "weather",
			expectKeywords: []string{"weather"},
			expectCanAnswer: false,
			expectParseOK:  true,
		},
		{
			name:           "垃圾输出走fallback",
			rawOutput:      "I'm sorry, I can't help with that.",
			expectIntent:   "",
			expectKeywords: []string{},
			expectCanAnswer: true,
			expectParseOK:  false,
		},
		{
			name:           "useless场景",
			rawOutput:      `{"answer":"你好！","intent":"闲聊","useless":true,"keywords":[],"can_answer":true}`,
			expectIntent:   "闲聊",
			expectKeywords: []string{},
			expectCanAnswer: true,
			expectParseOK:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jsonContent := extractJSON(tc.rawOutput)

			var result core.ThinkingResult
			err := json.Unmarshal([]byte(jsonContent), &result)

			if tc.expectParseOK {
				require.NoError(t, err, "JSON 解析应该成功")
				assert.Equal(t, tc.expectIntent, result.Intent)
				assert.Equal(t, tc.expectKeywords, result.Keywords)
				assert.Equal(t, tc.expectCanAnswer, result.CanAnswer)
			} else {
				// 模拟 thinking.go:280-285 的 fallback 逻辑
				if err != nil {
					result = core.ThinkingResult{
						Answer:    tc.rawOutput,
						Intent:    "",
						Keywords:  []string{},
						CanAnswer: true,
					}
				}
				assert.Equal(t, tc.expectIntent, result.Intent)
				assert.Equal(t, tc.expectKeywords, result.Keywords)
				assert.Equal(t, tc.expectCanAnswer, result.CanAnswer)
			}
		})
	}
}

func TestThinkingResultUnmarshal(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expectError bool
		check       func(t *testing.T, r *core.ThinkingResult)
	}{
		{
			name: "完整字段",
			json: `{"answer":"hi","intent":"greet","useless":true,"keywords":["hello"],"can_answer":true,"has_schedule":false,"schedule_name":"","schedule_cron":"","schedule_message":"","send_to":""}`,
			check: func(t *testing.T, r *core.ThinkingResult) {
				assert.Equal(t, "hi", r.Answer)
				assert.Equal(t, "greet", r.Intent)
				assert.True(t, r.Useless)
				assert.Equal(t, []string{"hello"}, r.Keywords)
				assert.True(t, r.CanAnswer)
				assert.False(t, r.HasSchedule)
				assert.Empty(t, r.SendTo)
			},
		},
		{
			name: "缺少可选字段使用默认值",
			json: `{"answer":"hi","intent":"greet"}`,
			check: func(t *testing.T, r *core.ThinkingResult) {
				assert.Equal(t, "hi", r.Answer)
				assert.Equal(t, "greet", r.Intent)
				assert.False(t, r.Useless)
				assert.Nil(t, r.Keywords)
				assert.False(t, r.CanAnswer)
			},
		},
		{
			name: "空keywords数组",
			json: `{"keywords":[]}`,
			check: func(t *testing.T, r *core.ThinkingResult) {
				assert.NotNil(t, r.Keywords)
				assert.Len(t, r.Keywords, 0)
			},
		},
		{
			name: "schedule字段",
			json: `{"has_schedule":true,"schedule_name":"drink","schedule_cron":"0 9 * * *","schedule_message":"drink water"}`,
			check: func(t *testing.T, r *core.ThinkingResult) {
				assert.True(t, r.HasSchedule)
				assert.Equal(t, "drink", r.ScheduleName)
				assert.Equal(t, "0 9 * * *", r.ScheduleCron)
				assert.Equal(t, "drink water", r.ScheduleMessage)
			},
		},
		{
			name: "send_to字段",
			json: `{"send_to":"wechat","answer":"forwarded"}`,
			check: func(t *testing.T, r *core.ThinkingResult) {
				assert.Equal(t, "wechat", r.SendTo)
				assert.Equal(t, "forwarded", r.Answer)
			},
		},
		{
			name: "未知字段被忽略",
			json: `{"answer":"hi","unknown_field":123,"another":true}`,
			check: func(t *testing.T, r *core.ThinkingResult) {
				assert.Equal(t, "hi", r.Answer)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var result core.ThinkingResult
			err := json.Unmarshal([]byte(tc.json), &result)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tc.check(t, &result)
			}
		})
	}
}
