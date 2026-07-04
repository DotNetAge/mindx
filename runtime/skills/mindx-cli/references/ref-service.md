# Service Lifecycle & Installation

Commands for installing, upgrading, running, and maintaining the MindX daemon service.
Most of these work offline (no daemon required).

## Install & Setup

| Task | Command | Notes |
|------|---------|-------|
| Fresh install | `mindx install` | Copies binary, configures PATH, registers system service |
| Install without daemon | `mindx install --no-daemon` | For containers or servers with custom process management |
| Skip PATH setup | `mindx install --no-path` | When PATH is already configured |
| No desktop shortcut | `mindx install --no-shortcut` | Headless environments |
| Custom install dir | `mindx install --dir /opt/mindx` | Non-default location |
| Force binary copy | `mindx install --force-copy` | Copy binary even from a package-manager-managed location |
| **Full uninstall** | `mindx uninstall` | Stops daemon, removes service, cleans PATH, deletes binary, removes shortcut |
| Uninstall (keep binary) | `mindx uninstall --keep-binary` | Remove integrations only, keep binary in place |
| Skip daemon cleanup | `mindx uninstall --no-daemon` | Don't touch daemon service during uninstall |
| Skip PATH cleanup | `mindx uninstall --no-path` | Leave PATH unchanged |
| Skip shortcut removal | `mindx uninstall --no-shortcut` | Leave desktop shortcut in place |
| Check version | `mindx version` | Shows build info, Go runtime, platform |
| Check for updates | `mindx upgrade --check` | Dry run — does not install |
| Upgrade to latest | `mindx upgrade` | Downloads and installs from GitHub |
| Health diagnosis | `mindx doctor` | Checks config, paths, permissions, connectivity |
| Auto-fix issues | `mindx doctor -f` | Attempts to fix detected problems |
| Open WebUI | `mindx web` | Opens browser at default port :1313; requires daemon to serve the UI |
| WebUI on custom port | `mindx web -p :8080` | Override default port |

## macOS App Bundle

| Task | Command | Notes |
|------|---------|-------|
| Create .app bundle | `mindx app create` | Generates .app with embedded icon in /Applications |
| Custom output path | `mindx app create -o ~/Desktop` | Override destination |
| Export icon | `mindx app icon ./icon.png` | Extracts the embedded app icon |

## Shell Completion

Generate tab-completion scripts for popular shells.

| Task | Command | Notes |
|------|---------|-------|
| Bash completion | `mindx completion bash` | Output to `/etc/bash_completion.d/mindx` or `$(brew --prefix)/etc/bash_completion.d/mindx` |
| Zsh completion | `mindx completion zsh` | Output to `${fpath[1]}/_mindx` or `$(brew --prefix)/share/zsh/site-functions/_mindx` |
| Fish completion | `mindx completion fish` | Output to `~/.config/fish/completions/mindx.fish` |
| PowerShell completion | `mindx completion powershell` | Pipe to profile |
| Disable descriptions | `mindx completion bash --no-descriptions` | Omit command descriptions from generated script |

## Daemon Service Management

| Task | Command | Notes |
|------|---------|-------|
| Start daemon | `mindx start` | Launches via system service manager (launchctl/systemd/schtasks) |
| Stop daemon | `mindx stop` | Graceful shutdown |
| Restart daemon | `mindx restart` | After config change or upgrade |
| Check status | `mindx status` | Shows binary path, config, daemon state, platform info |
| Reload agents | `mindx reload agents` | Hot-reload agent configs without full restart |
| Reload skills | `mindx reload skills` | Hot-reload skill configs without full restart |

### Direct Daemon Run (Development)

| Task | Command | Notes |
|------|---------|-------|
| Run daemon directly | `mindx daemon` | Foreground process — for dev/containers only |
| Custom WebSocket port | `mindx daemon -p :1314` | Default is :1314 |
| Custom WS path | `mindx daemon --path /ws` | Default WebSocket endpoint path |
| Show daemon version | `mindx daemon version` | Server-side version info |
| Show daemon version (JSON) | `mindx daemon version --json` | Machine-readable output |
| Check for daemon update | `mindx daemon check-update` | Server self-update check |
| Apply daemon update | `mindx daemon apply-update` | Hot-reload new binary |
| Restart from within | `mindx daemon restart` | In-process restart |
| Show daemon config | `mindx daemon config` | Displays active configuration |
| Show daemon config (JSON) | `mindx daemon config --json` | Machine-readable output |

> **Important**: Use `mindx start/stop/restart` for production. Use `mindx daemon`
> only for development or containerized environments where you manage the process yourself.

## Logs

| Task | Command | Notes |
|------|---------|-------|
| View recent logs | `mindx logs -n 50` | Last 50 lines of all log files |
| Tail logs live | `mindx logs -f` | Follow mode (like tail -f) |
| All log files checked: | `daemon.log`, `daemon.err`, `mindx.log` | — |

### Daemon Log API (requires daemon)

| Task | Command | Notes |
|------|---------|-------|
| Paginated read (newest first) | `mindx log read --limit 30` | Reverse chronological |
| Read error stream only | `mindx log read --limit 30 --stream error` | Filter by stream |
| Read from offset | `mindx log read --offset 100 --limit 30` | For pagination |
| Stream mode via API | `mindx log read --stream main --limit 10` | Live tail through daemon |
| Clear all logs | `mindx log clear --confirm` | **Destructive** — boolean flag, required to clear |
| Count log entries | `mindx log count` | Per-stream breakdown |

## Common Workflows

### First-time Setup
```bash
mindx install
mindx doctor
mindx status
mindx web
```

### After Upgrade
```bash
mindx upgrade --check
mindx upgrade
mindx restart
mindx doctor
```

### Troubleshooting
```bash
mindx version
mindx status
mindx doctor -f
mindx logs -n 50
mindx log read --limit 30 --stream error
```

### Complete Removal
```bash
mindx uninstall              # Remove all system integrations
rm -rf ~/.mindx             # Optionally remove all data (logs, sessions, config)
```

### Development Mode
```bash
mindx daemon --port :1314
# In another terminal:
mindx logs -f
```
