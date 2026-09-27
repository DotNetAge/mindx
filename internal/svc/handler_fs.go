package svc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

type FSEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
}

func (d *Daemon) handleFSList(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FSListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	dirPath := p.Path
	if dirPath == "" {
		dirPath = defaultFSHome()
	}

	cleanPath := filepath.Clean(dirPath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			if mkdirErr := os.MkdirAll(absPath, 0755); mkdirErr != nil {
				return nil, fmt.Errorf("path does not exist and cannot create: %s: %w", cleanPath, mkdirErr)
			}
		} else {
			return nil, fmt.Errorf("access error: %w", err)
		}
	} else if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", cleanPath)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory: %w", err)
	}

	result := make([]FSEntry, 0, len(entries))
	for _, entry := range entries {
		fi, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, FSEntry{
			Name:    entry.Name(),
			Path:    filepath.Join(absPath, entry.Name()),
			Size:    fi.Size(),
			IsDir:   entry.IsDir(),
			Mode:    fi.Mode().String(),
			ModTime: fi.ModTime(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

func defaultFSHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func (d *Daemon) handleFSHome(_ context.Context, _ json.RawMessage) (any, error) {
	return map[string]string{"path": defaultFSHome()}, nil
}

// handleFSChooseDir 弹出系统级目录选择对话框（macOS NSOpenPanel / Windows
// FolderBrowserDialog / Linux zenity），供无文件系统能力的 Web 前端选取
// 工作区绝对路径。用户取消返回空 path（不视为错误）；无 GUI 可弹（纯
// CLI/headless 环境）时返回明确错误。
func (d *Daemon) handleFSChooseDir(_ context.Context, _ json.RawMessage) (any, error) {
	var (
		path string
		err  error
	)
	switch runtime.GOOS {
	case "darwin":
		path, err = chooseDirDarwin()
	case "windows":
		path, err = chooseDirWindows()
	case "linux":
		path, err = chooseDirLinux()
	default:
		return nil, fmt.Errorf("目录选择对话框不支持当前平台: %s", runtime.GOOS)
	}
	// 调用留痕：方法缺失（旧二进制）与系统层失败在日志侧可区分，便于前端报错溯源
	d.logger.Info("fs.choose_dir: finished", "path", path, "err", err)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return map[string]string{"path": ""}, nil
	}
	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		return nil, fmt.Errorf("invalid path: %w", absErr)
	}
	return map[string]string{"path": abs}, nil
}

// chooseDirDarwin 经 osascript 调原生 choose folder；用户取消（-128）返回空路径，
// 自动化授权被拒（-1743）返回明确指引（首次调用 macOS 会弹授权确认）
func chooseDirDarwin() (string, error) {
	out, err := exec.Command("osascript", "-e",
		`POSIX path of (choose folder with prompt "选择工作区目录")`).CombinedOutput()
	if err != nil {
		combined := string(out)
		switch {
		case strings.Contains(combined, "-128"):
			return "", nil // 用户取消
		case strings.Contains(combined, "-1743"):
			return "", fmt.Errorf("系统未授权 Apple 事件：请在 系统设置 → 隐私与安全性 → 自动化 中允许终端/daemon 控制 Finder，然后重试")
		}
		return "", fmt.Errorf("系统目录选择失败: %w: %s", err, strings.TrimSpace(combined))
	}
	return strings.TrimSuffix(strings.TrimSpace(string(out)), "/"), nil
}

// chooseDirWindows 经 PowerShell FolderBrowserDialog；取消无 stdout，返回空路径
func chooseDirWindows() (string, error) {
	script := `
Add-Type -AssemblyName System.Windows.Forms | Out-Null
$f = New-Object System.Windows.Forms.FolderBrowserDialog
$f.Description = '选择工作区目录'
$f.ShowNewFolderButton = $true
if ($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $f.SelectedPath }
`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return "", fmt.Errorf("系统目录选择失败: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// chooseDirLinux 经 zenity；取消（exit 1）返回空路径，未安装 zenity 返回明确错误
func chooseDirLinux() (string, error) {
	out, err := exec.Command("zenity", "--file-selection", "--directory",
		"--title", "选择工作区目录").Output()
	if err != nil {
		if exitCode(err) == 1 {
			return "", nil // 用户取消
		}
		return "", fmt.Errorf("系统目录选择失败（需要 zenity）: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// exitCode 提取 exec.ExitError 的退出码，非 ExitError 返回 -1
func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func (d *Daemon) handleFSRead(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FSReadParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("is a directory: %s", p.Path)
	}
	if info.Size() > 100*1024*1024 {
		return nil, fmt.Errorf("file too large: %s (%.1f MB)", p.Path, float64(info.Size())/(1024*1024))
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}
	return rpc.FSReadResult{Content: string(data)}, nil
}

// ── 新增：read_base64 (用于二进制文件如图片) ──

func (d *Daemon) handleFSReadBase64(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FSReadBase64Params
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("is a directory: %s", p.Path)
	}
	if info.Size() > 50*1024*1024 {
		return nil, fmt.Errorf("file too large for base64 read: %s (%.1f MB)", p.Path, float64(info.Size())/(1024*1024))
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}
	mime := mime.TypeByExtension(filepath.Ext(absPath))
	if mime == "" {
		mime = "application/octet-stream"
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return rpc.FSReadBase64Result{Content: encoded, Mime: mime}, nil
}

func (d *Daemon) handleFSWrite(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FSWriteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create parent directory: %w", err)
	}
	if err := os.WriteFile(absPath, []byte(p.Content), 0644); err != nil {
		return nil, fmt.Errorf("cannot write file: %w", err)
	}
	return map[string]string{"status": "ok"}, nil
}

// ── write_base64（二进制文件写入，如粘贴/上传的图片落盘到会话临时目录）──

func (d *Daemon) handleFSWriteBase64(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FSWriteBase64Params
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Path == "" || p.Content == "" {
		return nil, fmt.Errorf("path and content are required")
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	// base64 内容上限：解码前先校验（base64 体积约为原始数据的 4/3），
	// 防止超大 payload 拖垮 daemon。20MB base64 ≈ 15MB 原始图片，远超正常使用。
	if len(p.Content) > 20*1024*1024 {
		return nil, fmt.Errorf("content too large for base64 write: %.1f MB", float64(len(p.Content))/(1024*1024))
	}
	raw, err := base64.StdEncoding.DecodeString(p.Content)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 content: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty content")
	}
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create parent directory: %w", err)
	}
	if err := os.WriteFile(absPath, raw, 0644); err != nil {
		return nil, fmt.Errorf("cannot write file: %w", err)
	}
	return rpc.FSWriteBase64Result{Status: "ok"}, nil
}

// ── 新增：mkdir ──

func (d *Daemon) handleFSMkdir(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FSMkdirParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	return map[string]string{"status": "ok"}, nil
}

// ── 新增：rm ──

func (d *Daemon) handleFSRm(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FSRmParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path not found: %s", p.Path)
		}
		return nil, fmt.Errorf("cannot access path: %w", err)
	}
	if info.IsDir() {
		if p.Recurse {
			if err := os.RemoveAll(absPath); err != nil {
				return nil, fmt.Errorf("cannot remove directory tree: %w", err)
			}
		} else {
			entries, err := os.ReadDir(absPath)
			if err != nil {
				return nil, fmt.Errorf("cannot read directory: %w", err)
			}
			if len(entries) > 0 {
				if p.Force {
					if err := os.RemoveAll(absPath); err != nil {
						return nil, fmt.Errorf("cannot force remove directory: %w", err)
					}
				} else {
					return nil, fmt.Errorf("directory not empty: %s", p.Path)
				}
			} else {
				if err := os.Remove(absPath); err != nil {
					return nil, fmt.Errorf("cannot remove directory: %w", err)
				}
			}
		}
	} else {
		if err := os.Remove(absPath); err != nil {
			return nil, fmt.Errorf("cannot remove file: %w", err)
		}
	}
	return map[string]string{"status": "ok"}, nil
}

// ── 新增：mv ──

func (d *Daemon) handleFSMv(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FSMvParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	srcPath := filepath.Clean(p.Src)
	dstPath := filepath.Clean(p.Dst)
	absSrc, err := filepath.Abs(srcPath)
	if err != nil {
		return nil, fmt.Errorf("invalid source path: %w", err)
	}
	absDst, err := filepath.Abs(dstPath)
	if err != nil {
		return nil, fmt.Errorf("invalid target path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absDst), 0755); err != nil {
		return nil, fmt.Errorf("cannot create target parent: %w", err)
	}
	if err := os.Rename(absSrc, absDst); err != nil {
		return nil, fmt.Errorf("cannot move/rename: %w", err)
	}
	return map[string]string{"status": "ok"}, nil
}

// ── 新增：reveal ──

// handleFSReveal opens the file's parent directory in the native file manager,
// and on macOS also highlights/selects the file.
func (d *Daemon) handleFSReveal(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FSRevealParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("cannot access path: %w", err)
	}

	switch runtime.GOOS {
	case "darwin":
		// macOS: open -R reveals the file in Finder
		cmd := exec.Command("open", "-R", absPath)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to reveal in Finder: %w", err)
		}
	case "windows":
		// Windows: explorer /select highlights the file
		cmd := exec.Command("explorer", "/select,", absPath)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to reveal in Explorer: %w", err)
		}
	default:
		// Linux: open the parent directory with the default file manager
		parentDir := filepath.Dir(absPath)
		cmd := exec.Command("xdg-open", parentDir)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to open file manager: %w", err)
		}
	}

	return rpc.FSRevealResult{Status: "ok"}, nil
}

// ── 新增：stat ──

// handleFSStat returns file metadata (like os.Stat) for the given path.
// Used by the frontend to verify file existence before opening.
func (d *Daemon) handleFSStat(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.FSStatParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", cleanPath)
		}
		return nil, fmt.Errorf("cannot access path: %w", err)
	}
	return rpc.FSStatResult{
		Name:    info.Name(),
		Path:    absPath,
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime().Format(time.RFC3339),
	}, nil
}
