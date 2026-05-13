package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func DaemonInstalled(workspaceDir string) bool {
	cfg, err := LoadMindxConfig(workspaceDir)
	if err != nil {
		return false
	}
	return cfg.Daemon.Installed
}

func SetupDaemon(workspaceDir string) error {
	switch runtime.GOOS {
	case "darwin":
		return setupDaemonMacOS(workspaceDir)
	case "linux":
		return setupDaemonLinux(workspaceDir)
	case "windows":
		return setupDaemonWindows(workspaceDir)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func setupDaemonMacOS(workspaceDir string) error {
	plistPath := filepath.Join(os.TempDir(), "com.mindx.daemon.plist")

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
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
        <string>start</string>
    </array>
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
</plist>`, exePath, workspaceDir, filepath.Join(workspaceDir, "logs", "daemon.log"), filepath.Join(workspaceDir, "logs", "daemon.err.log"))

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	launchAgentDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentDir, 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	agentPlist := filepath.Join(launchAgentDir, "com.mindx.daemon.plist")
	if err := os.Rename(plistPath, agentPlist); err != nil {
		// if rename fails (cross-device), copy instead
		data, _ := os.ReadFile(plistPath)
		if err := os.WriteFile(agentPlist, data, 0644); err != nil {
			return fmt.Errorf("copy plist to LaunchAgents: %w", err)
		}
		os.Remove(plistPath)
	}

	if err := exec.Command("launchctl", "bootstrap", "gui/"+fmt.Sprint(os.Getuid()), agentPlist).Run(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w", err)
	}

	return nil
}

func setupDaemonLinux(workspaceDir string) error {
	serviceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=MindX Daemon
After=network.target

[Service]
Type=simple
ExecStart=%s start
WorkingDirectory=%s
Restart=on-failure
RestartSec=5
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, exePath, workspaceDir, filepath.Join(workspaceDir, "logs", "daemon.log"), filepath.Join(workspaceDir, "logs", "daemon.err.log"))

	servicePath := filepath.Join(serviceDir, "mindx-daemon.service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	cmds := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "mindx-daemon.service"},
		{"systemctl", "--user", "start", "mindx-daemon.service"},
	}
	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
	}

	return nil
}

func setupDaemonWindows(workspaceDir string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	taskName := "MindXDaemon"
	cmd := exec.Command("schtasks", "/create", "/tn", taskName,
		"/tr", fmt.Sprintf(`"%s" start`, exePath),
		"/sc", "onlogon",
		"/rl", "limited",
		"/f")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("schtasks create: %w", err)
	}

	return nil
}
