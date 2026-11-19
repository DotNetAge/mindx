//go:build !darwin

package discovery

// isDarwin 在非 macOS 平台为 false（这些平台没有 Bonjour 名与 uname 名之分）。
const isDarwin = false
