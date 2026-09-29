package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// 子进程 PATH 增强：daemon 常由 GUI（launchd/Electron）拉起，进程 PATH 仅含系统
// 基础目录（实测 /Users/ray/.mindx/bin:/usr/local/bin:/usr/bin:/bin），用户级安装的
// 运行时（nvm 的 node/npx、homebrew 等）不在其中，stdio 连接器 exec "npx" 即报
// executable file not found。惯例做法（VSCode/Claude Desktop 同款）：以用户登录
// shell 探测完整 PATH 并入子进程环境；探测失败静默回退 daemon 自身 PATH（容错
// 优先，不阻塞连接）。

// loginShellPathTimeout 探测超时：nvm.sh 等初始化较慢（秒级），rc 挂死时放弃探测
const loginShellPathTimeout = 5 * time.Second

// loginShellPath 以登录 shell 探测用户 PATH（进程级缓存：rc 开销只吃一次）。
//   - 命令：$SHELL -l -i -c 'echo $PATH'（交互式登录，VSCode 同款）——nvm 等版本
//     管理器普遍只写在 ~/.zshrc / ~/.bashrc（交互式才加载），纯 -l 不够
//   - 回退链：$SHELL → /bin/zsh → /bin/bash（macOS/Linux）
//   - 取输出中最后一个含分隔符的非空行（交互模式 rc 可能 echo 杂物污染 stdout）
//   - Windows 无登录 shell 语义，直接返回空串
var loginShellPath = sync.OnceValue(func() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	shells := []string{os.Getenv("SHELL"), "/bin/zsh", "/bin/bash"}
	for _, sh := range shells {
		if sh == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), loginShellPathTimeout)
		out, err := exec.CommandContext(ctx, sh, "-l", "-i", "-c", "echo $PATH").Output()
		cancel()
		if err != nil {
			continue
		}
		lines := strings.Split(string(out), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if p := strings.TrimSpace(lines[i]); strings.Contains(p, string(os.PathListSeparator)) {
				return p
			}
		}
	}
	return ""
})

// enhancedPath 返回并入用户 PATH 后的合并结果（用户段在前——与用户终端行为一致，
// 同名工具优先命中用户版本；保序去重）。
func enhancedPath() string {
	merged := mergePaths(loginShellPath(), os.Getenv("PATH"))
	if merged == "" {
		return os.Getenv("PATH")
	}
	return merged
}

// resolveCommand 在增强 PATH 里解析裸命令名为绝对路径。关键背景：exec.Command
// 构造期的命令解析只看 daemon 自身 PATH（cmd.Env 不参与 LookPath），GUI 拉起的
// 精简 PATH 直接导致 exec "npx" 失败——必须先解析再构造。
//   - 含路径分隔符的命令原样返回（相对/绝对路径交 exec 自行处理）
//   - Windows 构建跳过（探测语义不适用），保留原生 LookPath 行为
//   - 解析失败返回原名，保留 exec 原生错误语义（"npx": executable file not found）
func resolveCommand(command string) string {
	if runtime.GOOS == "windows" || strings.ContainsRune(command, os.PathSeparator) {
		return command
	}
	for seg := range strings.SplitSeq(enhancedPath(), string(os.PathListSeparator)) {
		if !filepath.IsAbs(seg) {
			continue
		}
		full := filepath.Join(seg, command)
		info, err := os.Stat(full)
		if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			continue
		}
		return full
	}
	return command
}

// mergePaths 保序去重合并多段 PATH（分隔符跨平台取 os.PathListSeparator）。
func mergePaths(paths ...string) string {
	sep := os.PathListSeparator
	seen := make(map[string]struct{})
	var parts []string
	for _, p := range paths {
		for seg := range strings.SplitSeq(p, string(sep)) {
			seg = strings.TrimSpace(seg)
			if seg == "" {
				continue
			}
			if _, ok := seen[seg]; ok {
				continue
			}
			seen[seg] = struct{}{}
			parts = append(parts, seg)
		}
	}
	return strings.Join(parts, string(sep))
}
