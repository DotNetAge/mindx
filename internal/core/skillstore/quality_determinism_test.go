package skillstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCheckQuality_DeterminismNoScripts 正文代码块出现网络/远程执行命令而
// scripts/ 缺失 → determinism 告警（确定性操作疑似未脚本化）。
func TestCheckQuality_DeterminismNoScripts(t *testing.T) {
	dir := writeSkillDir(t, `---
name: determinism-demo
description: 测试确定性操作脚本化检查时使用
---
按以下命令拉取清单：

`+"```bash"+`
curl -H "Authorization: Bearer $TOKEN" https://example.com/api/list
`+"```"+`
`)
	rep, err := CheckQuality(dir)
	require.NoError(t, err)
	det := qualityIssuesByCode(rep, "determinism")
	require.Len(t, det, 1)
	require.Contains(t, det[0].Message, "scripts/ 缺失")
}

// TestCheckQuality_DeterminismRouted scripts/ 存在且正文已引用脚本 → 不告警。
func TestCheckQuality_DeterminismRouted(t *testing.T) {
	dir := writeSkillDir(t, `---
name: determinism-routed
description: 测试脚本已路由不告警时使用
---
执行一律调脚本：

`+"```bash"+`
python3 scripts/fetch.py --kind article
`+"```"+`
`)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "fetch.py"), []byte("#!/usr/bin/env python3\n"), 0o755))
	rep, err := CheckQuality(dir)
	require.NoError(t, err)
	require.Empty(t, qualityIssuesByCode(rep, "determinism"))
	require.Empty(t, qualityIssuesByCode(rep, "scripts-unrouted"))
}

// TestCheckQuality_ScriptsUnrouted scripts/ 存在但正文未引用任何脚本 →
// scripts-unrouted 告警（执行路由缺失）。
func TestCheckQuality_ScriptsUnrouted(t *testing.T) {
	dir := writeSkillDir(t, `---
name: determinism-unrouted
description: 测试脚本未路由告警时使用
---
按以下命令拉取清单：

`+"```bash"+`
curl https://example.com/api/list
`+"```"+`
`)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "fetch.py"), []byte("#!/usr/bin/env python3\n"), 0o755))
	rep, err := CheckQuality(dir)
	require.NoError(t, err)
	require.Len(t, qualityIssuesByCode(rep, "scripts-unrouted"), 1)
}

// TestCheckQuality_DeterminismInlineCodeExempt 行内代码中的命令（自指引用）
// 不触发告警；围栏外的普通文本也不触发。
func TestCheckQuality_DeterminismInlineCodeExempt(t *testing.T) {
	dir := writeSkillDir(t, `---
name: determinism-exempt
description: 测试行内代码豁免时使用
---
行内命令如 `+"`curl -s`"+` 只是提及，不是执行块；本技能无网络执行动作。
`)
	rep, err := CheckQuality(dir)
	require.NoError(t, err)
	require.Empty(t, qualityIssuesByCode(rep, "determinism"))
	require.Empty(t, qualityIssuesByCode(rep, "scripts-unrouted"))
}
