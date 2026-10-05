package skillstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetch(t *testing.T) {
	testDeadline(t)

	t.Run("正常取用:全局技能复制到项目级库", func(t *testing.T) {
		s, globalDir, _, projectDir := newTestStore(t)
		mustWriteSkill(t, globalDir, "pdf", "全局版技能")
		// notes 等子目录应随目录整体复制
		notesDir := filepath.Join(globalDir, "pdf", "notes")
		if err := os.MkdirAll(notesDir, 0755); err != nil {
			t.Fatalf("创建 notes 目录失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(notesDir, "glm-experience-踩坑.md"), []byte("踩坑记录"), 0644); err != nil {
			t.Fatalf("写入 notes 失败: %v", err)
		}

		if err := s.Fetch(FetchOptions{SkillName: "pdf", ProjectDir: projectDir}); err != nil {
			t.Fatalf("Fetch 失败: %v", err)
		}

		dst := filepath.Join(projectDir, ".agents", "skills", "pdf", "SKILL.md")
		content, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("项目级副本 SKILL.md 不存在: %v", err)
		}
		if !strings.Contains(string(content), "全局版技能") {
			t.Fatalf("副本内容不符: %s", content)
		}
		if _, err := os.Stat(filepath.Join(projectDir, ".agents", "skills", "pdf", "notes", "glm-experience-踩坑.md")); err != nil {
			t.Fatalf("notes 子目录应随副本复制: %v", err)
		}
	})

	t.Run("目标同名默认拒绝", func(t *testing.T) {
		s, globalDir, _, projectDir := newTestStore(t)
		mustWriteSkill(t, globalDir, "pdf", "全局版技能")
		mustWriteSkill(t, filepath.Join(projectDir, ".agents", "skills"), "pdf", "项目级已有副本")

		err := s.Fetch(FetchOptions{SkillName: "pdf", ProjectDir: projectDir})
		if err == nil {
			t.Fatalf("目标已存在同名技能时默认应拒绝")
		}
		if !strings.Contains(err.Error(), "--force") {
			t.Fatalf("拒绝信息应引导使用 --force, 实际: %v", err)
		}
		// 项目级副本内容不应被改动
		content, _ := os.ReadFile(filepath.Join(projectDir, ".agents", "skills", "pdf", "SKILL.md"))
		if !strings.Contains(string(content), "项目级已有副本") {
			t.Fatalf("默认拒绝时项目级副本不应被覆盖")
		}
	})

	t.Run("Overwrite 覆盖项目级副本", func(t *testing.T) {
		s, globalDir, _, projectDir := newTestStore(t)
		mustWriteSkill(t, globalDir, "pdf", "全局版技能")
		mustWriteSkill(t, filepath.Join(projectDir, ".agents", "skills"), "pdf", "旧版副本")

		if err := s.Fetch(FetchOptions{SkillName: "pdf", ProjectDir: projectDir, Overwrite: true}); err != nil {
			t.Fatalf("Fetch(Overwrite) 失败: %v", err)
		}
		content, err := os.ReadFile(filepath.Join(projectDir, ".agents", "skills", "pdf", "SKILL.md"))
		if err != nil {
			t.Fatalf("读取覆盖后的副本失败: %v", err)
		}
		if !strings.Contains(string(content), "全局版技能") {
			t.Fatalf("覆盖后副本应为全局版内容, 实际: %s", content)
		}
	})

	t.Run("Overwrite 覆盖时旧 notes 迁入 notes.prev 留档", func(t *testing.T) {
		s, globalDir, _, projectDir := newTestStore(t)
		mustWriteSkill(t, globalDir, "pdf", "全局版技能")
		mustWriteSkill(t, filepath.Join(projectDir, ".agents", "skills"), "pdf", "旧版副本")
		// 旧副本内的进化记录(notes 资产,不可再生)
		notesDir := filepath.Join(projectDir, ".agents", "skills", "pdf", "notes")
		if err := os.MkdirAll(notesDir, 0755); err != nil {
			t.Fatalf("创建旧 notes 失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(notesDir, "glm-experience-踩坑.md"), []byte("进化记录"), 0644); err != nil {
			t.Fatalf("写入旧 notes 失败: %v", err)
		}

		if err := s.Fetch(FetchOptions{SkillName: "pdf", ProjectDir: projectDir, Overwrite: true}); err != nil {
			t.Fatalf("Fetch(Overwrite) 失败: %v", err)
		}

		// 新副本 SKILL.md 为全局版;旧 notes 未丢,留档于 notes.prev-*
		content, err := os.ReadFile(filepath.Join(projectDir, ".agents", "skills", "pdf", "SKILL.md"))
		require.NoError(t, err)
		if !strings.Contains(string(content), "全局版技能") {
			t.Fatalf("覆盖后 SKILL.md 应为全局版, 实际: %s", content)
		}
		matches, err := filepath.Glob(filepath.Join(projectDir, ".agents", "skills", "pdf", "notes.prev-*", "glm-experience-踩坑.md"))
		require.NoError(t, err)
		if len(matches) != 1 {
			t.Fatalf("旧进化记录应留档于 notes.prev-*/, 实际匹配: %v", matches)
		}
		// 覆盖后新副本的 notes/ 不存在(全局源无 notes),不残留临时目录
		entries, err := os.ReadDir(filepath.Join(projectDir, ".agents", "skills"))
		require.NoError(t, err)
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".fetch-stage-") {
				t.Fatalf("覆盖后不应残留暂存目录: %s", e.Name())
			}
		}
	})

	t.Run("Overwrite 时全局源自带 notes 的共存", func(t *testing.T) {
		s, globalDir, _, projectDir := newTestStore(t)
		mustWriteSkill(t, globalDir, "pdf", "全局版技能")
		// 全局源自带 notes(随 promote 上去的旧记录)
		srcNotes := filepath.Join(globalDir, "pdf", "notes")
		if err := os.MkdirAll(srcNotes, 0755); err != nil {
			t.Fatalf("创建源 notes 失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(srcNotes, "glm-experience-源记录.md"), []byte("源记录"), 0644); err != nil {
			t.Fatalf("写入源 notes 失败: %v", err)
		}
		mustWriteSkill(t, filepath.Join(projectDir, ".agents", "skills"), "pdf", "旧版副本")
		oldNotes := filepath.Join(projectDir, ".agents", "skills", "pdf", "notes")
		if err := os.MkdirAll(oldNotes, 0755); err != nil {
			t.Fatalf("创建旧 notes 失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(oldNotes, "glm-experience-旧记录.md"), []byte("旧记录"), 0644); err != nil {
			t.Fatalf("写入旧 notes 失败: %v", err)
		}

		if err := s.Fetch(FetchOptions{SkillName: "pdf", ProjectDir: projectDir, Overwrite: true}); err != nil {
			t.Fatalf("Fetch(Overwrite) 失败: %v", err)
		}

		// 新副本 notes/ 为全局源内容
		srcNote, err := os.ReadFile(filepath.Join(projectDir, ".agents", "skills", "pdf", "notes", "glm-experience-源记录.md"))
		require.NoError(t, err)
		if !strings.Contains(string(srcNote), "源记录") {
			t.Fatalf("新副本 notes/ 应为全局源内容")
		}
		// 旧副本 notes 留档于 notes.prev-*
		matches, err := filepath.Glob(filepath.Join(projectDir, ".agents", "skills", "pdf", "notes.prev-*", "glm-experience-旧记录.md"))
		require.NoError(t, err)
		if len(matches) != 1 {
			t.Fatalf("旧记录应留档于 notes.prev-*/, 实际匹配: %v", matches)
		}
	})

	t.Run("非法技能名拒绝", func(t *testing.T) {
		s, globalDir, _, projectDir := newTestStore(t)
		mustWriteSkill(t, globalDir, "pdf", "全局版技能")

		if err := s.Fetch(FetchOptions{SkillName: "../escape", ProjectDir: projectDir}); err == nil {
			t.Fatalf("路径穿越技能名应被拒绝")
		}
		if err := s.Fetch(FetchOptions{SkillName: "..", ProjectDir: projectDir}); err == nil {
			t.Fatalf("\"..\" 应被拒绝")
		}
		if err := s.Fetch(FetchOptions{SkillName: ".", ProjectDir: projectDir}); err == nil {
			t.Fatalf("\".\" 应被拒绝")
		}
	})

	t.Run("全局库中不存在技能时报错", func(t *testing.T) {
		s, _, _, projectDir := newTestStore(t)

		if err := s.Fetch(FetchOptions{SkillName: "no-such-skill", ProjectDir: projectDir}); err == nil {
			t.Fatalf("全局库中不存在的技能应报错")
		}
	})

	t.Run("缺少项目目录时报错", func(t *testing.T) {
		s, globalDir, _, _ := newTestStore(t)
		mustWriteSkill(t, globalDir, "pdf", "全局版技能")

		if err := s.Fetch(FetchOptions{SkillName: "pdf", ProjectDir: ""}); err == nil {
			t.Fatalf("缺少项目目录应报错")
		}
	})
}
