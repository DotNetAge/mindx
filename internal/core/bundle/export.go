package bundle

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/skillstore"
)

// ExportAgent 将 Agent 导出为分发包（PR-PROMPTS 第四节"导出保真原则"）。
//
// 打包内容（保证分发包行为与本地运行时一致，"逻辑重写"不漂移）：
//   - agent/IDENTITY.md、agent/SOUL.md：按磁盘现状打包（保留手写痕迹）；
//   - agent/skills/<name>/：Agent 级技能库目录全部内容（实际生效的覆盖版本），
//     加上声明引用（frontmatter skills）但不在 Agent 级目录中的全局技能副本。
//
// 清单 Category 取 Agent 自身属性（frontmatter category，中文业务分类），
// 本地注册表与市场货架同口径。
//
// 返回生成的包清单；声明引用的技能在本地缺失时记入 warnings（加载降级语义，
// 不阻断导出）。
func ExportAgent(agent *agentstore.Agent, skills *skillstore.Store, outPath string) (*Manifest, []string, error) {
	if agent == nil {
		return nil, nil, fmt.Errorf("agent 不能为空")
	}
	if strings.TrimSpace(outPath) == "" {
		return nil, nil, fmt.Errorf("导出路径不能为空")
	}
	if agent.Dir == "" {
		return nil, nil, fmt.Errorf("agent %q 缺少磁盘目录，无法导出", agent.Meta.Name)
	}

	var warnings []string
	manifest := &Manifest{
		Format:      PackageFormat,
		Kind:        KindAgent,
		Name:        agent.Meta.Name,
		Description: agent.Meta.Description,
		Icon:        agent.Meta.Icon,
		Role:        agent.Meta.Role,
		Category:    agent.Meta.Category,
	}

	// Agent 级技能目录（磁盘现状 = 实际生效版本）
	agentSkillDir := filepath.Join(agent.Dir, "skills")
	entries, err := os.ReadDir(agentSkillDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("读取 Agent 级技能目录 %s 失败：%w", agentSkillDir, err)
	}
	agentLevel := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() {
			agentLevel[e.Name()] = true
		}
	}

	// 声明引用但不在 Agent 级目录中的技能：从全局库取副本补入（保真）
	globalRefs := make([]string, 0)
	for _, name := range agent.Meta.Skills {
		if agentLevel[name] {
			continue
		}
		sk, err := skills.Global().GetSkill(name)
		if err != nil {
			// 引用缺失与运行时加载降级语义一致：跳过并告警，不阻断导出
			warnings = append(warnings, fmt.Sprintf("引用技能 %q 在本地库中不存在，未打入包内", name))
			continue
		}
		globalRefs = append(globalRefs, name)
		_ = sk // 仅需 RootDir，打包时统一处理
	}

	// 组装 manifest.Skills（Agent 级 + 全局引用补入，排序保证稳定）
	packed := make([]string, 0, len(agentLevel)+len(globalRefs))
	for name := range agentLevel {
		packed = append(packed, name)
	}
	packed = append(packed, globalRefs...)
	sort.Strings(packed)
	manifest.Skills = packed

	// 写 zip
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return nil, nil, fmt.Errorf("创建导出目录失败：%w", err)
	}
	out, err := os.Create(outPath)
	if err != nil {
		return nil, nil, fmt.Errorf("创建分发包 %s 失败：%w", outPath, err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer func() { _ = zw.Close() }()

	if err := zipFileEntry(zw, filepath.Join(agent.Dir, identityFileName), zipPrefixAgent+identityFileName); err != nil {
		return nil, nil, err
	}
	// SOUL.md 缺省为空是合法状态，跳过即可
	if _, err := os.Stat(filepath.Join(agent.Dir, soulFileName)); err == nil {
		if err := zipFileEntry(zw, filepath.Join(agent.Dir, soulFileName), zipPrefixAgent+soulFileName); err != nil {
			return nil, nil, err
		}
	}
	// Agent 级技能目录整体打包；顺路收集技能中文展示名（metadata.name_zh）与描述（frontmatter description）
	skillNames := make(map[string]string)
	skillDescs := make(map[string]string)
	for name := range agentLevel {
		if err := zipDir(zw, filepath.Join(agentSkillDir, name), zipPrefixAgent+zipPrefixSkills+name); err != nil {
			return nil, nil, fmt.Errorf("打包 Agent 级技能 %s 失败：%w", name, err)
		}
		dir := filepath.Join(agentSkillDir, name)
		if zh := skillstore.LoadSkillDisplayName(dir); zh != "" {
			skillNames[name] = zh
		}
		if desc := skillstore.LoadSkillDescription(dir); desc != "" {
			skillDescs[name] = desc
		}
	}
	// 全局引用技能副本打包（按全局库版本）
	for _, name := range globalRefs {
		sk, err := skills.Global().GetSkill(name)
		if err != nil || sk.RootDir == "" {
			continue
		}
		if err := zipDir(zw, sk.RootDir, zipPrefixAgent+zipPrefixSkills+name); err != nil {
			return nil, nil, fmt.Errorf("打包全局技能 %s 副本失败：%w", name, err)
		}
		if zh := skillstore.LoadSkillDisplayName(sk.RootDir); zh != "" {
			skillNames[name] = zh
		}
		if desc := skillstore.LoadSkillDescription(sk.RootDir); desc != "" {
			skillDescs[name] = desc
		}
	}
	manifest.SkillNames = skillNames
	manifest.SkillDescs = skillDescs
	// 清单最后写入（内容已定）
	if err := writeManifestEntry(zw, manifest); err != nil {
		return nil, nil, err
	}

	return manifest, warnings, nil
}

// ExportSkill 将全局库中的单个技能导出为 Skill 分发包（Agent 包的子集）。
func ExportSkill(skills *skillstore.Store, name, outPath string) (*Manifest, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("技能名不能为空")
	}
	if strings.TrimSpace(outPath) == "" {
		return nil, fmt.Errorf("导出路径不能为空")
	}
	sk, err := skills.Global().GetSkill(name)
	if err != nil {
		return nil, fmt.Errorf("全局库中不存在技能 %q", name)
	}
	if sk.RootDir == "" {
		return nil, fmt.Errorf("技能 %q 无磁盘目录，无法导出", name)
	}

	manifest := &Manifest{
		Format:      PackageFormat,
		Kind:        KindSkill,
		Name:        sk.Name,
		Description: sk.Description,
		Skills:      []string{sk.Name},
	}
	// 技能中文展示名（SKILL.md metadata.name_zh；市场卡片展示用，缺省不写入）
	if zh := skillstore.LoadSkillDisplayName(sk.RootDir); zh != "" {
		manifest.SkillNames = map[string]string{sk.Name: zh}
	}
	// 技能描述（frontmatter description；市场详情页展示用，缺省不写入）
	if desc := skillstore.LoadSkillDescription(sk.RootDir); desc != "" {
		manifest.SkillDescs = map[string]string{sk.Name: desc}
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return nil, fmt.Errorf("创建导出目录失败：%w", err)
	}
	out, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("创建分发包 %s 失败：%w", outPath, err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer func() { _ = zw.Close() }()

	if err := zipDir(zw, sk.RootDir, zipPrefixSkills+sk.Name); err != nil {
		return nil, fmt.Errorf("打包技能 %s 失败：%w", name, err)
	}
	if err := writeManifestEntry(zw, manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

// writeManifestEntry 将清单写入包内固定路径。
func writeManifestEntry(zw *zip.Writer, m *Manifest) error {
	w, err := zw.Create(zipPathManifest)
	if err != nil {
		return fmt.Errorf("创建清单条目失败：%w", err)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("序列化清单失败：%w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("写入清单失败：%w", err)
	}
	return nil
}
