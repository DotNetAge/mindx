package bundle

import (
	"fmt"
	"strings"
)

// trimSpace 统一空白处理（版本号两端空格无意义）。
func trimSpace(s string) string { return strings.TrimSpace(s) }

// entityVersionDoc 返回承载版本的实体定义文件（报错文案用）。
func entityVersionDoc(kind PackageKind) string {
	switch kind {
	case KindTeam:
		return "TEAM.md"
	case KindSkill:
		return "SKILL.md"
	default:
		return "IDENTITY.md"
	}
}

// ResolvePublishVersion 收敛发布版本的唯一权威：**实体 frontmatter 里的 version 是血统事实源**。
//
//   - 参数为空：取 frontmatter（发布表单可以不管版本，直接跟随定义文件）；
//   - 参数与 frontmatter 都非空：必须完全一致——不一致就拒绝，否则同一个包会同时存在
//     「表单版本」与「文件版本」两个事实源，升级比对立刻失效；
//   - 两者都空：拒绝（发布必须有版本；无版本的包进不了升级链）。
//
// kind/name 仅用于报错文案（告诉用户去改哪个文件的哪一行）。
func ResolvePublishVersion(kind PackageKind, name, paramVersion, frontmatterVersion string) (string, error) {
	param := trimSpace(paramVersion)
	fm := trimSpace(frontmatterVersion)
	doc := entityVersionDoc(kind)

	if param == "" && fm == "" {
		return "", fmt.Errorf(
			"发布「%s」需要一个版本号：请在它的 %s frontmatter 里写 version（例如 1.2.0），或在发布表单里填写",
			name, doc,
		)
	}
	if param != "" && fm != "" && param != fm {
		return "", fmt.Errorf(
			"版本号有两个说法：发布表单填的是 %s，%s 里写的是 %s。只保留一处，另一处请改成一致",
			param, doc, fm,
		)
	}
	if fm != "" {
		return fm, nil
	}
	return param, nil
}
