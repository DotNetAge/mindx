package bundle

import (
	"bytes"
	"strings"
)

// hired 是**安装者的运行态**（这个 Agent / 团队在本机是否已雇佣），不是包的语义。
// 让它随包流动会在两个方向上出错：
//   - 装别人的包等于替用户做雇佣决定（包里写 hired: true 就直接生效）；
//   - 覆盖升级一个已雇佣实体时，若新包没写 hired，用户会看到「突然被解雇」。
//
// 因此三条纪律：打包时剥离（发布/导出都不带）、安装时剥离（旧包与第三方包防御）、
// 覆盖安装时把本地原有的雇佣态写回。这里全部按行处理，不做 YAML 重排——frontmatter
// 里可能有手写顺序与注释，重排会让整个文件产生无意义 diff。

// splitLinesKeepEnds 按 \n 切行并保留行尾（\r\n 原样保留在行内）。
func splitLinesKeepEnds(b []byte) [][]byte {
	out := make([][]byte, 0, bytes.Count(b, []byte("\n"))+1)
	for len(b) > 0 {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			out = append(out, b)
			break
		}
		out = append(out, b[:i+1])
		b = b[i+1:]
	}
	return out
}

// isFrontmatterDelimiter 判断该行是否为 frontmatter 分隔符（容忍 BOM 与空白）。
func isFrontmatterDelimiter(line []byte) bool {
	s := strings.TrimSpace(strings.TrimPrefix(string(line), "\ufeff"))
	return s == "---"
}

// frontmatterRange 返回 frontmatter 的起始分隔符行号与结束分隔符行号（含）。
// 不成形（无 frontmatter）时 ok=false。
func frontmatterRange(lines [][]byte) (start, end int, ok bool) {
	if len(lines) == 0 || !isFrontmatterDelimiter(lines[0]) {
		return -1, -1, false
	}
	for i := 1; i < len(lines); i++ {
		if isFrontmatterDelimiter(lines[i]) {
			return 0, i, true
		}
	}
	return -1, -1, false
}

// hiredLineValue 解析 "hired: xxx" 行；第二返回值表示这行是否就是 hired 字段。
func hiredLineValue(line []byte) (value bool, isHired bool) {
	s := strings.TrimSpace(string(line))
	if !strings.HasPrefix(s, "hired:") {
		return false, false
	}
	rest := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(s, "hired:")))
	switch rest {
	case "true", "yes", "on", "1":
		return true, true
	default:
		return false, true
	}
}

// lineEnding 返回该行的换行符（无换行时按 \n）。
func lineEnding(line []byte) string {
	if bytes.HasSuffix(line, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// stripHiredField 删除 frontmatter 里的 hired 字段行，其余字节原样保留。
// 无 frontmatter 或无该字段时返回原内容。
func stripHiredField(content []byte) []byte {
	lines := splitLinesKeepEnds(content)
	start, end, ok := frontmatterRange(lines)
	if !ok {
		return content
	}
	out := make([][]byte, 0, len(lines))
	for i, line := range lines {
		if i > start && i < end {
			if _, isHired := hiredLineValue(line); isHired {
				continue
			}
		}
		out = append(out, line)
	}
	return bytes.Join(out, nil)
}

// frontmatterHired 读取 frontmatter 的 hired（缺省 false，无 frontmatter 亦 false）。
func frontmatterHired(content []byte) bool {
	lines := splitLinesKeepEnds(content)
	start, end, ok := frontmatterRange(lines)
	if !ok {
		return false
	}
	for i := start + 1; i < end; i++ {
		if v, isHired := hiredLineValue(lines[i]); isHired {
			return v
		}
	}
	return false
}

// withHiredTrue 把 frontmatter 的 hired 置为 true：已有该字段则原行改写（保留缩进位置），
// 没有则在结束分隔符前插入。用于覆盖安装时恢复本地雇佣态。
func withHiredTrue(content []byte) []byte {
	lines := splitLinesKeepEnds(content)
	start, end, ok := frontmatterRange(lines)
	if !ok {
		// 没有 frontmatter：补一个最小块，正文原样跟在后面
		return append([]byte("---\nhired: true\n---\n"), content...)
	}
	for i := start + 1; i < end; i++ {
		if _, isHired := hiredLineValue(lines[i]); isHired {
			lines[i] = []byte("hired: true" + lineEnding(lines[i]))
			return bytes.Join(lines, nil)
		}
	}
	out := make([][]byte, 0, len(lines)+1)
	out = append(out, lines[:end]...)
	out = append(out, []byte("hired: true"+lineEnding(lines[end])))
	out = append(out, lines[end:]...)
	return bytes.Join(out, nil)
}
