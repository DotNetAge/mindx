package skillstore

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/goharness/skill"
)

// SkillLevel 标识技能库层级（PR-PROMPTS 第三节三级库）。
type SkillLevel string

const (
	// LevelGlobal 全局库：~/.mindx/skills，被验证过的"最佳实践"经验。
	LevelGlobal SkillLevel = "global"
	// LevelAgent Agent 级库：agents/<name>/skills，Agent 私有经验。
	LevelAgent SkillLevel = "agent"
	// LevelProject 项目级库：<ProjectDir>/.agents/skills，发现式的"可能"经验
	// （军规：动态技能绝不进入系统提示词，经 mindx skills discovery 发现、
	// Skill 工具按需加载回退解析）。
	LevelProject SkillLevel = "project"
)

// Valid 校验层级取值。
func (l SkillLevel) Valid() bool {
	switch l {
	case LevelGlobal, LevelAgent, LevelProject:
		return true
	}
	return false
}

// PromoteOptions 描述一次晋升操作的全部参数。
type PromoteOptions struct {
	// SkillName 技能名（即源库中的技能目录名）。
	SkillName string
	// From 来源层级（agent / project；全局库是晋升终点，不作为来源）。
	From SkillLevel
	// To 目标层级（global / agent）。
	To SkillLevel
	// AgentName From=agent 或 To=agent 时必填。
	AgentName string
	// ProjectDir From=project 时必填。
	ProjectDir string
	// Overwrite 目标存在同名技能时是否覆盖（由用户在界面确认后传入）。
	Overwrite bool
}

// ProjectSkillDir 返回项目级技能库目录（<projectDir>/.agents/skills，
// 隐藏目录）。projectDir 为空时返回空字符串。
func (s *Store) ProjectSkillDir(projectDir string) string {
	if strings.TrimSpace(projectDir) == "" {
		return ""
	}
	return filepath.Join(projectDir, ".agents", "skills")
}

// DiscoverProject 扫描项目级技能库（发现式：仅扫描装载，不写入任何内存注册表）。
// 目录不存在视为空库（不报错）；单技能装载失败跳过并汇入错误汇总。
func (s *Store) DiscoverProject(projectDir string) (*Registry, []error) {
	return s.loadDir(s.ProjectSkillDir(projectDir))
}

// ResolveProject 按名称解析项目动态技能（Skill 工具回退加载入口）。
// 仅读取装载，不写入注册表；名称为空、含路径分隔符（防目录穿越）或
// 技能不存在时返回 ErrSkillNotFound。
func (s *Store) ResolveProject(projectDir, name string) (*skill.Skill, error) {
	if name == "" || name != filepath.Base(name) {
		return nil, skill.ErrSkillNotFound
	}
	sk, _, err := LoadSkillFromDir(filepath.Join(s.ProjectSkillDir(projectDir), name), "project")
	if err != nil || sk == nil {
		return nil, skill.ErrSkillNotFound
	}
	return sk, nil
}

// Promote 将技能在库层级间晋升（目录整体复制，保留 SKILL.md 与 references/scripts 资源）。
//
// 语义（PR-PROMPTS 第三节"晋升管线"）：一切晋升由用户裁决，本方法只做机械搬运：
//   - 项目级 → Agent 级 / 全局级；
//   - Agent 级 → 全局级；
//   - 全局级是晋升终点，不作为来源；
//   - 目标已存在同名技能时默认拒绝（返回错误），由界面确认后带 Overwrite 重试。
//
// 复制完成后：目标为全局库时自动 ReloadGlobal（原子替换指针，即刻对所有
// 会话生效）；目标为 Agent 级库时无需重载（运行时经 LiveRegistry 实时读盘，
// 晋升落盘后下一轮 Skill 工具检索即可见）。
func (s *Store) Promote(o PromoteOptions) error {
	if strings.TrimSpace(o.SkillName) == "" {
		return fmt.Errorf("技能名不能为空")
	}
	if !o.From.Valid() || !o.To.Valid() {
		return fmt.Errorf("未知的技能库层级：from=%q to=%q", o.From, o.To)
	}
	if o.From == LevelGlobal {
		return fmt.Errorf("全局库是晋升终点，不能作为晋升来源")
	}
	if o.From == o.To {
		return fmt.Errorf("晋升来源与目标层级相同（%s）", o.From)
	}

	srcDir, err := s.levelDir(o.From, o.AgentName, o.ProjectDir)
	if err != nil {
		return err
	}
	dstDir, err := s.levelDir(o.To, o.AgentName, o.ProjectDir)
	if err != nil {
		return err
	}

	from := filepath.Join(srcDir, o.SkillName)
	to := filepath.Join(dstDir, o.SkillName)

	// 源校验：技能目录必须存在且含 SKILL.md
	if info, err := os.Stat(filepath.Join(from, "SKILL.md")); err != nil || info.IsDir() {
		return fmt.Errorf("来源库中不存在技能 %q（%s）", o.SkillName, from)
	}

	// 目标校验：同名技能默认拒绝，覆盖需显式确认
	if _, err := os.Stat(filepath.Join(to, "SKILL.md")); err == nil {
		if !o.Overwrite {
			return fmt.Errorf("目标库已存在同名技能 %q，如需覆盖请在界面中确认后重试", o.SkillName)
		}
		if err := os.RemoveAll(to); err != nil {
			return fmt.Errorf("清理目标同名技能失败：%w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查目标技能目录失败：%w", err)
	}

	if err := copyDir(from, to); err != nil {
		return fmt.Errorf("复制技能目录失败：%w", err)
	}

	// 全局库有内存注册表，复制后立即重载使晋升即刻生效；
	// Agent 级 / 项目级库按需装载，无需重载。
	if o.To == LevelGlobal {
		if err := s.ReloadGlobal(); err != nil {
			return fmt.Errorf("技能已复制但全局库重载失败：%w", err)
		}
	}
	return nil
}

// levelDir 解析层级对应的库根目录。
func (s *Store) levelDir(level SkillLevel, agentName, projectDir string) (string, error) {
	switch level {
	case LevelGlobal:
		return s.globalDir, nil
	case LevelAgent:
		dir := s.AgentSkillDir(agentName)
		if dir == "" {
			return "", fmt.Errorf("Agent 级库需要指定 Agent 名称")
		}
		return dir, nil
	case LevelProject:
		dir := s.ProjectSkillDir(projectDir)
		if dir == "" {
			return "", fmt.Errorf("项目级库需要指定项目目录")
		}
		return dir, nil
	default:
		return "", fmt.Errorf("未知的技能库层级：%q", level)
	}
}

// copyDir 递归复制目录树（保留相对结构与普通文件内容）。
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !d.Type().IsRegular() {
			// 跳过符号链接等特殊条目，只搬运常规文件
			return nil
		}
		return copyFile(path, target)
	})
}

// copyFile 复制单个文件（0644 权限；技能资源均为文本，无需保留位）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
