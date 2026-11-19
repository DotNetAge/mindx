//go:build darwin

package discovery

// isDarwin 标记当前是否 macOS。
//
// 单独放文件而不是用 runtime.GOOS 判断：runtime 判断也行，但用构建标签
// 可以让非 macOS 的构建完全不包含 scutil 调用路径。
const isDarwin = true
