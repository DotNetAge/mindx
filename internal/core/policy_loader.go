package core

// policy_loader.go 提供 ~/.mindx/policy.json 的沙箱豁免扩展机制。
//
// 设计原则：三类被管制的信息（Path / Command / Network）除默认配置外，
// 都应能从外部扩展豁免项。本文件是统一的配置入口：
//
//   - Path:    file.allowed_dirs / file.allowed_paths / file.allowed_globs
//     → SandboxPolicy.AllowedDirs / AllowedFilePaths / AllowedFileGlobs
//     （allowed_dirs 扩展目录边界——工作区外的目录免授权，如 /tmp；
//     allowed_paths / allowed_globs 豁免敏感文件硬拒绝，不豁免目录边界
//     与设备文件，见 sandbox 包设计）
//   - Command: command.allowed                        → SandboxPolicy.AllowedCommands（追加命令白名单）
//   - Network: network.allow_cidrs                    → SandboxPolicy.NetworkAllowSubnets（CIDR，优先于 deny）
//
// 语义：增量豁免——追加到默认策略之上，不替换任何黑名单。
// 文件缺失属常规路径（返回零值，不告警）；JSON 损坏或单项 CIDR 非法
// 仅告警跳过，不阻断启动（对齐 sandbox 包 parseCIDRs 的容错哲学）。
//
// 首个豁免项：~/.mindx/USER.md——秘书 Agent 维护的用户偏好文件，
// 是所有 Agent 了解用户的共同入口，必须始终可读写。

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/goharness/sandbox"

	"github.com/DotNetAge/goharness/logging"
)

// policyFileOverrides 是文件访问的豁免配置。
type policyFileOverrides struct {
	// AllowedDirs 是显式豁免的目录边界（绝对路径或 ~/ 前缀）：
	// 目录子树整体免授权（不触发越界 AskUser），如 /tmp。
	AllowedDirs []string `json:"allowed_dirs"`
	// AllowedPaths 是显式豁免的文件路径（绝对路径或 ~/ 前缀；
	// 含 "*" 的条目按路径通配匹配，单个 * 跨任意层级）。
	AllowedPaths []string `json:"allowed_paths"`
	// AllowedGlobs 是显式豁免的文件名 glob 模式（仅匹配 basename）。
	AllowedGlobs []string `json:"allowed_globs"`
}

// policyCommandOverrides 是命令执行的豁免配置。
type policyCommandOverrides struct {
	// Allowed 是追加到命令白名单的命令名（basename，大小写不敏感）。
	Allowed []string `json:"allowed"`
}

// policyNetworkOverrides 是网络访问的豁免配置。
type policyNetworkOverrides struct {
	// AllowCIDRs 是显式放行的 IP 段（CIDR 格式，优先于默认 deny 列表）。
	AllowCIDRs []string `json:"allow_cidrs"`
}

// policyOverrides 是 policy.json 的顶层结构。
type policyOverrides struct {
	File    policyFileOverrides    `json:"file"`
	Command policyCommandOverrides `json:"command"`
	Network policyNetworkOverrides `json:"network"`
}

// loadPolicyOverrides 读取 dir/policy.json。
// 文件不存在返回零值与 nil（常规路径，用户未定制豁免）。
func loadPolicyOverrides(dir string) (policyOverrides, error) {
	var ov policyOverrides
	path := filepath.Join(dir, "policy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ov, nil
		}
		return ov, fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	if err := json.Unmarshal(data, &ov); err != nil {
		return ov, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	// ~/ 前缀展开为用户主目录，避免 policy.json 里写死机器相关绝对路径
	for i, p := range ov.File.AllowedPaths {
		ov.File.AllowedPaths[i] = expandTilde(p)
	}
	for i, d := range ov.File.AllowedDirs {
		ov.File.AllowedDirs[i] = expandTilde(d)
	}
	return ov, nil
}

// applyPolicyOverrides 把外部豁免增量并入基础策略（追加，不替换默认黑名单）。
// CIDR 解析失败的项记 Warn 并跳过，不阻断启动。
func applyPolicyOverrides(p *sandbox.SandboxPolicy, ov policyOverrides, logger logging.Logger) {
	p.AllowedDirs = append(p.AllowedDirs, ov.File.AllowedDirs...)
	p.AllowedFilePaths = append(p.AllowedFilePaths, ov.File.AllowedPaths...)
	p.AllowedFileGlobs = append(p.AllowedFileGlobs, ov.File.AllowedGlobs...)
	p.AllowedCommands = append(p.AllowedCommands, ov.Command.Allowed...)
	for _, cidr := range ov.Network.AllowCIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil || n == nil {
			logger.Warn("policy.json 网络白名单项非法，已跳过", "cidr", cidr)
			continue
		}
		p.NetworkAllowSubnets = append(p.NetworkAllowSubnets, n)
	}
}

// expandTilde 把 "~" 或 "~/..." 前缀展开为用户主目录；其余原样返回。
// 主目录解析失败时原样保留 "~/..." 前缀——该条目将无法命中任何真实路径
// （静默失效，方向 fail-closed），无安全风险。
func expandTilde(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}
