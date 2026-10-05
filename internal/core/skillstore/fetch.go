package skillstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FetchOptions 描述一次取用操作的全部参数。
type FetchOptions struct {
	// SkillName 技能名（即全局库中的技能目录名，必须是纯目录名）。
	SkillName string
	// ProjectDir 目标项目目录（必填）。
	ProjectDir string
	// Overwrite 项目级已存在同名技能时是否覆盖（由调用方显式确认后传入）。
	Overwrite bool
}

// Fetch 将全局库技能取用到项目级库（全局 → 项目级，目录整体复制）。
//
// 与 Promote 对偶：Promote 把验证过的经验向上晋升（项目级/Agent 级 → 全局），
// Fetch 把全局最佳实践下发到项目级作为技能进化的起点。不并入 Promote 的
// 原因：晋升管线军规"全局库是晋升终点，不能作为来源"是单向向上的语义
// 约束（promote.go From 校验），下发是另一条受控通道，分开表达避免破坏
// 晋升校验。
//
// 语义：
//   - 仅支持 全局 → 项目级；
//   - 项目级已存在同名技能时默认拒绝，覆盖需调用方显式传 Overwrite=true；
//   - Overwrite 覆盖时保护项目副本的 notes/（进化复盘记录，不可再生）：
//     旧记录整体迁入新副本的 notes.prev-<unix>/ 留档，新副本 notes/ 以全局源为准；
//   - 复制走点前缀暂存目录 + 原子 Rename：copyDir 中途失败不会在项目库
//     残留残缺技能（残缺副本会被 DiscoverProject 正常装载，使用时才暴露）；
//   - 纯复制（保留 notes/ 等子目录），全局库不做任何改动。
//
// 复制完成后无需重载：项目级库按需装载（ResolveProject 每次实时读盘），
// 下一次 Skill 工具解析即可见。
func (s *Store) Fetch(o FetchOptions) error {
	if strings.TrimSpace(o.SkillName) == "" {
		return fmt.Errorf("技能名不能为空")
	}
	// 防目录穿越：技能名必须是纯目录名（对齐 ResolveProject）；
	// 显式拒绝 "." 与 ".."——filepath.Base 对二者原样返回，放行会绕过
	// 单技能复制边界（把整个全局库平铺进项目库）
	if o.SkillName != filepath.Base(o.SkillName) || o.SkillName == "." || o.SkillName == ".." {
		return fmt.Errorf("非法的技能名：%q", o.SkillName)
	}
	if strings.TrimSpace(o.ProjectDir) == "" {
		return fmt.Errorf("项目级库需要指定项目目录")
	}

	srcDir, err := s.levelDir(LevelGlobal, "", "")
	if err != nil {
		return err
	}
	dstDir := s.ProjectSkillDir(o.ProjectDir)

	from := filepath.Join(srcDir, o.SkillName)
	to := filepath.Join(dstDir, o.SkillName)

	// 源校验：全局库中技能目录必须存在且含 SKILL.md
	info, statErr := os.Stat(filepath.Join(from, "SKILL.md"))
	if statErr != nil || info.IsDir() {
		return fmt.Errorf("全局库中不存在技能 %q（%s）", o.SkillName, from)
	}

	// 目标校验：同名技能默认拒绝，覆盖需显式确认
	_, statErr = os.Stat(filepath.Join(to, "SKILL.md"))
	overwrite := statErr == nil
	if statErr == nil && !o.Overwrite {
		return fmt.Errorf("项目级已存在技能 %q，如需覆盖请使用 --force 重试", o.SkillName)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("检查项目级技能目录失败：%w", statErr)
	}

	// 统一暂存复制 + 原子切换：暂存目录用点前缀放库根下（loadDir 对无
	// SKILL.md 的目录静默跳过，瞬态不污染发现结果）
	if mkErr := os.MkdirAll(dstDir, 0755); mkErr != nil {
		return fmt.Errorf("创建项目级技能库目录失败：%w", mkErr)
	}
	tmpRoot, mkErr := os.MkdirTemp(dstDir, ".fetch-stage-")
	if mkErr != nil {
		return fmt.Errorf("创建暂存目录失败：%w", mkErr)
	}
	staged := filepath.Join(tmpRoot, o.SkillName)

	// Overwrite：保护旧副本的 notes/（进化复盘资产，不可再生），
	// 迁入暂存目录，复制完成后迁回新副本 notes.prev-<unix>/ 留档
	var legacyNotes string
	if overwrite {
		if _, nerr := os.Stat(filepath.Join(to, "notes")); nerr == nil {
			legacyNotes = filepath.Join(tmpRoot, "legacy-notes")
			if rErr := os.Rename(filepath.Join(to, "notes"), legacyNotes); rErr != nil {
				os.RemoveAll(tmpRoot)
				return fmt.Errorf("保护进化记录失败：%w", rErr)
			}
		}
		if rmErr := os.RemoveAll(to); rmErr != nil {
			return fmt.Errorf("清理项目级同名技能失败：%w（旧进化记录暂存于 %s，未删除）", rmErr, legacyNotes)
		}
	}

	if cErr := copyDir(from, staged); cErr != nil {
		os.RemoveAll(tmpRoot)
		if legacyNotes != "" {
			return fmt.Errorf("复制技能目录失败：%w（旧进化记录暂存于 %s，未删除）", cErr, legacyNotes)
		}
		return fmt.Errorf("复制技能目录失败：%w", cErr)
	}

	// 原子切换：Rename 同盘瞬时完成，项目库不会出现残缺技能
	if rErr := os.Rename(staged, to); rErr != nil {
		os.RemoveAll(tmpRoot)
		if legacyNotes != "" {
			return fmt.Errorf("技能切换失败：%w（旧进化记录暂存于 %s，未删除）", rErr, legacyNotes)
		}
		return fmt.Errorf("技能切换失败：%w", rErr)
	}

	if legacyNotes != "" {
		// 旧进化记录迁回新副本留档；时间戳用纳秒，同秒内连续覆盖不冲突
		dst := filepath.Join(to, fmt.Sprintf("notes.prev-%d", time.Now().UnixNano()))
		if mErr := os.Rename(legacyNotes, dst); mErr != nil {
			return fmt.Errorf("恢复进化记录失败（暂存于 %s）：%w", legacyNotes, mErr)
		}
	}
	// 清理空的暂存容器
	if rmErr := os.RemoveAll(tmpRoot); rmErr != nil {
		return fmt.Errorf("清理暂存目录失败：%w", rmErr)
	}
	return nil
}
