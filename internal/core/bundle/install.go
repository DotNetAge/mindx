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
	// AgentsDir agents 根目录（kind=agent / kind=team 时必填）。
	AgentsDir string
	// TeamsDir teams 根目录（kind=team 时必填）。
	TeamsDir string
	// GlobalDir 全局技能库目录（三种类型均必填）。
	GlobalDir string
	// Overwrite 目标已存在同名 Agent / 团队 / 技能时是否覆盖（由用户在界面确认后传入）。
	Overwrite bool
}

// Install 将分发包展开落库（分发格式 ≠ 存储格式，冗余只存在于传输层）：
//   - Agent 包：agent/ → agents/<name>/；包内技能与全局库同名 → 落 Agent 目录
//     skills/（Agent 级覆盖 = 逻辑重写），不同名 → 进全局库；
//   - Skill 包：skills/<name>/ → 全局库；
//   - Team 包：team/ → teams/<name>/；包内 agents/<n>/ 逐成员按 Agent 包规则落位
//     （整队招聘：团队与成员一次装齐，落地后恢复纯引用）；
//   - 同名冲突默认拒绝（返回错误），由界面确认后带 Overwrite 重试。
//
// 全量条目先读入内存并完成冲突预检后才写盘，避免"写一半失败"的中间态。
// 调用方负责安装后的注册表重载（ReloadAgents / ReloadTeams / ReloadSkills）与运行时失效。
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
	case KindTeam:
		return installTeam(entries, manifest, opts)
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

	if err := installAgentDir(entries, zipPrefixAgent, agentDir, opts.GlobalDir, globalNames, skillGlobal, skillAgent); err != nil {
		return nil, err
	}

	result.SkillsGlobal = sortedNames(skillGlobal)
	result.SkillsAgent = sortedNames(skillAgent)
	return result, nil
}

// installTeam 展开 Team 分发包（整队招聘）：team/TEAM.md → teams/<name>/，
// 包内 agents/<n>/ 逐成员按 Agent 包规则落位（本体直落、技能按全局库同名分流）。
// 冲突预检：团队目录或任一成员 Agent 目录已存在 → 默认拒绝（一次预检全部条目）。
func installTeam(entries []zipEntry, manifest *Manifest, opts InstallOptions) (*InstallResult, error) {
	if strings.TrimSpace(opts.TeamsDir) == "" {
		return nil, fmt.Errorf("安装 Team 分发包需要指定 teams 目录")
	}
	if strings.TrimSpace(opts.AgentsDir) == "" {
		return nil, fmt.Errorf("安装 Team 分发包需要指定 agents 目录")
	}

	// 包内成员 Agent 名集合（agents/<n>/ 前缀一级目录）
	packedAgents := make(map[string]bool)
	for _, e := range entries {
		if strings.HasPrefix(e.Name, zipPrefixAgents) {
			if name, _, ok := strings.Cut(strings.TrimPrefix(e.Name, zipPrefixAgents), "/"); ok && name != "" {
				packedAgents[name] = true
			}
		}
	}
	if len(packedAgents) == 0 {
		return nil, fmt.Errorf("团队包缺少成员 Agent 目录（agents/<name>/）")
	}
	if manifest.Leader != "" && !packedAgents[manifest.Leader] {
		return nil, fmt.Errorf("团队包清单的负责人 %q 不在包内成员中，包不完整", manifest.Leader)
	}

	// 冲突预检：团队目录与全部成员 Agent 目录一次检完（避免写一半才发现冲突）
	teamDir := filepath.Join(opts.TeamsDir, strings.ToLower(manifest.Name))
	overwritten := false
	conflict := func(existing, label string) error {
		return fmt.Errorf("已存在同名%s %q，如需覆盖请在界面中确认后重试", label, existing)
	}
	if _, err := os.Stat(teamDir); err == nil {
		if !opts.Overwrite {
			return nil, conflict("团队", manifest.Name)
		}
		overwritten = true
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("检查团队目录失败：%w", err)
	}
	for _, name := range sortedNames(packedAgents) {
		if _, err := os.Stat(filepath.Join(opts.AgentsDir, strings.ToLower(name))); err == nil {
			if !opts.Overwrite {
				return nil, conflict("智能体", name)
			}
			overwritten = true
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("检查智能体目录失败：%w", err)
		}
	}

	// 全局库名集合（决定包内技能落全局还是 Agent 级）
	globalNames, err := dirNames(opts.GlobalDir)
	if err != nil {
		return nil, err
	}

	// 团队本体：team/TEAM.md → teams/<name>/TEAM.md
	teamDocWritten := false
	for _, e := range entries {
		if e.Name == zipPathTeamDoc {
			if err := writeEntryFile(filepath.Join(teamDir, teamDocFileName), e.Content); err != nil {
				return nil, err
			}
			teamDocWritten = true
			break
		}
	}
	if !teamDocWritten {
		return nil, fmt.Errorf("团队包缺少 %s", zipPathTeamDoc)
	}

	// 成员 Agent 逐个落位（复用 Agent 包展开逻辑；技能跨成员共享计数）
	result := &InstallResult{Kind: KindTeam, Name: manifest.Name, Overwritten: overwritten}
	skillGlobal := make(map[string]bool)
	skillAgent := make(map[string]bool)
	for _, name := range sortedNames(packedAgents) {
		prefix := zipPrefixAgents + name + "/"
		agentDir := filepath.Join(opts.AgentsDir, strings.ToLower(name))
		if err := installAgentDir(entries, prefix, agentDir, opts.GlobalDir, globalNames, skillGlobal, skillAgent); err != nil {
			return nil, err
		}
	}
	result.Members = sortedNames(packedAgents)
	result.SkillsGlobal = sortedNames(skillGlobal)
	result.SkillsAgent = sortedNames(skillAgent)
	return result, nil
}

// installAgentDir 将 zip 前缀 prefix 之下的单个 Agent 目录内容落位到 agentDir：
// 本体文件（IDENTITY.md / SOUL.md 等）直落，agent 级技能按全局库同名分流
// （同名 → agentDir/skills/<n>/ 覆盖版本；不同名 → globalDir 进全局库）。
// 写入计数累计到 skillGlobal / skillAgent（调用方按需初始化，跨成员共享时天然去重）。
// 该函数不预检冲突（调用方负责），installAgent 与 installTeam 共用。
func installAgentDir(entries []zipEntry, prefix, agentDir, globalDir string, globalNames map[string]bool, skillGlobal, skillAgent map[string]bool) error {
	skillLevel := prefix + zipPrefixSkills
	for _, e := range entries {
		switch {
		case e.Name == prefix+identityFileName || e.Name == prefix+soulFileName:
			// Agent 本体文件：IDENTITY.md / SOUL.md
			rel := strings.TrimPrefix(e.Name, prefix)
			if err := writeEntryFile(filepath.Join(agentDir, filepath.FromSlash(rel)), e.Content); err != nil {
				return err
			}
		case strings.HasPrefix(e.Name, skillLevel):
			// 包内技能：<prefix>skills/<skill-name>/<rel>
			rest := strings.TrimPrefix(e.Name, skillLevel)
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
				dest = filepath.Join(globalDir, skillName, filepath.FromSlash(rel))
				skillGlobal[skillName] = true
			}
			if err := writeEntryFile(dest, e.Content); err != nil {
				return err
			}
		default:
			// 未知条目（前向兼容）忽略
		}
	}
	return nil
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
