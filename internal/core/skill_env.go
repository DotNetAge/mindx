package core

import (
	"github.com/DotNetAge/mindx/internal/core/skillstore"
)

// SkillSubProcessEnvResolver 返回当前 Agent 挂载技能的子进程环境解析器，
// 经 agents.WithSubProcessEnv 注入 goharness Runtime（ToolContext 透传，
// Bash 执行时叠加到子进程环境）。
//
// 解析语义与技能准备流程（skill.prepare）一致：仅对象形态 requires.envs
// 条目参与凭据注入，逐条经 ResolveSkillCredential 三层链（凭据库 → 进程
// 环境 → 缺失）取值；字符串形态条目维持进程环境直通语义，不在注入范围。
// 两个技能声明同名变量时按目录序（Catalog 已排序）后者覆盖，保证确定性。
//
// 每次 Bash 执行时实时解析：技能热加载与凭据收集（skill.credential.set）
// 即刻生效，无需缓存失效；技能数量有限，SKILL.md 头部读取开销可接受。
// 返回 nil 表示无注入值（Bash 维持父进程环境继承行为）；
// 返回值可能含敏感凭据，只进子进程环境，严禁写日志。
func (a *App) SkillSubProcessEnvResolver(agentName string, declared []string) func() map[string]string {
	return func() map[string]string {
		if a.skills == nil {
			return nil
		}
		extras := make(map[string]string)
		for _, sk := range a.skills.Catalog(agentName, declared) {
			detail, _, err := skillstore.LoadSkillDetailFromDir(sk.RootDir, "filesystem")
			if err != nil || detail == nil || detail.Requires == nil {
				continue
			}
			for _, req := range detail.Requires.Envs {
				if req.Name == "" {
					continue
				}
				if v, ok := ResolveSkillCredential(a.credStore, sk.Name, req.Name); ok && v != "" {
					extras[req.Name] = v
				}
			}
		}
		if len(extras) == 0 {
			return nil
		}
		return extras
	}
}
