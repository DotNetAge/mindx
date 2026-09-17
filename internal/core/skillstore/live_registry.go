package skillstore

import (
	"path/filepath"

	"github.com/DotNetAge/goharness/skill"
)

// LiveRegistry 是 Runtime 持有的"活"技能检索视图（不快照）：
// GetSkill 实时反映 Store 当前状态——全局库读取 ReloadGlobal 原子替换后的
// 当前注册表，Agent 级库实时读磁盘。技能增删改即刻对全部会话生效，
// 无需失效或重建 Runtime（PR-PROMPTS 第二节：热加载需实时生效，
// 会话快照会改变热加载语义）。
//
// 与 RegistryFor（快照，供 Catalog 组装提示词目录）的分工：
// 运行时检索走本视图，提示词目录每次构建时经 Store 实时重算。
type LiveRegistry struct {
	store     *Store
	agentName string
}

// 编译期接口检查：实现 goharness 检索 SPI（仅 GetSkill，P4 收窄后契约）。
var _ skill.SkillRegistry = (*LiveRegistry)(nil)

// LiveRegistryFor 返回指定 Agent 的活检索视图（轻量包装，共享 Store 状态）。
// Runtime 创建时应持有该视图而非 RegistryFor 的快照。
func (s *Store) LiveRegistryFor(agentName string) *LiveRegistry {
	return &LiveRegistry{store: s, agentName: agentName}
}

// GetSkill 按名称检索技能：Agent 级库实时读磁盘（同名覆盖全局级），
// 未命中或装载失败时回退全局库当前注册表（与 RegistryFor 的容错语义一致）。
func (l *LiveRegistry) GetSkill(name string) (*skill.Skill, error) {
	// 名称合法性校验：name 会被直接拼进磁盘路径，必须阻断路径穿越
	if !validSkillName(name) {
		return nil, skill.ErrSkillNotFound
	}

	// Agent 级库实时读盘：同名覆盖全局级；目录不存在/解析失败按未命中处理
	if dir := l.store.AgentSkillDir(l.agentName); dir != "" {
		sk, _, err := LoadSkillFromDir(filepath.Join(dir, name), "filesystem")
		if err == nil && sk != nil {
			return sk, nil
		}
	}

	// 全局库：读取当前原子替换后的注册表指针（RLock 下取引用，检索在锁外）
	l.store.mu.RLock()
	reg := l.store.global
	l.store.mu.RUnlock()
	return reg.GetSkill(name)
}

// validSkillName 校验技能名符合 SKILL.md 规范（仅小写字母、数字、连字符，
// 最长 64 字符）。非法名称直接按未找到处理，不触碰磁盘。
func validSkillName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}
