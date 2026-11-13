package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/DotNetAge/mindx/internal/core"
)

func DaemonInstalled(workspaceDir string) bool {
	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return false
	}
	return cfg.Daemon.Installed
}

func SetupDaemon(workspaceDir, binPath string) error {
	// 工作目录兜底创建：各平台服务定义里的 WorkingDirectory 都指向它，
	// 目录缺失会让服务启动即失败并被反复重启。
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return fmt.Errorf("创建工作目录失败: %w", err)
	}
	// binPath 为空时回落当前可执行文件（doctor / 向导等原位调用场景）。
	if binPath == "" {
		resolved, err := os.Executable()
		if err != nil {
			return fmt.Errorf("定位程序文件失败: %w", err)
		}
		binPath = resolved
	}

	// Detect sandboxed/containerized environments first — they have
	// their own service managers that override the OS defaults.
	if isSnap() {
		return setupDaemonSnap(workspaceDir)
	}
	if isFlatpak() {
		return setupDaemonFlatpak(workspaceDir)
	}

	switch runtime.GOOS {
	case "darwin":
		return setupDaemonMacOS(workspaceDir, binPath)
	case "linux":
		return setupDaemonLinux(workspaceDir, binPath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func setupDaemonMacOS(workspaceDir, binPath string) error {
	home, _ := os.UserHomeDir()
	pathEnv := "/usr/local/bin:/usr/bin:/bin"
	if home != "" {
		pathEnv = filepath.Join(home, ".mindx", "bin") + ":" + pathEnv
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.mindx.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>daemon</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>%s</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>WorkingDirectory</key>
    <string>%s</string>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>`, binPath, pathEnv, workspaceDir, filepath.Join(workspaceDir, "logs", "daemon.log"), filepath.Join(workspaceDir, "logs", "daemon.err.log"))

	agentPlist := filepath.Join(home, "Library", "LaunchAgents", "com.mindx.daemon.plist")

	// 变化检测收敛：plist 内容一致且服务已加载即跳过注册。
	// bootout→bootstrap 在 macOS 看来是「删除旧后台项 + 添加新项」——重复 install /
	// doctor --fix 会反复触发「App 后台活动」系统通知，且 bootout 先杀活实例造成
	// 服务闪断。仅在首次安装或配置真实变更（二进制/工作目录/日志路径变化）时注册。
	if existing, err := os.ReadFile(agentPlist); err == nil && string(existing) == plistContent && macosAgentLoaded() {
		if cfg, err := core.LoadMindxConfig(workspaceDir); err == nil {
			cfg.Daemon.Installed = true
			cfg.Daemon.InstallMethod = "launchd"
			_ = cfg.Save()
		}
		return nil
	}

	launchAgentDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentDir, 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	if err := os.WriteFile(agentPlist, []byte(plistContent), 0o644); err != nil {
		return fmt.Errorf("copy plist to LaunchAgents: %w", err)
	}

	// Record install method so start/stop/status know to use launchd
	if cfg, err := core.LoadMindxConfig(workspaceDir); err == nil {
		cfg.Daemon.Installed = true
		cfg.Daemon.InstallMethod = "launchd"
		_ = cfg.Save()
	}

	// Try to unload any existing service with the same label first
	// launchctl bootout returns exit status 5 if not loaded, which is fine
	_ = exec.Command("launchctl", "bootout", "gui/"+fmt.Sprint(os.Getuid()), agentPlist).Run()

	if out, err := exec.Command("launchctl", "bootstrap", "gui/"+fmt.Sprint(os.Getuid()), agentPlist).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w\n%s", err, string(out))
	}

	return nil
}

// macosAgentLoaded 报告 com.mindx.daemon 是否已加载进当前用户的 launchd 域
// （launchctl print 非零退出 = 未加载或 plist 已失效）。
func macosAgentLoaded() bool {
	service := "gui/" + fmt.Sprint(os.Getuid()) + "/com.mindx.daemon"
	out, err := exec.Command("launchctl", "print", service).CombinedOutput()
	return err == nil && len(out) > 0
}

func setupDaemonLinux(workspaceDir, binPath string) error {
	serviceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	home, _ := os.UserHomeDir()
	pathEnv := "/usr/local/bin:/usr/bin:/bin"
	if home != "" {
		pathEnv = filepath.Join(home, ".mindx", "bin") + ":" + pathEnv
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=MindX Daemon
After=network.target

[Service]
Type=simple
ExecStart=%s daemon
WorkingDirectory=%s
Environment=PATH=%s
Restart=on-failure
RestartSec=5
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, binPath, workspaceDir, pathEnv, filepath.Join(workspaceDir, "logs", "daemon.log"), filepath.Join(workspaceDir, "logs", "daemon.err.log"))

	servicePath := filepath.Join(serviceDir, linuxServiceName+".service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	// Record install method so start/stop/status know to use systemctl
	if cfg, err := core.LoadMindxConfig(workspaceDir); err == nil {
		cfg.Daemon.Installed = true
		cfg.Daemon.InstallMethod = "systemd"
		_ = cfg.Save()
	}

	cmds := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", linuxServiceName + ".service"},
		{"systemctl", "--user", "start", linuxServiceName + ".service"},
	}
	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
	}

	return nil
}

// ── Sandbox / package-manager environment detection ─────────────────────

// isSnap returns true when running inside a Snap sandbox.
// Snap sets the SNAP environment variable to the snap's base directory.
func isSnap() bool {
	return os.Getenv("SNAP") != ""
}

// isFlatpak returns true when running inside a Flatpak sandbox.
// Flatpak sets FLATPAK_ID to the application ID (e.g. "com.dotnetage.mindex").
func isFlatpak() bool {
	return os.Getenv("FLATPAK_ID") != ""
}

// ── Snap daemon (snapctl) ────────────────────────────────────────────────
//
// Snap packages manage services through `snapctl start/stop` rather than
// systemd or launchd directly. The snap's service definition lives in
// snap/snapcraft.yaml under the "apps" → "daemon" section with "daemon: simple".
//
// When installed via Snap, users control the daemon with:
//   snap start/stop/restart mindx.daemon
//
// mindx install inside a Snap just marks it as installed in config so that
// the CLI knows to delegate to snapctl.

func setupDaemonSnap(workspaceDir string) error {
	// Mark daemon as installed in our config so status/start/stop commands
	// know to use snapctl instead of launchd/systemd.
	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.Daemon.Installed = true
	cfg.Daemon.InstallMethod = "snapctl"
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// Try to start the snap service via snapctl (best effort — may fail if
	// the user doesn't have a connected snap interface).
	if out, err := exec.Command("snapctl", "start", "--enable", "mindx.daemon").CombinedOutput(); err != nil {
		// snapctl may not be available during build/packaging; don't fail install.
		fmt.Fprintf(os.Stderr, "  ⚠  Could not start snap service: %s\n", string(out))
		fmt.Fprintln(os.Stderr, "     Use 'snap start mindx.daemon' to start the daemon manually.")
		return nil
	}
	return nil
}

// ── Flatpak daemon (D-Bus activation) ────────────────────────────────────
//
// Flatpak sandboxes cannot register systemd user services or launchd plists.
// The daemon must be activated via D-Bus, which is defined in the Flatpak
// manifest (flatpak/build/com.dotnetage.MindX.service and .desktop file).
//
// When installed via Flatpak, the daemon starts on-demand when a D-Bus
// client calls the well-known name. There is no persistent background process
// to register from within the sandbox.

func setupDaemonFlatpak(workspaceDir string) error {
	// Mark as installed so the CLI knows we're in a flatpak context.
	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.Daemon.Installed = true
	cfg.Daemon.InstallMethod = "dbus"
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintln(os.Stderr, "  ℹ️  Flatpak installation: daemon runs via D-Bus on-demand.")
	fmt.Fprintln(os.Stderr, "     No persistent background process to register.")
	return nil
}
