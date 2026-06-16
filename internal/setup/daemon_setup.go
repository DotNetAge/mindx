package setup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

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

	home, _ := os.UserHomeDir()
	pathEnv := "/usr/local/bin:/usr/bin:/bin"
	if home != "" {
		pathEnv = filepath.Join(home, ".mindx", "bin") + ":" + pathEnv
	}

	binPath, _ := os.Executable()

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
		_ = os.Remove(plistPath)
	}

	// Record install method so start/stop/status know to use launchd
	if cfg, err := core.LoadMindxConfig(workspaceDir); err == nil {
		cfg.Daemon.Installed = true
		cfg.Daemon.InstallMethod = "launchd"
		_ = cfg.Save()
	}

	// Try to unload any existing service with the same label first
	// launchctl bootout returns exit status 5 if not loaded, which is fine
	exec.Command("launchctl", "bootout", "gui/"+fmt.Sprint(os.Getuid()), agentPlist)

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

	home, _ := os.UserHomeDir()
	binPath, _ := os.Executable()
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

	servicePath := filepath.Join(serviceDir, "mindx-daemon.service")
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
	taskName := "MindXDaemon"
	username := os.Getenv("USERNAME")
	if username == "" {
		if out, e := exec.Command("whoami").Output(); e == nil {
			username = strings.TrimSpace(string(out))
		}
	}
	if username == "" {
		username = "%USERNAME%"
	}

	// Create VBS launcher that starts mindx with a hidden window (no Cmd popup on logon).
	binPath, _ := os.Executable()
	vbsPath := filepath.Join(workspaceDir, "bin", "MindXDaemon.vbs")
	if err := os.MkdirAll(filepath.Dir(vbsPath), 0755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	vbsContent := fmt.Sprintf(`CreateObject("WScript.Shell").Run "%s daemon", 0, False`, binPath)
	if err := os.WriteFile(vbsPath, []byte(vbsContent), 0644); err != nil {
		return fmt.Errorf("write vbs launcher: %w", err)
	}

	// Resolve wscript.exe path — handles non-C: system drives (e.g. D:\Windows\System32\wscript.exe)
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = "C:\\Windows"
	}
	wscriptPath := filepath.Join(systemRoot, "System32", "wscript.exe")

	xmlContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>MindX Daemon</Description>
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
      <Command>%s</Command>
      <Arguments>%s</Arguments>
      <WorkingDirectory>%s</WorkingDirectory>
    </Exec>
  </Actions>
</Task>`, username, username, wscriptPath, vbsPath, workspaceDir)

	// Write temp XML file (schtasks /create /xml requires UTF-16LE with BOM)
	tmpXML := filepath.Join(os.TempDir(), "MindXDaemon.xml")
	if err := os.WriteFile(tmpXML, toUTF16LE(xmlContent), 0644); err != nil {
		return fmt.Errorf("write task xml: %w", err)
	}
	defer func() { _ = os.Remove(tmpXML) }()

	// Try schtasks /create first
	if err := createSchtasks(taskName, tmpXML); err == nil {
		recordInstallMethod(workspaceDir, "schtasks")
		return nil
	}

	// Fallback: PowerShell New-ScheduledTask (more reliable on some Windows configs)
	if err := setupDaemonWindowsPowerShell(vbsPath, workspaceDir, taskName); err != nil {
		return err
	}
	recordInstallMethod(workspaceDir, "schtasks")
	return nil
}

func createSchtasks(taskName, xmlPath string) error {
	cmd := exec.Command("schtasks", "/create", "/tn", taskName, "/xml", xmlPath, "/f")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks: %s", decodeWindowsOutput(out))
	}
	return nil
}

func setupDaemonWindowsPowerShell(vbsPath, workspaceDir, taskName string) error {
	psScript := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$action = New-ScheduledTaskAction -Execute 'wscript.exe' -Argument '"%s"' -WorkingDirectory '%s'
$trigger = New-ScheduledTaskTrigger -AtLogOn
$principal = New-ScheduledTaskPrincipal -UserId "$env:USERNAME" -LogonType InteractiveToken -RunLevel LeastPrivilege
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew
Register-ScheduledTask -TaskName '%s' -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force
`, vbsPath, workspaceDir, taskName)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell fallback: %s", decodeWindowsOutput(out))
	}
	return nil
}

// toUTF16LE converts a UTF-8 string to UTF-16LE with BOM, as required by schtasks /xml.
// Uses golang.org/x/text for correct surrogate pair handling (non-BMP characters).
func toUTF16LE(s string) []byte {
	enc := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

	var buf bytes.Buffer
	// Write BOM
	buf.WriteByte(0xFF)
	buf.WriteByte(0xFE)
	// Encode the string using the transformer
	writer := transform.NewWriter(&buf, enc.NewEncoder())
	_, _ = writer.Write([]byte(s))
	_ = writer.Close()
	return buf.Bytes()
}

// decodeWindowsOutput attempts to decode Windows command output.
// Tries UTF-8 first, then GBK (common on Chinese Windows), then raw bytes.
func decodeWindowsOutput(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	// Try UTF-8 first
	s := string(raw)
	if utf8Valid(s) {
		return s
	}
	// Try GBK (common on Chinese Windows systems)
	if decoded, err := decodeGBK(raw); err == nil && utf8Valid(decoded) {
		return decoded
	}
	// Fallback: raw string (may have replacement chars)
	return s
}

// utf8Valid checks if a string contains only valid UTF-8 sequences.
func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '\ufffd' {
			return false
		}
	}
	return true
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

// recordInstallMethod persists the daemon install method to config.
// Errors are intentionally ignored — this is best-effort metadata;
// the actual service registration already succeeded by this point.
func recordInstallMethod(workspaceDir string, method string) {
	cfg, err := core.LoadMindxConfig(workspaceDir)
	if err != nil {
		return
	}
	cfg.Daemon.Installed = true
	cfg.Daemon.InstallMethod = method
	_ = cfg.Save()
}
