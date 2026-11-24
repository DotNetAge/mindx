package bundle

import (
	"bytes"
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
	// Policy 冲突处置策略。零值 = 保守语义（冲突报错，覆盖需 Overwrite）；
	// ConflictSmart = 市场招募语义（一致不重写、本地旧则覆盖、其余保留本地）。
	Policy ConflictPolicy
}

// Install 将分发包展开落库（分发格式 ≠ 存储格式，冗余只存在于传输层）：
//   - Agent 包：agent/ → agents/<name>/；包内技能与全局库同名 → 落 Agent 目录
//     skills/（Agent 级覆盖 = 逻辑重写），不同名 → 进全局库；
//   - Skill 包：skills/<name>/ → 全局库；
//   - Team 包：team/ → teams/<name>/；包内 agents/<n>/ 逐成员按 Agent 包规则落位
//     （整队招聘：团队与成员一次装齐，落地后恢复纯引用）；
//   - 同名冲突默认拒绝（返回错误），由界面确认后带 Overwrite 重试；
//     但包内容与盘上逐字节一致（未修改的重复安装）时静默放行，不要求确认；
//   - Policy=ConflictSmart 时改由 decideConflict 自行取舍，不向用户抛冲突。
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

	agentDir, err := joinUnder(opts.AgentsDir, manifest.Name)
	if err != nil {
		return nil, err
	}
	overwritten := false

	// 全局库名集合（决定包内技能落全局还是 Agent 级）
	globalNames, err := dirNames(opts.GlobalDir)
	if err != nil {
		return nil, err
	}

	// 冲突预检：按策略决定覆盖 / 跳过 / 保留本地（包内技能无冲突——同名落
	// Agent 级、不同名进全局库，规则本身消除了技能层的同名写入冲突）
	if _, err := os.Stat(agentDir); err == nil {
		identical := agentPrefixContentIdentical(entries, zipPrefixAgent, agentDir, opts.GlobalDir, globalNames)
		switch decideConflict(opts.Policy, opts.Overwrite, identical, localVersion(KindAgent, agentDir), manifest.Version) {
		case ConflictSkipIdentical:
			// 内容与包一致：不重写文件，但沿用 Overwritten 表示「命中了已存在目标」
			return &InstallResult{Kind: KindAgent, Name: manifest.Name, Overwritten: true, Outcome: OutcomeIdentical}, nil
		case ConflictKeepLocal:
			return &InstallResult{Kind: KindAgent, Name: manifest.Name, Outcome: OutcomeKeptLocal}, nil
		case ConflictOverwrite:
			overwritten = true
		default:
			return nil, fmt.Errorf("已存在同名智能体 %q，与本机内容不同；覆盖安装需显式指定 overwrite（CLI 用 --overwrite）", manifest.Name)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("检查智能体目录失败：%w", err)
	}

	result := &InstallResult{
		Kind:        KindAgent,
		Name:        manifest.Name,
		Overwritten: overwritten,
		Outcome:     outcomeOf(overwritten),
	}
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
// 冲突预检：团队目录或任一成员 Agent 目录已存在 → 默认拒绝（一次预检全部条目）；
// 团队与全部成员的盘上内容与包逐字节一致（未修改的重复招募）时静默放行。
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

	// 冲突预检：团队目录与全部成员 Agent 目录一次检完（避免写一半才发现冲突）。
	// 盘上内容与包逐字节一致（未修改的重复招募）时静默放行，不要求确认
	teamDir, err := joinUnder(opts.TeamsDir, manifest.Name)
	if err != nil {
		return nil, err
	}
	overwritten := false
	conflict := func(existing, label string) error {
		return fmt.Errorf("已存在同名%s %q，与本机内容不同；覆盖安装需显式指定 overwrite（CLI 用 --overwrite）", label, existing)
	}
	teamExists := false
	if _, err := os.Stat(teamDir); err == nil {
		teamExists = true
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("检查团队目录失败：%w", err)
	}
	memberExists := make(map[string]bool)
	anyMemberExists := false
	for _, name := range sortedNames(packedAgents) {
		switch _, err := os.Stat(filepath.Join(opts.AgentsDir, strings.ToLower(name))); {
		case err == nil:
			memberExists[name] = true
			anyMemberExists = true
		case !os.IsNotExist(err):
			return nil, fmt.Errorf("检查智能体目录失败：%w", err)
		}
	}
	if teamExists {
		identical := anyMemberExists && teamUnmodified(entries, teamDir, packedAgents, opts)
		members := sortedNames(packedAgents)
		switch decideConflict(opts.Policy, opts.Overwrite, identical, localVersion(KindTeam, teamDir), manifest.Version) {
		case ConflictSkipIdentical:
			return &InstallResult{Kind: KindTeam, Name: manifest.Name, Members: members, Overwritten: true, Outcome: OutcomeIdentical}, nil
		case ConflictKeepLocal:
			return &InstallResult{Kind: KindTeam, Name: manifest.Name, Members: members, Outcome: OutcomeKeptLocal}, nil
		case ConflictOverwrite:
			overwritten = true
		default:
			return nil, conflict("团队", manifest.Name)
		}
	} else if anyMemberExists {
		// 团队本体缺失但成员已在（装了一半的状态）：补齐本体，按覆盖处理
		if opts.Overwrite || opts.Policy == ConflictSmart {
			overwritten = true
		} else {
			for _, name := range sortedNames(packedAgents) {
				if memberExists[name] {
					return nil, conflict("智能体", name)
				}
			}
		}
	}

	// 全局库名集合（决定包内技能落全局还是 Agent 级）
	globalNames, err := dirNames(opts.GlobalDir)
	if err != nil {
		return nil, err
	}

	// 团队本体：team/TEAM.md → teams/<name>/TEAM.md
	// 与 Agent 同则：剥掉包里的 hired，恢复本地原有的雇佣态
	prevTeamHired := false
	if data, err := os.ReadFile(filepath.Join(teamDir, teamDocFileName)); err == nil {
		prevTeamHired = frontmatterHired(data)
	}
	teamDocWritten := false
	for _, e := range entries {
		if e.Name == zipPathTeamDoc {
			content := stripHiredField(e.Content)
			if prevTeamHired {
				content = withHiredTrue(content)
			}
			if err := writeEntryFile(filepath.Join(teamDir, teamDocFileName), content); err != nil {
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
	result := &InstallResult{
		Kind:        KindTeam,
		Name:        manifest.Name,
		Overwritten: overwritten,
		Outcome:     outcomeOf(overwritten),
	}
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

// joinUnder 把包名作为单层目录拼到 base 之下，并断言结果没有越出 base。
//
// 与 Manifest.validate 的包名白名单互为兜底：白名单是第一道闸（给出可读的
// 拒绝原因），这里是最后一道——将来若有人放宽白名单或新增一条不走 validate
// 的安装路径，越界路径仍会被挡在这里。
func joinUnder(base, name string) (string, error) {
	cleanBase := filepath.Clean(base)
	joined := filepath.Join(cleanBase, strings.ToLower(name))
	if joined != filepath.Join(cleanBase, filepath.Clean(strings.ToLower(name))) ||
		!strings.HasPrefix(joined, cleanBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("包名 %q 会写到安装目录之外，已拒绝", name)
	}
	return joined, nil
}

// installAgentDir 将 zip 前缀 prefix 之下的单个 Agent 目录内容落位到 agentDir：
// 本体文件（IDENTITY.md / SOUL.md 等）直落，agent 级技能按全局库同名分流
// （同名 → agentDir/skills/<n>/ 覆盖版本；不同名 → globalDir 进全局库）。
// 写入计数累计到 skillGlobal / skillAgent（调用方按需初始化，跨成员共享时天然去重）。
// 该函数不预检冲突（调用方负责），installAgent 与 installTeam 共用。
func installAgentDir(entries []zipEntry, prefix, agentDir, globalDir string, globalNames map[string]bool, skillGlobal, skillAgent map[string]bool) error {
	skillLevel := prefix + zipPrefixSkills
	// 覆盖安装前的本地雇佣态：包里的 hired 一律不生效（雇佣是本机决定），
	// 但本地原本已雇佣的，覆盖后必须还是已雇佣——否则升级看起来像被解雇
	prevHired := false
	if data, err := os.ReadFile(filepath.Join(agentDir, identityFileName)); err == nil {
		prevHired = frontmatterHired(data)
	}
	for _, e := range entries {
		switch {
		case e.Name == prefix+identityFileName || e.Name == prefix+soulFileName:
			// Agent 本体文件：IDENTITY.md / SOUL.md
			rel := strings.TrimPrefix(e.Name, prefix)
			content := e.Content
			if rel == identityFileName {
				content = stripHiredField(content)
				if prevHired {
					content = withHiredTrue(content)
				}
			}
			if err := writeEntryFile(filepath.Join(agentDir, filepath.FromSlash(rel)), content); err != nil {
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

// teamUnmodified 判断团队包与盘上现状是否完全一致（本地未修改过的同一份包）：
// TEAM.md 与全部成员 Agent 内容（本体文件 + 技能按落点分流后的最终文件）逐字节比对。
// 任一文件缺失或不一致 → 视为已修改，走正常冲突确认路径。
func teamUnmodified(entries []zipEntry, teamDir string, packedAgents map[string]bool, opts InstallOptions) bool {
	globalNames, err := dirNames(opts.GlobalDir)
	if err != nil {
		return false
	}
	docSeen := false
	for _, e := range entries {
		if e.Name == zipPathTeamDoc {
			// TEAM.md 同样剥掉 hired 再比（与 Agent 的定义文件一致）
			if !fileMatchesIgnoringHired(filepath.Join(teamDir, teamDocFileName), e.Content) {
				return false
			}
			docSeen = true
			break
		}
	}
	if !docSeen {
		return false // 包不完整，走正常冲突路径
	}
	for _, name := range sortedNames(packedAgents) {
		agentDir := filepath.Join(opts.AgentsDir, strings.ToLower(name))
		if !agentPrefixContentIdentical(entries, zipPrefixAgents+name+"/", agentDir, opts.GlobalDir, globalNames) {
			return false
		}
	}
	return true
}

// agentPrefixContentIdentical 判断 zip 前缀 prefix 之下、installAgentDir 将写入的
// 每个文件与盘上现有内容是否逐字节一致（与 installAgentDir 同一套落点分流规则：
// 本体文件直落 agentDir，技能按全局库同名落 agentDir/skills 或全局库）。
// 盘上多出的文件不参与判断（install 不会删除它们）。
func agentPrefixContentIdentical(entries []zipEntry, prefix, agentDir, globalDir string, globalNames map[string]bool) bool {
	skillLevel := prefix + zipPrefixSkills
	for _, e := range entries {
		switch {
		case e.Name == prefix+identityFileName || e.Name == prefix+soulFileName:
			rel := strings.TrimPrefix(e.Name, prefix)
			dest := filepath.Join(agentDir, filepath.FromSlash(rel))
			// hired 是安装者的运行态，不随包流动：比对时必须先剥掉再比，
			// 否则「本地已雇佣」会让未改动的包永远被判成已修改（重装要确认、纳管认不出）。
			if rel == identityFileName {
				if !fileMatchesIgnoringHired(dest, e.Content) {
					return false
				}
				break
			}
			if !fileMatches(dest, e.Content) {
				return false
			}
		case strings.HasPrefix(e.Name, skillLevel):
			rest := strings.TrimPrefix(e.Name, skillLevel)
			skillName, rel, ok := strings.Cut(rest, "/")
			if !ok || skillName == "" || rel == "" {
				continue // 技能目录级条目，跳过
			}
			var dest string
			if globalNames[skillName] {
				dest = filepath.Join(agentDir, "skills", skillName, filepath.FromSlash(rel))
			} else {
				dest = filepath.Join(globalDir, skillName, filepath.FromSlash(rel))
			}
			if !fileMatches(dest, e.Content) {
				return false
			}
		}
	}
	return true
}

// fileMatches 盘上文件存在且内容与期望逐字节一致。
func fileMatches(path string, content []byte) bool {
	existing, err := os.ReadFile(path)
	return err == nil && bytes.Equal(existing, content)
}

// fileMatchesIgnoringHired 比较时两侧都剥掉 frontmatter 的 hired 字段。
// 只用于定义文件（IDENTITY.md / TEAM.md）：hired 是「本机是否雇佣」的运行态，
// 打包时被剥离、安装时保留本地值，因此它不该参与「内容是否被改过」的判断。
func fileMatchesIgnoringHired(path string, content []byte) bool {
	existing, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Equal(stripHiredField(existing), stripHiredField(content))
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
	// skipWrite 记录按策略不落盘的技能名（一致或保留本地），写入阶段整体跳过
	skipWrite := make(map[string]bool)
	identicals, keptLocals := 0, 0
	for _, name := range sortedNames(packedSkills) {
		dir := filepath.Join(opts.GlobalDir, name)
		if _, err := os.Stat(dir); err == nil {
			identical := skillPrefixContentIdentical(entries, zipPrefixSkills+name+"/", dir)
			// 保守语义下技能库同名一律拒绝（历史契约，本地导入与 CLI 依赖它）；
			// 只有 smart 策略才把「内容一致」当作无需重写的理由
			if identical && opts.Policy != ConflictSmart && !opts.Overwrite {
				return nil, fmt.Errorf("全局库已存在同名技能 %q；覆盖安装需显式指定 overwrite（CLI 用 --overwrite）", name)
			}
			switch decideConflict(opts.Policy, opts.Overwrite, identical, localVersion(KindSkill, dir), manifest.Version) {
			case ConflictSkipIdentical:
				skipWrite[name] = true
				identicals++
			case ConflictKeepLocal:
				skipWrite[name] = true
				keptLocals++
			case ConflictOverwrite:
				overwritten = true
			default:
				return nil, fmt.Errorf("全局库已存在同名技能 %q；覆盖安装需显式指定 overwrite（CLI 用 --overwrite）", name)
			}
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
		if name, _, ok := strings.Cut(rel, "/"); ok && skipWrite[name] {
			continue
		}
		if err := writeEntryFile(filepath.Join(opts.GlobalDir, filepath.FromSlash(rel)), e.Content); err != nil {
			return nil, err
		}
	}

	outcome := OutcomeInstalled
	if len(packedSkills) > 0 && identicals+keptLocals == len(packedSkills) {
		outcome = OutcomeIdentical
		if keptLocals > 0 {
			outcome = OutcomeKeptLocal
		}
	} else if overwritten {
		outcome = OutcomeOverwritten
	}
	return &InstallResult{
		Kind:         KindSkill,
		Name:         manifest.Name,
		SkillsGlobal: sortedNames(packedSkills),
		Overwritten:  overwritten,
		Outcome:      outcome,
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

// outcomeOf 把「是否覆盖」折算成落位结果标签。
func outcomeOf(overwritten bool) string {
	if overwritten {
		return OutcomeOverwritten
	}
	return OutcomeInstalled
}

// skillPrefixContentIdentical 判断包内某技能目录（prefix 之下）的每个文件
// 与盘上现有内容是否逐字节一致；盘上多出的文件（如本地 notes/）不参与判断。
func skillPrefixContentIdentical(entries []zipEntry, prefix, dir string) bool {
	for _, e := range entries {
		if !strings.HasPrefix(e.Name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(e.Name, prefix)
		if rel == "" || strings.HasSuffix(rel, "/") {
			continue
		}
		if !fileMatches(filepath.Join(dir, filepath.FromSlash(rel)), e.Content) {
			return false
		}
	}
	return true
}
