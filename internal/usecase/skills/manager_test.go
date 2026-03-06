package skills

import (
	"mindx/internal/entity"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillManager_LoadSkills(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// 创建测试技能
	testSkillDir := filepath.Join(skillsDir, "test_skill")
	require.NoError(t, os.MkdirAll(testSkillDir, 0755))

	skillMD := `---
name: test_skill
description: 测试技能
version: 1.0.0
author: test
tags: [test]
---

# Goal
测试目标

# Triggers
- 测试触发

# SOP
1. 步骤1
2. 步骤2
`

	skillPath := filepath.Join(testSkillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(skillPath, []byte(skillMD), 0644))

	// 验证文件已创建
	_, err := os.Stat(skillPath)
	require.NoError(t, err, "SKILL.md should exist")

	// 验证目录结构
	entries, err := os.ReadDir(skillsDir)
	require.NoError(t, err)
	t.Logf("Entries in skills dir: %d", len(entries))
	for _, entry := range entries {
		t.Logf("  - %s (isDir: %v)", entry.Name(), entry.IsDir())
	}

	// 测试解析器
	parser := NewSkillParser()
	skill, err := parser.Parse(skillPath)
	require.NoError(t, err, "parser should parse successfully")
	t.Logf("Parsed skill: %s", skill.Name)

	// 创建 SkillManager
	manager := NewSkillManager(skillsDir, nil)

	// 加载技能
	err = manager.LoadSkills()
	require.NoError(t, err)

	// 打印调试信息
	t.Logf("Skills directory: %s", skillsDir)
	t.Logf("Loaded skills count: %d", len(manager.skills))
	for name := range manager.skills {
		t.Logf("Loaded skill: %s", name)
	}

	// 验证技能加载
	skill, ok := manager.GetSkill("test_skill")
	if !ok {
		t.Fatal("skill not found")
	}
	assert.Equal(t, "test_skill", skill.Name)
	assert.Equal(t, "测试技能", skill.Description)
}

func TestSkillManager_GetSkillInfos(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// 创建多个测试技能
	for i := 1; i <= 3; i++ {
		skillName := filepath.Join(skillsDir, "skill"+string(rune('0'+i)))
		require.NoError(t, os.MkdirAll(skillName, 0755))

		skillMD := `---
name: skill` + string(rune('0'+i)) + `
description: 测试技能
version: 1.0.0
---

# Goal
测试
`

		require.NoError(t, os.WriteFile(
			filepath.Join(skillName, "SKILL.md"),
			[]byte(skillMD),
			0644,
		))
	}

	// 创建 SkillManager
	manager := NewSkillManager(skillsDir, nil)
	require.NoError(t, manager.LoadSkills())

	// 获取所有技能信息
	infos := manager.GetSkillInfos()
	assert.Len(t, infos, 3)
}

func TestSkillManager_GetSkillInfo(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// 创建测试技能
	testSkillDir := filepath.Join(skillsDir, "test_skill")
	require.NoError(t, os.MkdirAll(testSkillDir, 0755))

	skillMD := `---
name: test_skill
description: 测试技能
version: 1.0.0
---

# Goal
测试
`

	require.NoError(t, os.WriteFile(
		filepath.Join(testSkillDir, "SKILL.md"),
		[]byte(skillMD),
		0644,
	))

	// 创建 SkillManager
	manager := NewSkillManager(skillsDir, nil)
	require.NoError(t, manager.LoadSkills())

	// 获取技能信息
	info, ok := manager.GetSkillInfo("test_skill")
	assert.True(t, ok)
	assert.Equal(t, "test_skill", info.Def.Name)
	assert.Equal(t, "测试技能", info.Def.Description)

	// 获取不存在的技能
	_, ok = manager.GetSkillInfo("nonexistent")
	assert.False(t, ok)
}

func TestSkillManager_EnableDisable(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// 创建测试技能
	testSkillDir := filepath.Join(skillsDir, "test_skill")
	require.NoError(t, os.MkdirAll(testSkillDir, 0755))

	skillMD := `---
name: test_skill
description: 测试技能
version: 1.0.0
---

# Goal
测试
`

	require.NoError(t, os.WriteFile(
		filepath.Join(testSkillDir, "SKILL.md"),
		[]byte(skillMD),
		0644,
	))

	// 创建 SkillManager
	manager := NewSkillManager(skillsDir, nil)
	require.NoError(t, manager.LoadSkills())

	// 默认启用
	info, _ := manager.GetSkillInfo("test_skill")
	assert.True(t, info.Def.Enabled)

	// 禁用技能
	err := manager.Disable("test_skill")
	require.NoError(t, err)

	info, _ = manager.GetSkillInfo("test_skill")
	assert.False(t, info.Def.Enabled)

	// 启用技能
	err = manager.Enable("test_skill")
	require.NoError(t, err)

	info, _ = manager.GetSkillInfo("test_skill")
	assert.True(t, info.Def.Enabled)
}

func TestSkillManager_ReIndex(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// 创建测试技能
	testSkillDir := filepath.Join(skillsDir, "test_skill")
	require.NoError(t, os.MkdirAll(testSkillDir, 0755))

	skillMD := `---
name: test_skill
description: 测试技能
version: 1.0.0
---

# Goal
测试
`

	require.NoError(t, os.WriteFile(
		filepath.Join(testSkillDir, "SKILL.md"),
		[]byte(skillMD),
		0644,
	))

	// 创建 SkillManager
	manager := NewSkillManager(skillsDir, nil)
	require.NoError(t, manager.LoadSkills())

	// 重建索引
	assert.False(t, manager.IsReIndexing())
	assert.Nil(t, manager.GetReIndexError())

	err := manager.ReIndex()
	require.NoError(t, err)

	assert.False(t, manager.IsReIndexing())
	assert.Nil(t, manager.GetReIndexError())
}

func TestSkillManager_BatchConvert(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// 创建 SkillManager
	manager := NewSkillManager(skillsDir, nil)

	// 批量转换（Phase 4 中是 no-op）
	success, failed := manager.BatchConvert([]string{"skill1", "skill2"})
	assert.Len(t, success, 2)
	assert.Len(t, failed, 0)
}

func TestSkillManager_BatchInstall(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// 创建 SkillManager
	manager := NewSkillManager(skillsDir, nil)

	// 批量安装（Phase 4 中是 no-op）
	success, failed := manager.BatchInstall([]string{"skill1", "skill2"})
	assert.Len(t, success, 2)
	assert.Len(t, failed, 0)
}

func TestSkillManager_GetSkills(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// 创建测试技能
	testSkillDir := filepath.Join(skillsDir, "test_skill")
	require.NoError(t, os.MkdirAll(testSkillDir, 0755))

	skillMD := `---
name: test_skill
description: 测试技能
version: 1.0.0
---

# Goal
测试
`

	require.NoError(t, os.WriteFile(
		filepath.Join(testSkillDir, "SKILL.md"),
		[]byte(skillMD),
		0644,
	))

	// 创建 SkillManager
	manager := NewSkillManager(skillsDir, nil)
	require.NoError(t, manager.LoadSkills())

	// 获取所有技能
	skills, err := manager.GetSkills()
	require.NoError(t, err)
	assert.Len(t, skills, 1)
	assert.Equal(t, "test_skill", skills[0].Name)
}

func TestSkillManager_Execute(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// 创建 SkillManager
	manager := NewSkillManager(skillsDir, nil)

	// 执行技能（Phase 4 中是 no-op）
	skill := &entity.Skill{Name: "test_skill"}
	err := manager.Execute(skill, map[string]any{})
	require.NoError(t, err)
}
