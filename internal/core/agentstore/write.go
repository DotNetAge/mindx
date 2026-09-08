package agentstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Save 将 Agent 持久化为目录格式（IDENTITY.md + SOUL.md）并更新内存注册表。
//
// 原子性：单文件采用"写临时文件 → rename"原子替换；目录已存在时原地更新，
// 不重建目录（避免 rename 目录的窗口期）。全量强类型序列化保证不丢字段。
func (s *AgentStore) Save(agent *Agent) error {
	if agent == nil {
		return fmt.Errorf("agent 不能为空")
	}
	if strings.TrimSpace(agent.Meta.Name) == "" {
		return fmt.Errorf("agent 名称不能为空")
	}

	dir := s.agentDirPath(agent.Meta.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建 Agent 目录 %s 失败: %w", dir, err)
	}

	identity, err := renderIdentity(agent.Meta)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(dir, identityFileName), []byte(identity)); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", dir, err)
	}
	if err := writeFileAtomic(filepath.Join(dir, soulFileName), []byte(agent.Soul)); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", dir, err)
	}

	// 更新内存缓存（深拷贝语义：直接存调用方对象即可，调用方此后不应再修改）
	stored := *agent
	stored.Dir = dir
	s.mu.Lock()
	s.agents[stored.Meta.Name] = &stored
	s.mu.Unlock()
	return nil
}

// SetHired 更新指定 Agent 的雇佣标记并持久化。
// 强类型全量重写：icon / domains / exclude_tools 等字段随 Meta 一起保留，
// 不再需要旧版"文本级精准插入"的规避手段。
func (s *AgentStore) SetHired(name string, hired bool) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("agent 名称不能为空")
	}
	agent := s.Get(name)
	if agent == nil {
		return fmt.Errorf("未找到智能体 %q，请确认名称是否正确", name)
	}
	agent.Meta.Hired = hired
	return s.Save(agent)
}

// Remove 删除指定 Agent：先删磁盘目录，成功后移除内存注册。
func (s *AgentStore) Remove(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("agent 名称不能为空")
	}
	s.mu.Lock()
	_, exists := s.agents[name]
	s.mu.Unlock()
	if !exists {
		return fmt.Errorf("未找到智能体 %s", name)
	}

	dir := s.agentDirPath(name)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("无法删除目录 %s: %w", dir, err)
	}
	// 兼容清理：旧单文件迁移后的备份也一并移除
	_ = os.Remove(filepath.Join(s.dir, strings.ToLower(name)+".md"+legacyBackupExt))

	s.mu.Lock()
	delete(s.agents, name)
	s.mu.Unlock()
	return nil
}

// writeFileAtomic 原子写文件：先写同目录临时文件，再 rename 覆盖目标。
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("原子替换文件失败: %w", err)
	}
	return nil
}
