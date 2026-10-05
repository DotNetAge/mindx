package teamstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Save 将团队持久化为目录格式（TEAM.md）并更新内存注册表。
//
// 原子性：采用"写临时文件 → rename"原子替换；目录已存在时原地更新，
// 不重建目录。全量强类型序列化保证不丢字段。
func (s *Store) Save(team *Team) error {
	if team == nil {
		return fmt.Errorf("team 不能为空")
	}
	if strings.TrimSpace(team.Meta.Name) == "" {
		return fmt.Errorf("团队名称不能为空")
	}

	dir := s.teamDirPath(team.Meta.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建团队目录 %s 失败: %w", dir, err)
	}

	rendered, err := renderTeam(team.Meta, team.Profile)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(dir, teamFileName), []byte(rendered)); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", dir, err)
	}

	// 更新内存缓存（深拷贝语义：直接存调用方对象即可，调用方此后不应再修改）
	stored := *team
	stored.Dir = dir
	s.mu.Lock()
	s.teams[stored.Meta.Name] = &stored
	s.mu.Unlock()
	return nil
}

// SetHired 更新指定团队的雇佣标记并持久化。
func (s *Store) SetHired(name string, hired bool) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("团队名称不能为空")
	}
	team := s.Get(name)
	if team == nil {
		return fmt.Errorf("未找到团队 %q，请确认名称是否正确", name)
	}
	team.Meta.Hired = hired
	return s.Save(team)
}

// Remove 删除指定团队：先删磁盘目录，成功后移除内存注册。
// 仅删团队本身，不触碰被引用的 Agent（会话级悬空绑定由注入侧容错降级）。
func (s *Store) Remove(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("团队名称不能为空")
	}
	s.mu.Lock()
	_, exists := s.teams[name]
	s.mu.Unlock()
	if !exists {
		return fmt.Errorf("未找到团队 %s", name)
	}

	dir := s.teamDirPath(name)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("无法删除目录 %s: %w", dir, err)
	}

	s.mu.Lock()
	delete(s.teams, name)
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
