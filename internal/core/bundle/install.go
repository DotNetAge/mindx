package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InstallOptions 是安装参数。
type InstallOptions struct {
	// AgentsDir agents 根目录（kind=agent 时必填）。
	AgentsDir string
	// GlobalDir 全局技能库目录（两种类型均必填）。
	GlobalDir string
	// Overwrite 目标已存在同名 Agent / 技能时是否覆盖（由用户在界面确认后传入）。
	Overwrite bool
}

// Install 将分发包展开落库（分发格式 ≠ 存储格式，冗余只存在于传输层）：
//   - Agent 包：agent/ → agents/<name>/；包内技能与全局库同名 → 落 Agent 目录
//     skills/（Agent 级覆盖 = 逻辑重写），不同名 → 进全局库；
//   - Skill 包：skills/<name>/ → 全局库；
//   - 同名冲突默认拒绝（返回错误），由界面确认后带 Overwrite 重试。
//
// 全量条目先读入内存并完成冲突预检后才写盘，避免"写一半失败"的中间态。
// 调用方负责安装后的注册表重载（ReloadAgents / ReloadSkills）与运行时失效。
func Install(pkgPath string, opts InstallOptions) (*InstallResult, error) {
	if strings.TrimSpace(pkgPath) == "" {
		return nil, fmt.Errorf("分发包路径不能为空")
	}
	if strings.TrimSpace(opts.GlobalDir) == "" {
		return nil, fmt.Errorf("全局技能库目录不能为空")
	}

	entries, err := readPackageEntries(pkgPath)
	if err != nil {
		return nil, err
	}
	manifest, err := manifestFromEntries(entries)
	if err != nil {
		return nil, err
	}

	switch manifest.Kind {
	case KindAgent:
		return installAgent(entries, manifest, opts)
	case KindSkill:
		return installSkill(entries, manifest, opts)
	default:
		return nil, fmt.Errorf("未知的分发包类型：%q", manifest.Kind)
	}
}

// installAgent 展开 Agent 分发包。
func installAgent(entries []zipEntry, manifest *Manifest, opts InstallOptions) (*InstallResult, error) {
	if strings.TrimSpace(opts.AgentsDir) == "" {
		return nil, fmt.Errorf("安装 Agent 分发包需要指定 agents 目录")
	}

	agentDir := filepath.Join(opts.AgentsDir, strings.ToLower(manifest.Name))
	overwritten := false

	// 冲突预检：目标 Agent 已存在时默认拒绝（包内技能无冲突——同名落 Agent 级、
	// 不同名进全局库，规则本身消除了技能层的同名写入冲突）
	if _, err := os.Stat(agentDir); err == nil {
		if !opts.Overwrite {
			return nil, fmt.Errorf("已存在同名智能体 %q，如需覆盖请在界面中确认后重试", manifest.Name)
		}
		overwritten = true
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("检查智能体目录失败：%w", err)
	}

	// 全局库名集合（决定包内技能落全局还是 Agent 级）
	globalNames, err := dirNames(opts.GlobalDir)
	if err != nil {
		return nil, err
	}

	result := &InstallResult{Kind: KindAgent, Name: manifest.Name, Overwritten: overwritten}
	skillGlobal := make(map[string]bool)
	skillAgent := make(map[string]bool)

	for _, e := range entries {
		switch {
		case e.Name == zipPathManifest:
			continue // 清单不落盘
		case e.Name == zipPrefixAgent+identityFileName || e.Name == zipPrefixAgent+soulFileName:
			// Agent 本体文件：IDENTITY.md / SOUL.md
			rel := strings.TrimPrefix(e.Name, zipPrefixAgent)
			if err := writeEntryFile(filepath.Join(agentDir, filepath.FromSlash(rel)), e.Content); err != nil {
				return nil, err
			}
		case strings.HasPrefix(e.Name, zipPrefixAgent+zipPrefixSkills):
			// 包内技能：agent/skills/<skill-name>/<rel>
			rest := strings.TrimPrefix(e.Name, zipPrefixAgent+zipPrefixSkills)
			skillName, rel, ok := strings.Cut(rest, "/")
			if !ok || skillName == "" || rel == "" {
				continue // 技能目录级条目，跳过
			}
			var dest string
			if globalNames[skillName] {
				// 全局库已有同名：落 Agent 级目录（Agent 级覆盖 = 逻辑重写）
				dest = filepath.Join(agentDir, "skills", skillName, filepath.FromSlash(rel))
				skillAgent[skillName] = true
			} else {
				// 全局库没有：进全局库，Agent 引用列表天然指向本地
				dest = filepath.Join(opts.GlobalDir, skillName, filepath.FromSlash(rel))
				skillGlobal[skillName] = true
			}
			if err := writeEntryFile(dest, e.Content); err != nil {
				return nil, err
			}
		default:
			// 未知条目（前向兼容）忽略
		}
	}

	result.SkillsGlobal = sortedNames(skillGlobal)
	result.SkillsAgent = sortedNames(skillAgent)
	return result, nil
}

