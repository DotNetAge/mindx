package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 本文件是升级链路的两把闸（设计与契约见 mindx-market/docs/设计定案-市场包血统与升级链路.md）：
//
//   - 版本闸：本地定义文件 frontmatter 的 version 与市场最新版本是否相同；
//   - 内容闸：盘上现装内容与「已装那版」的包字节是否逐字节一致（判断本地改没改过）。
//
// 血统只有一个事实源：定义文件 frontmatter。不落旁挂清单——旁挂文件是第二事实源，
// 导出/导入/转发/覆盖任何一条路径漏写就断链；写进 frontmatter 则随文件走，
// 打包即发布、落盘即生效。

// skillDocName 技能定义文件名（技能包与本地技能库同名同布局）。
const skillDocName = "SKILL.md"

// docFileNameFor 包类型 → 记录血统的定义文件。
func docFileNameFor(kind PackageKind) (string, error) {
	switch kind {
	case KindAgent:
		return identityFileName, nil
	case KindTeam:
		return teamDocFileName, nil
	case KindSkill:
		return skillDocName, nil
	default:
		return "", fmt.Errorf("未知的分发包类型：%q", kind)
	}
}

// LocalVersion 读取本地已装包记录在定义文件 frontmatter 里的版本号。
// found=false 表示该包没有记录版本（定案前的旧安装、或第三方手工放入），不是错误——
// 调用方据此提示「来源不明，重装即获血统」，不要猜。
func LocalVersion(kind PackageKind, dir string) (version string, found bool, err error) {
	doc, err := docFileNameFor(kind)
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(filepath.Join(dir, doc))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	v, ok := frontmatterVersion(data, kind == KindSkill)
	return v, ok, nil
}

// frontmatterVersion 从 frontmatter 取 version：
// 顶层的 `version:` 优先；技能另允许 metadata 块下的 `version:`（SKILL.md 的
// 扩展键都写在 metadata 里，build-market 取的正是 metadata.version）。
func frontmatterVersion(content []byte, allowMetadata bool) (string, bool) {
	lines := splitLinesKeepEnds(content)
	start, end, ok := frontmatterRange(lines)
	if !ok {
		return "", false
	}
	inMeta := false
	var nested string
	for i := start + 1; i < end; i++ {
		raw := strings.TrimRight(string(lines[i]), "\r\n")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indented := raw != strings.TrimLeft(raw, " \t")
		key, value, hasColon := strings.Cut(trimmed, ":")
		if !hasColon {
			continue
		}
		key = strings.TrimSpace(key)
		if !indented {
			inMeta = key == "metadata"
		}
		if key != "version" {
			continue
		}
		if !indented {
			return unquoteScalar(value), true
		}
		if allowMetadata && inMeta && nested == "" {
			nested = unquoteScalar(value) // 记下来，顶层没写时才用
		}
	}
	if nested != "" {
		return nested, true
	}
	return "", false
}

// unquoteScalar 去掉 YAML 标量两侧引号与空白。
func unquoteScalar(raw string) string {
	s := strings.TrimSpace(raw)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			s = s[1 : len(s)-1]
		}
	}
	return strings.TrimSpace(s)
}

// ContentMatches 内容闸：盘上现装内容是否与给定包文件逐字节一致。
//
// 语义边界（务必按这个用）：一致只能推出「自安装以来没被本地改过」，
// 不能推出「等于哪个版本还完好」——所以调用方必须传入**本地记录的那个版本**的
// 包文件来比对，而不是最新版；拿最新版比会把「有更新」误判成「被改过」。
//
// 盘上多出的文件不参与判断（安装本就不删文件；残留不该阻止升级）。
func ContentMatches(pkgPath string, opts InstallOptions) (bool, error) {
	if strings.TrimSpace(opts.GlobalDir) == "" {
		return false, fmt.Errorf("全局技能库目录不能为空")
	}
	entries, err := readPackageEntries(pkgPath)
	if err != nil {
		return false, err
	}
	manifest, err := manifestFromEntries(entries)
	if err != nil {
		return false, err
	}
	switch manifest.Kind {
	case KindAgent:
		if strings.TrimSpace(opts.AgentsDir) == "" {
			return false, fmt.Errorf("比对 Agent 内容需要指定 agents 目录")
		}
		agentDir, err := joinUnder(opts.AgentsDir, manifest.Name)
		if err != nil {
			return false, err
		}
		globalNames, err := dirNames(opts.GlobalDir)
		if err != nil {
			return false, err
		}
		return agentPrefixContentIdentical(entries, zipPrefixAgent, agentDir, opts.GlobalDir, globalNames), nil
	case KindTeam:
		if strings.TrimSpace(opts.TeamsDir) == "" || strings.TrimSpace(opts.AgentsDir) == "" {
			return false, fmt.Errorf("比对团队内容需要指定 teams 与 agents 目录")
		}
		teamDir, err := joinUnder(opts.TeamsDir, manifest.Name)
		if err != nil {
			return false, err
		}
		packedAgents := packedAgentNames(entries)
		if len(packedAgents) == 0 {
			return false, fmt.Errorf("团队包缺少成员 Agent 目录（agents/<name>/）")
		}
		return teamUnmodified(entries, teamDir, packedAgents, opts), nil
	case KindSkill:
		return skillContentIdentical(entries, opts.GlobalDir), nil
	default:
		return false, fmt.Errorf("未知的分发包类型：%q", manifest.Kind)
	}
}

// packedAgentNames 包内成员 Agent 名集合（agents/<n>/ 前缀一级目录）。
func packedAgentNames(entries []zipEntry) map[string]bool {
	out := make(map[string]bool)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name, zipPrefixAgents) {
			continue
		}
		if name, _, ok := strings.Cut(strings.TrimPrefix(e.Name, zipPrefixAgents), "/"); ok && name != "" {
			out[name] = true
		}
	}
	return out
}

// skillContentIdentical 技能包内容是否与全局库现装内容逐字节一致。
// zip 内路径穿越（..）一律判为不一致——这种包本身就不该被安装。
func skillContentIdentical(entries []zipEntry, globalDir string) bool {
	for _, e := range entries {
		if !strings.HasPrefix(e.Name, zipPrefixSkills) {
			continue
		}
		rest := strings.TrimPrefix(e.Name, zipPrefixSkills)
		if rest == "" || strings.HasSuffix(rest, "/") {
			continue // 技能目录级条目
		}
		if strings.Contains(rest, "..") {
			return false
		}
		if !fileMatches(filepath.Join(globalDir, filepath.FromSlash(rest)), e.Content) {
			return false
		}
	}
	return true
}
