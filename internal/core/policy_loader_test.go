package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/goharness/logging"
	"github.com/DotNetAge/goharness/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPolicyLogger() logging.Logger {
	return logging.NewNopLogger()
}

// TestLoadPolicyOverrides_MissingFile_ReturnsZeroValues 验证文件缺失是常规路径：
// 返回零值与 nil，调用方无需特殊处理。
func TestLoadPolicyOverrides_MissingFile_ReturnsZeroValues(t *testing.T) {
	dir := t.TempDir()

	ov, err := loadPolicyOverrides(dir)

	require.NoError(t, err)
	assert.Empty(t, ov.File.AllowedPaths)
	assert.Empty(t, ov.File.AllowedGlobs)
	assert.Empty(t, ov.Command.Allowed)
	assert.Empty(t, ov.Network.AllowCIDRs)
}

// TestLoadPolicyOverrides_Parse 验证三类配置的解析与 ~/ 前缀展开。
func TestLoadPolicyOverrides_Parse(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"file": {
			"allowed_paths": ["~/.mindx/USER.md"],
			"allowed_globs": ["*.pem.example"]
		},
		"command": { "allowed": ["ffmpeg", "FFprobe"] },
		"network": { "allow_cidrs": ["192.168.1.0/24"] }
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.json"), []byte(content), 0644))

	ov, err := loadPolicyOverrides(dir)

	require.NoError(t, err)
	home, homeErr := os.UserHomeDir()
	require.NoError(t, homeErr)
	assert.Equal(t, []string{filepath.Join(home, ".mindx", "USER.md")}, ov.File.AllowedPaths,
		"~/ 前缀应展开为用户主目录")
	assert.Equal(t, []string{"*.pem.example"}, ov.File.AllowedGlobs)
	assert.Equal(t, []string{"ffmpeg", "FFprobe"}, ov.Command.Allowed,
		"命令名保留原样，小写归一化由 sandbox.Compile 负责")
	assert.Equal(t, []string{"192.168.1.0/24"}, ov.Network.AllowCIDRs)
}

// TestLoadPolicyOverrides_BadJSON_ReturnsError 验证 JSON 损坏返回错误（调用方告警后忽略）。
func TestLoadPolicyOverrides_BadJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.json"), []byte("{broken"), 0644))

	_, err := loadPolicyOverrides(dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy.json")
}

// TestApplyPolicyOverrides_MergesIncrementally 验证合并语义：追加到默认策略，不替换黑名单。
func TestApplyPolicyOverrides_MergesIncrementally(t *testing.T) {
	base := sandbox.SandboxPolicy{
		AllowedCommands:    sandbox.DefaultAllowedCommands(),
		DeniedFileGlobs:    sandbox.DefaultDeniedFileGlobs(),
		NetworkDenySubnets: sandbox.DefaultDeniedSubnets(),
	}
	// 真实存在的敏感文件（文件不存在会走 ENOENT→Allow，验证不到豁免路径）
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(envFile, []byte("SECRET=xxx"), 0644))
	ov := policyOverrides{
		File: policyFileOverrides{
			AllowedPaths: []string{envFile},
			AllowedGlobs: []string{"*.pem.example"},
		},
		Command: policyCommandOverrides{Allowed: []string{"ffmpeg"}},
		Network: policyNetworkOverrides{AllowCIDRs: []string{"10.0.0.5/32"}},
	}

	applyPolicyOverrides(&base, ov, testPolicyLogger())

	assert.Contains(t, base.AllowedFilePaths, envFile)
	assert.Contains(t, base.AllowedFileGlobs, "*.pem.example")
	assert.Contains(t, base.AllowedCommands, "ffmpeg")
	assert.Len(t, base.AllowedCommands, len(sandbox.DefaultAllowedCommands())+1,
		"默认命令白名单应保留，仅追加")
	assert.Len(t, base.NetworkAllowSubnets, 1, "默认 deny 列表不进入 allow")

	// 基线：无豁免时 .env 被硬拒；豁免生效后放行
	sbPlain, err := sandbox.NewSandbox(&sandbox.SandboxPolicy{
		AllowedDirs:     []string{dir},
		DeniedFileGlobs: sandbox.DefaultDeniedFileGlobs(),
	}, testPolicyLogger())
	require.NoError(t, err)
	assert.Equal(t, sandbox.DecisionDeny, sbPlain.CheckFile(envFile, dir).Decision)

	sb, err := sandbox.NewSandbox(&base, testPolicyLogger())
	require.NoError(t, err)
	dec := sb.CheckFile(envFile, dir)
	assert.Equal(t, sandbox.DecisionAllow, dec.Decision,
		"policy.json 豁免项应在沙箱决策中生效")
}

// TestApplyPolicyOverrides_BadCIDR_SkipsAndKeepsOthers 验证非法 CIDR 跳过不阻断，合法项保留。
func TestApplyPolicyOverrides_BadCIDR_SkipsAndKeepsOthers(t *testing.T) {
	base := sandbox.SandboxPolicy{}
	ov := policyOverrides{
		Network: policyNetworkOverrides{AllowCIDRs: []string{"not-a-cidr", "192.168.1.0/24"}},
	}

	assert.NotPanics(t, func() {
		applyPolicyOverrides(&base, ov, testPolicyLogger())
	})
	assert.Len(t, base.NetworkAllowSubnets, 1, "非法项跳过，合法项保留")
}

// TestExpandTilde 验证 ~/ 前缀展开的各形态。
func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".mindx", "USER.md"), expandTilde("~/.mindx/USER.md"))
	assert.Equal(t, home, expandTilde("~"))
	assert.Equal(t, "/abs/path", expandTilde("/abs/path"), "绝对路径原样返回")
	assert.Equal(t, "relative.md", expandTilde("relative.md"), "相对路径原样返回")
}
