package bundle

import (
	"strconv"
	"strings"
)

// ConflictPolicy 决定「目标已存在同名实体」时的落位方式。
//
// 默认（ConflictAsk）是保守语义：冲突即报错，覆盖必须由调用方显式传 Overwrite。
// 市场招募场景（ConflictSmart）按用户预期自行取舍，不再把冲突当错误抛给人看。
type ConflictPolicy string

const (
	// ConflictAsk 冲突即报错，覆盖需显式 Overwrite（CLI / 外部调用方的默认语义）。
	ConflictAsk ConflictPolicy = ""
	// ConflictSmart 智能取舍：内容一致不重写；本地版本低于包版本则覆盖；
	// 其余（本地改过、版本无法比较）一律保留本地。
	ConflictSmart ConflictPolicy = "smart"
)

// ConflictDecision 是冲突预检的结论。
type ConflictDecision int

const (
	// ConflictInstall 目标不存在，正常展开。
	ConflictInstall ConflictDecision = iota
	// ConflictAsk 按默认语义拒绝，交调用方决定是否带 Overwrite 重试。
	ConflictAskDecision
	// ConflictSkipIdentical 盘上内容与包逐字节一致，跳过写入。
	ConflictSkipIdentical
	// ConflictOverwrite 覆盖写入（本地旧于包，或调用方显式要求）。
	ConflictOverwrite
	// ConflictKeepLocal 保留本地现状，跳过写入。
	ConflictKeepLocal
)

// InstallOutcome 是 InstallResult.Outcome 的取值（安装实际做了什么）。
const (
	OutcomeInstalled   = "installed"
	OutcomeIdentical   = "identical"
	OutcomeOverwritten = "overwritten"
	OutcomeKeptLocal   = "kept_local"
)

// decideConflict 依据策略与事实给出落位结论。事实由调用方提供：
// 目标是否存在由调用方先判（不存在时不必调用本函数），identical 表示盘上
// 内容与包逐字节一致，两个版本都可能为空（老实体未写版本 / 本地导出的包）。
func decideConflict(policy ConflictPolicy, overwrite, identical bool, localVersion, pkgVersion string) ConflictDecision {
	if overwrite {
		return ConflictOverwrite
	}
	if identical {
		return ConflictSkipIdentical
	}
	if policy == ConflictSmart {
		if versionLess(localVersion, pkgVersion) {
			return ConflictOverwrite
		}
		return ConflictKeepLocal
	}
	return ConflictAskDecision
}

// versionLess 判断 a 是否严格旧于 b（按点分段数值比较，缺段按 0）。
// 任一方没有可比较的数字段时返回 false——版本不明时不认定「本地旧」，
// 落位结论会因此退回保留本地。
func versionLess(a, b string) bool {
	av, aok := parseVersionParts(a)
	bv, bok := parseVersionParts(b)
	if !aok || !bok {
		return false
	}
	n := len(av)
	if len(bv) > n {
		n = len(bv)
	}
	for i := 0; i < n; i++ {
		x, y := 0, 0
		if i < len(av) {
			x = av[i]
		}
		if i < len(bv) {
			y = bv[i]
		}
		if x != y {
			return x < y
		}
	}
	return false
}

// parseVersionParts 把 "1.0.4" / "v1.2" / "1.0.0-beta.1" 解析成数字段。
// 只取开头的数字段，遇到非数字段（如 beta）即停止；一段数字都没有则 ok=false。
func parseVersionParts(v string) ([]int, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	if v == "" {
		return nil, false
	}
	parts := make([]int, 0, 4)
	for _, seg := range strings.Split(v, ".") {
		digits := 0
		for digits < len(seg) && seg[digits] >= '0' && seg[digits] <= '9' {
			digits++
		}
		if digits == 0 {
			break
		}
		n, err := strconv.Atoi(seg[:digits])
		if err != nil {
			break
		}
		parts = append(parts, n)
	}
	return parts, len(parts) > 0
}

// localVersion 读本地实体记录在定义文件里的版本；目录不存在或未写版本时为空串。
func localVersion(kind PackageKind, dir string) string {
	v, _, err := LocalVersion(kind, dir)
	if err != nil {
		return ""
	}
	return v
}