// installSkill 展开 Skill 分发包（直接进全局库）。
func installSkill(entries []zipEntry, manifest *Manifest, opts InstallOptions) (*InstallResult, error) {
	overwritten := false

	// 冲突预检：包内任一技能在全局库已存在时默认拒绝
	packedSkills := make(map[string]bool)
	for _, e := range entries {
		if strings.HasPrefix(e.Name, zipPrefixSkills) {
			if name, _, ok := strings.Cut(strings.TrimPrefix(e.Name, zipPrefixSkills), "/"); ok && name != "" {
				packedSkills[name] = true
			}
		}
	}
	for _, name := range sortedNames(packedSkills) {
		if _, err := os.Stat(filepath.Join(opts.GlobalDir, name)); err == nil {
			if !opts.Overwrite {
				return nil, fmt.Errorf("全局库已存在同名技能 %q，如需覆盖请在界面中确认后重试", name)
			}
			overwritten = true
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("检查技能目录失败：%w", err)
		}
	}

	for _, e := range entries {
		if e.Name == zipPathManifest || !strings.HasPrefix(e.Name, zipPrefixSkills) {
			continue
		}
		rel := strings.TrimPrefix(e.Name, zipPrefixSkills)
		if rel == "" {
			continue
		}
		if err := writeEntryFile(filepath.Join(opts.GlobalDir, filepath.FromSlash(rel)), e.Content); err != nil {
			return nil, err
		}
	}

	return &InstallResult{
		Kind:         KindSkill,
		Name:         manifest.Name,
		SkillsGlobal: sortedNames(packedSkills),
		Overwritten:  overwritten,
	}, nil
}

// manifestFromEntries 从已读取的包条目中解析并校验清单。
func manifestFromEntries(entries []zipEntry) (*Manifest, error) {
	for _, e := range entries {
		if e.Name != zipPathManifest {
			continue
		}
		var m Manifest
		if err := json.Unmarshal(e.Content, &m); err != nil {
			return nil, fmt.Errorf("解析 manifest.json 失败：%w", err)
		}
		if err := m.validate(); err != nil {
			return nil, err
		}
		return &m, nil
	}
	return nil, fmt.Errorf("分发包缺少 manifest.json")
}

// dirNames 列出目录下的一级子目录名集合；目录不存在视为空集。
func dirNames(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("读取目录 %s 失败：%w", dir, err)
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names[e.Name()] = true
		}
	}
	return names, nil
}

// writeEntryFile 写入单个安装文件（父目录按需创建）。
func writeEntryFile(destPath string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("创建目录 %s 失败：%w", filepath.Dir(destPath), err)
	}
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("写入 %s 失败：%w", destPath, err)
	}
	return nil
}

// sortedNames 返回集合的排序列表（保证输出稳定）。
func sortedNames(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
