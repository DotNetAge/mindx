package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DotNetAge/mindx/internal/core/agentstore"
)

// 存量纳管：给「版本字段出现之前装的包」补一个可信的版本号。
//
// 定案原本写「不迁移，重装即获血统」，理由是：回填要靠内容比对去认领版本，
// 一旦认错就把别人的血统挂到本地改动上。本文实现的是**不猜**的那一半——
// 只有在「本地内容与某一版包逐字节一致」时才认领该版本（hired 作为运行态剔除）。
// 逐字节相等不存在认错的可能；一个版本都不匹配就如实说「认不出来」，不写。
//
// 这与「三方合并」是两件事：纳管只补一个字段，不改任何内容。

// DefaultVersion 本地自建实体的版本起点。
// 直接引用 agentstore 的常量而不是再写一遍字面量：版本起点只有一个值，
// 三处各写一遍迟早改漏一处。
const DefaultVersion = agentstore.DefaultVersion

// MatchInstalledVersion 找出本地目录内容对应市场里的哪个版本。
// 返回 ("", false, nil) 表示没有任一版本与之逐字节一致（本地改过 / 来源不同）。
func MatchInstalledVersion(kind PackageKind, dir string, pkg MarketPackage, opts InstallOptions, client *MarketClient) (string, bool, error) {
	if client == nil {
		return "", false, fmt.Errorf("缺少市场客户端")
	}
	// 版本按发布时间倒序：先比最近的版本，命中就早退（多数情况第一版就中）
	for _, v := range pkg.Versions {
		if v.Deleted {
			continue
		}
		path, err := client.DownloadVersion(pkg, v.Version)
		if err != nil {
			continue // 单版取不到不影响其他版本
		}
		same, cerr := DefinitionMatches(path, dir, opts)
		if cerr != nil {
			continue
		}
		if same {
			return v.Version, true, nil
		}
	}
	return "", false, nil
}

// versionPattern 版本号允许的字符集：字母数字与 . + _ -，首字符非符号，最长 64。
// 覆盖 semver（1.2.3、1.2.3-beta.1）与历史时间戳形态（20261116-123456）。
// 必须校验：版本号来自远端市场响应，直接拼进 frontmatter 会被用来注入任意 YAML 字段
// （例如在版本号里塞一个换行加 `hired: true`，就把「装个包」变成「替用户雇佣」）。
var versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$`)

// WriteLocalVersion 把版本号写进本地定义文件的 frontmatter，其余字节原样保留。
// 已有 version 行则原位改写；没有则插在结束分隔符之前。
func WriteLocalVersion(kind PackageKind, dir, version string) error {
	v := strings.TrimSpace(version)
	if v == "" {
		return fmt.Errorf("版本号不能为空")
	}
	if !versionPattern.MatchString(v) {
		return fmt.Errorf("版本号 %q 含非法字符，拒绝写入（只允许字母数字与 . + _ -）", v)
	}
	doc, err := docFileNameFor(kind)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, doc)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := splitLinesKeepEnds(data)
	start, end, ok := frontmatterRange(lines)
	if !ok {
		return fmt.Errorf("%s 缺少 frontmatter，无法写入版本号", doc)
	}
	for i := start + 1; i < end; i++ {
		trimmed := strings.TrimSpace(string(lines[i]))
		if !strings.HasPrefix(trimmed, "version:") {
			continue
		}
		// 只替换顶层的 version（缩进行属于 metadata 块，不动）
		raw := strings.TrimRight(string(lines[i]), "\r\n")
		if raw != strings.TrimLeft(raw, " \t") {
			continue
		}
		lines[i] = []byte("version: " + v + lineEnding(lines[i]))
		return os.WriteFile(path, joinLines(lines), 0644)
	}
	out := make([][]byte, 0, len(lines)+1)
	out = append(out, lines[:end]...)
	out = append(out, []byte("version: "+v+"\n"))
	out = append(out, lines[end:]...)
	return os.WriteFile(path, joinLines(out), 0644)
}

// joinLines 按原样拼接行（行尾已在各行内）。
func joinLines(lines [][]byte) []byte {
	var b []byte
	for _, l := range lines {
		b = append(b, l...)
	}
	return b
}
