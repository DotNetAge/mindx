package bundle

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// 定义文件比较（升级判定与存量纳管都用它）。
//
// 为什么不能比原始字节：这些文件会被我们自己的写入路径重新序列化——雇佣/解雇
// 会走 AgentStore.Save，整份 YAML 被重新 marshal，列表缩进、键顺序、标量引号
// 全都可能变。于是「内容其实没变」的文件在字节层面永远不相等，后果是：
//   - 内容闸永远判定「本地改过」→ 每次升级都要用户确认；
//   - 存量纳管永远认不出版本。
// 两条都是实证过的（本地 ai-artist 与线上 1.0.0 的 IDENTITY.md 只差排版与 version）。
//
// 因此比较改成**语义比较**：解析 frontmatter，剔除运行态与血统字段（hired / version），
// 再各自规范序列化后比对；正文按字节比。语义相同即视为同一份定义。
//
// 比较范围只含「这个实体自己的定义文件」（Agent: IDENTITY.md + SOUL.md；
// Team: TEAM.md + 成员的定义文件），不含包内附带的技能副本与全局技能库——
// 技能有自己的版本链，拿共享库的差异去判定「这个 Agent 被改过」是错的。

// volatileFrontmatterKeys 不参与定义比较的字段：前者是安装者的运行态，
// 后者正是本次要判定/写入的对象。
var volatileFrontmatterKeys = []string{"hired", "version"}

// DefinitionMatches 盘上实体的定义是否与包内定义语义一致。
// dir 为空时按包内清单名推导（与安装落点同规则）。
func DefinitionMatches(pkgPath, dir string, opts InstallOptions) (bool, error) {
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
		agentDir := dir
		if agentDir == "" {
			if strings.TrimSpace(opts.AgentsDir) == "" {
				return false, fmt.Errorf("比对 Agent 定义需要指定 agents 目录")
			}
			if agentDir, err = joinUnder(opts.AgentsDir, manifest.Name); err != nil {
				return false, err
			}
		}
		return agentDefinitionIdentical(entries, zipPrefixAgent, agentDir), nil

	case KindTeam:
		teamDir := dir
		if teamDir == "" {
			if strings.TrimSpace(opts.TeamsDir) == "" {
				return false, fmt.Errorf("比对团队定义需要指定 teams 目录")
			}
			if teamDir, err = joinUnder(opts.TeamsDir, manifest.Name); err != nil {
				return false, err
			}
		}
		return teamDefinitionIdentical(entries, teamDir, opts), nil

	case KindSkill:
		// 技能包就是它的 SKILL.md 与资源文件，按字节比即可（技能库不会被运行时重写）
		if dir != "" {
			return skillContentIdenticalAt(entries, dir), nil
		}
		return skillContentIdentical(entries, opts.GlobalDir), nil

	default:
		return false, fmt.Errorf("未知的分发包类型：%q", manifest.Kind)
	}
}

// agentDefinitionIdentical 比对一个 Agent 的定义文件（IDENTITY.md 语义、SOUL.md 字节）。
func agentDefinitionIdentical(entries []zipEntry, prefix, agentDir string) bool {
	sawDoc := false
	for _, e := range entries {
		switch e.Name {
		case prefix + identityFileName:
			if !definitionFileEquals(filepath.Join(agentDir, identityFileName), e.Content) {
				return false
			}
			sawDoc = true
		case prefix + soulFileName:
			// SOUL.md 同样按语义比：我们自己的写入路径写出的文件尾部可能少一个换行，
			// 那不是用户改动（实测本地与线上只差这一个字节）
			if !definitionFileEquals(filepath.Join(agentDir, soulFileName), e.Content) {
				return false
			}
		}
	}
	return sawDoc
}

// teamDefinitionIdentical 比对团队的 TEAM.md 与全部成员的定义文件。
func teamDefinitionIdentical(entries []zipEntry, teamDir string, opts InstallOptions) bool {
	sawDoc := false
	for _, e := range entries {
		if e.Name == zipPathTeamDoc {
			if !definitionFileEquals(filepath.Join(teamDir, teamDocFileName), e.Content) {
				return false
			}
			sawDoc = true
			break
		}
	}
	if !sawDoc {
		return false
	}
	for name := range packedAgentNames(entries) {
		memberDir := filepath.Join(opts.AgentsDir, strings.ToLower(name))
		if !agentDefinitionIdentical(entries, zipPrefixAgents+name+"/", memberDir) {
			return false
		}
	}
	return true
}

// definitionFileEquals 语义比较一份定义文件：frontmatter 解析后剔除
// hired / version 再规范序列化比对，正文按字节比（忽略行尾空白差异）。
func definitionFileEquals(path string, content []byte) bool {
	existing, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return semanticDefinitionEquals(existing, content)
}

// semanticDefinitionEquals 见 definitionFileEquals。
func semanticDefinitionEquals(a, b []byte) bool {
	fmA, bodyA := splitDefinition(a)
	fmB, bodyB := splitDefinition(b)
	if !bytes.Equal(normalizeBody(bodyA), normalizeBody(bodyB)) {
		return false
	}
	return canonicalFrontmatter(fmA) == canonicalFrontmatter(fmB)
}

// splitDefinition 切出 frontmatter 与正文（无 frontmatter 时前者为空）。
func splitDefinition(data []byte) (frontmatter, body []byte) {
	lines := splitLinesKeepEnds(data)
	start, end, ok := frontmatterRange(lines)
	if !ok {
		return nil, data
	}
	var fm, rest []byte
	for i := start + 1; i < end; i++ {
		fm = append(fm, lines[i]...)
	}
	for i := end + 1; i < len(lines); i++ {
		rest = append(rest, lines[i]...)
	}
	return fm, rest
}

// canonicalFrontmatter 把 frontmatter 解析成 map、剔除 volatile 键后用 YAML
// 规范序列化（map 的键序由 yaml 库排序，缩进与引号风格统一）。
// 解析失败时退回原文比较——解析不了的 frontmatter 不该被判成「相同」。
func canonicalFrontmatter(fm []byte) string {
	if len(bytes.TrimSpace(fm)) == 0 {
		return ""
	}
	var raw map[string]any
	if err := yaml.Unmarshal(fm, &raw); err != nil {
		return "\x00raw:" + string(fm)
	}
	for _, k := range volatileFrontmatterKeys {
		delete(raw, k)
	}
	out, err := yaml.Marshal(raw)
	if err != nil {
		return "\x00raw:" + string(fm)
	}
	return string(out)
}

// normalizeBody 去掉正文首尾空白（写入路径可能在文末补换行，那不是内容差异）。
func normalizeBody(body []byte) []byte {
	return bytes.TrimSpace(body)
}
