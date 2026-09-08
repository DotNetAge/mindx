package core

import (
	"fmt"
	"strings"

	"github.com/DotNetAge/mindx/internal/core/agentstore"
)

// SetAgentHired 修改 Agent 的雇佣标记并持久化到 IDENTITY.md frontmatter，
// 同时更新 AgentStore 内存缓存（无需 reload 即刻生效）。
//
// hired 现为 frontmatter 一级字段，走 agentstore 的全量强类型序列化写入，
// 不再有旧版"SaveTo 重建 frontmatter 丢失手写字段"的问题。
// hired 缺省语义为 false（未雇佣），文件中显式写入 true/false 以固化状态。
func SetAgentHired(store *agentstore.AgentStore, name string, hired bool) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("agent 名称不能为空")
	}
	if store == nil {
		return fmt.Errorf("agent 注册表不可用")
	}

	agent := store.Get(name)
	if agent == nil {
		return fmt.Errorf("未找到智能体 %q，请确认名称是否正确", name)
	}

	updated := *agent
	updated.Meta = agent.Meta
	updated.Meta.Hired = hired
	if err := store.Save(&updated); err != nil {
		return fmt.Errorf("修改 Agent %q 的雇佣标记失败: %w", name, err)
	}
	return nil
}
