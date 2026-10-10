package skillstore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// qualityIssuesByCode 按 Code 过滤质量报告条目。
func qualityIssuesByCode(rep *QualityReport, code string) []QualityIssue {
	var out []QualityIssue
	for _, iss := range rep.Issues {
		if iss.Code == code {
			out = append(out, iss)
		}
	}
	return out
}

// TestCheckQuality_EnvsObjectForm 校验 requires.envs 对象形态的两条质量规则：
// prompt 缺失告警；sensitive 条目正文明文赋值样例告警（带行号）。
func TestCheckQuality_EnvsObjectForm(t *testing.T) {
	dir := writeSkillDir(t, `---
name: env-quality-demo
description: 测试 envs 对象形态质量校验时使用
metadata:
  requires:
    envs:
      - name: LARK_APP_ID
      - name: LARK_APP_SECRET
        prompt: 在飞书开放平台应用凭证页获取
        sensitive: true
---
正文引用 envs 对象条目质量校验。
export LARK_APP_SECRET=sk-plaintext-123
`)
	rep, err := CheckQuality(dir)
	require.NoError(t, err)

	envIssues := qualityIssuesByCode(rep, "quality-envs")
	require.Len(t, envIssues, 2)

	// LARK_APP_ID 缺 prompt → 告警
	require.Contains(t, envIssues[0].Message, "LARK_APP_ID")
	require.Contains(t, envIssues[0].Message, "prompt")
	// LARK_APP_SECRET sensitive 明文样例 → 告警带行号
	require.Contains(t, envIssues[1].Message, "LARK_APP_SECRET")
	require.Equal(t, 13, envIssues[1].Line)
}

// TestCheckQuality_EnvsSensitiveExempt sensitive 条目正文的占位符与自引用赋值
// 不告警（<your-key>、$VAR、${VAR} 三类豁免，防误报）。
func TestCheckQuality_EnvsSensitiveExempt(t *testing.T) {
	dir := writeSkillDir(t, `---
name: env-exempt-demo
description: 测试 sensitive 明文豁免规则时使用
metadata:
  requires:
    envs:
      - name: MY_TOKEN
        prompt: 到控制台申请
        sensitive: true
---
配置示例：export MY_TOKEN=<your-token> 或引用现有值。
export MY_TOKEN=$MY_TOKEN
export MY_TOKEN=${MY_TOKEN}
`)
	rep, err := CheckQuality(dir)
	require.NoError(t, err)
	require.Empty(t, qualityIssuesByCode(rep, "quality-envs"))
}

// TestCheckQuality_EnvsOtherVarNoFalsePositive 其它变量的赋值行不触发
// sensitive 检查（按变量名精确匹配，防误报）。
func TestCheckQuality_EnvsOtherVarNoFalsePositive(t *testing.T) {
	dir := writeSkillDir(t, `---
name: env-precise-demo
description: 测试按变量名精确匹配时使用
metadata:
  requires:
    envs:
      - name: MY_TOKEN
        prompt: 到控制台申请
        sensitive: true
---
export OTHER_TOKEN=sk-not-mine
`)
	rep, err := CheckQuality(dir)
	require.NoError(t, err)
	require.Empty(t, qualityIssuesByCode(rep, "quality-envs"))
}
