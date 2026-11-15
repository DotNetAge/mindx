package setup

import (
	"bytes"
	"encoding/xml"
	"strings"
)

// 安装目录会进入四种宿主格式：shell rc、launchd plist、systemd unit、以及
// 命令行参数。每种格式的转义规则不同，**必须按落点转义**，不能靠「路径里
// 一般不会有特殊字符」这种假设——`--dir` 由上层 Client 子进程传入，静默
// 安装模式下没有人工确认环节。
//
// 共同前提：路径本身已通过 validateInstallDir 的合法性检查（绝对路径、
// 无控制字符）。以下函数只负责「在各自格式里成为字面量」。

// shellQuoteSingle 把字符串包成 POSIX 单引号字面量。
// 单引号内不做任何展开（$ ` \ 都失去含义），内部单引号按 shell 规矩写成 '\”。
func shellQuoteSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// xmlEscapeText 转义 XML 文本节点内容（plist 的 <string> 里用）。
func xmlEscapeText(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// systemdQuote 把值包成 systemd 支持的双引号字面量（可安全出现在
// ExecStart / WorkingDirectory / Environment / StandardOutput 等赋值右侧）。
// systemd 引号内只认 \" 与 \\ 两种转义。
func systemdQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
