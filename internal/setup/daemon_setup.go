package setup

import (
	"bytes"
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

	// Use XML-based task definition for reliable quoting and working directory support.
	// CLI schtasks /create mangles quotes in /tr when paths contain spaces.
	xmlContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>MindX Daemon — auto-start on user logon</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>%s</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal>
      <UserId>%s</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
  </Settings>
  <Actions>
    <Exec>
      <Command>"%s"</Command>
      <Arguments>start</Arguments>
      <WorkingDirectory>%s</WorkingDirectory>
    </Exec>
  </Actions>
</Task>`, os.Getenv("USERNAME"), os.Getenv("USERNAME"), exePath, workspaceDir)

	// Write temp XML file (schtasks /create /xml requires UTF-16LE with BOM)
	tmpXML := filepath.Join(os.TempDir(), "MindXDaemon.xml")
	if err := os.WriteFile(tmpXML, toUTF16LE(xmlContent), 0644); err != nil {
		return fmt.Errorf("write task xml: %w", err)
	}
	defer os.Remove(tmpXML)

	// Create or force-update the scheduled task from XML
	cmd := exec.Command("schtasks", "/create", "/tn", taskName, "/xml", tmpXML, "/f")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks create: %w\n%s", err, decodeWindowsOutput(out))
	}

	return nil
}

// toUTF16LE converts a UTF-8 string to UTF-16LE with BOM, as required by schtasks /xml.
func toUTF16LE(s string) []byte {
	var buf bytes.Buffer
	buf.WriteByte(0xFF)
	buf.WriteByte(0xFE)
	for _, r := range s {
		buf.Write([]byte{byte(r), byte(r >> 8)})
	}
	return buf.Bytes()
}

// decodeWindowsOutput attempts to decode Windows command output.
// On Chinese Windows systems, stderr is often GBK-encoded; this fallback
// prevents garbled ◇◇◇? characters in error messages.
func decodeWindowsOutput(raw []byte) string {
	if decoded, err := decodeGBK(raw); err == nil {
		return decoded
	}
	return string(raw)
}

// decodeGBK tries to decode bytes as GBK/GB2312 encoding.
func decodeGBK(b []byte) (string, error) {
	var runes []rune
	i := 0
	for i < len(b) {
		if b[i] >= 0x81 && b[i] <= 0xFE && i+1 < len(b) {
			next := b[i+1]
			if (next >= 0x40 && next <= 0x7E) || (next >= 0x80 && next <= 0xFE) {
				runes = append(runes, rune(uint16(b[i])<<8|uint16(next)))
				i += 2
				continue
			}
		}
		runes = append(runes, rune(b[i]))
		i++
	}
	if len(runes) == 0 {
		return "", fmt.Errorf("empty output")
	}
	result := string(runes)
	for _, r := range result {
		if r >= 0x20 && r != '\ufffd' {
			return result, nil
		}
	}
	return "", fmt.Errorf("invalid GBK")
}
