package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Updater handles checking, downloading, and installing updates.
type Updater struct {
	httpClient   *http.Client
	currentVer   string
	installedVer string
	workspaceDir string
	saveConfig   func(version string) error
	logf         func(string, ...any)
}

// NewUpdater creates a new Updater.
//
//	currentVer    - the version this binary was built with (core.Version)
//	installedVer  - the version recorded in config (MindxConfig.InstalledVersion)
//	workspaceDir  - ~/.mindx
//	saveConfig    - callback to persist the new installed version
//	logf          - optional log function
func NewUpdater(currentVer, installedVer, workspaceDir string, saveConfig func(version string) error, logf func(string, ...any)) *Updater {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Updater{
		httpClient:   &http.Client{Timeout: 5 * time.Minute},
		currentVer:   currentVer,
		installedVer: installedVer,
		workspaceDir: workspaceDir,
		saveConfig:   saveConfig,
		logf:         logf,
	}
}

// VersionInfo holds the current check result.
type VersionInfo struct {
	CurrentVersion  string `json:"current_version"`
	InstalledVer    string `json:"installed_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	LatestURL       string `json:"latest_url,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	InstallSource   string `json:"install_source,omitempty"`
	Error           string `json:"error,omitempty"`
}

// Check returns version info, optionally fetching latest from GitHub.
func (u *Updater) Check(checkRemote bool) *VersionInfo {
	info := &VersionInfo{
		CurrentVersion: u.currentVer,
		InstalledVer:   u.installedVer,
	}

	if !checkRemote {
		return info
	}

	rel, err := LatestRelease(u.httpClient)
	if err != nil {
		info.Error = err.Error()
		return info
	}

	info.LatestVersion = strings.TrimPrefix(rel.TagName, "v")
	info.LatestURL = rel.HTMLURL
	info.UpdateAvailable = IsNewer(rel.TagName, u.currentVer)
	return info
}

// Download downloads the latest release into a staging directory WITHOUT
// touching the running service or the installed binary — the daemon keeps
// serving throughout the download (which can take minutes on slow networks).
// Stopping the service is a user-authorized action and only ever happens
// right before the second-long InstallFromStage swap.
//
// Returns the staged binary path, the new version, and a cleanup func that
// removes the staging dir. If no newer release exists, stageBinary is ""
// and version is the current version.
func (u *Updater) Download(ctx context.Context) (stageBinary string, version string, cleanup func(), err error) {
	u.logf("checking for updates: current=%s installed=%s", u.currentVer, u.installedVer)

	rel, err := LatestRelease(u.httpClient)
	if err != nil {
		return "", u.currentVer, func() {}, fmt.Errorf("fetch latest release: %w", err)
	}
	if !IsNewer(rel.TagName, u.currentVer) {
		u.logf("already up-to-date (%s)", u.currentVer)
		return "", u.currentVer, func() {}, nil
	}

	downloadURL, assetName, err := rel.FindAssetForPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", "", nil, fmt.Errorf("find asset: %w", err)
	}

	u.logf("downloading %s ...", assetName)

	tmpDir, err := os.MkdirTemp("", "mindx-update-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(tmpDir) }

	archivePath := filepath.Join(tmpDir, assetName)
	if err := u.downloadFile(ctx, downloadURL, archivePath); err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("download: %w", err)
	}

	// 若发布附带了校验文件则校验完整性；未附带则提示跳过。
	if sumURL, sumName, ok := rel.FindChecksumAsset(assetName); ok {
		sumPath := filepath.Join(tmpDir, sumName)
		if err := u.downloadFile(ctx, sumURL, sumPath); err != nil {
			cleanup()
			return "", "", nil, fmt.Errorf("download checksum: %w", err)
		}
		expected, err := parseChecksumFile(sumPath, assetName)
		if err != nil {
			cleanup()
			return "", "", nil, fmt.Errorf("解析校验文件失败: %w", err)
		}
		actual, err := SHA256(archivePath)
		if err != nil {
			cleanup()
			return "", "", nil, fmt.Errorf("计算下载文件校验值失败: %w", err)
		}
		if !strings.EqualFold(actual, expected) {
			cleanup()
			return "", "", nil, fmt.Errorf("下载的更新包校验不匹配，已中止安装（文件可能下载不完整或被篡改）")
		}
		u.logf("checksum verified for %s", assetName)
	} else {
		// 没有校验文件就无法验证下载物是否被替换/截断。以前这里是「跳过校验
		// 继续装」，等于把完整性判定交给运气。发布流水线每次都产出
		// checksums.txt（release.yml 的 Generate SHA256 checksums），
		// 缺校验文件只可能是异常或人为投毒，一律中止。
		cleanup()
		return "", "", nil, fmt.Errorf("发布没有附带 %s 的校验文件，无法验证更新包完整性，已中止安装", assetName)
	}

	u.logf("downloaded to %s, extracting...", archivePath)

	var binaryPath string
	if strings.HasSuffix(assetName, ".tar.gz") {
		binaryPath, err = extractTarGz(archivePath, tmpDir)
	} else {
		cleanup()
		return "", "", nil, fmt.Errorf("unsupported archive format: %s", assetName)
	}
	if err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("extract: %w", err)
	}

	if err := os.Chmod(binaryPath, 0755); err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("chmod binary: %w", err)
	}

	return binaryPath, strings.TrimPrefix(rel.TagName, "v"), cleanup, nil
}

// ── 持久暂存区（~/.mindx/bin/.update/，写方仅 mindx upgrade） ──────────────
//
// 静默链两段式（--stage 与 --yes 是两次独立进程调用）的唯一交接面：暂存必须
// 落盘存活到应用时刻。禁止用进程存活期的临时目录充当暂存——实证事故：旧实现
// --stage 的下载物随进程退出被 cleanup 删除，--yes 再全量下载一次，一次升级
// 两次下载且零离线可用性。

func (u *Updater) stageDir() string {
	return filepath.Join(u.workspaceDir, "bin", ".update")
}

func (u *Updater) stageBinaryPath() string {
	return filepath.Join(u.stageDir(), "mindx")
}

func (u *Updater) stageVersionPath() string {
	return filepath.Join(u.stageDir(), "version.json")
}

// Staged 返回已持久化暂存的二进制路径与版本；版本标记或二进制缺失视为无暂存
// （二进制被 InstallFromStage rename 消费后，陈旧标记自动失效）。
func (u *Updater) Staged() (string, string) {
	data, err := os.ReadFile(u.stageVersionPath())
	if err != nil {
		return "", ""
	}
	var meta struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &meta); err != nil || meta.Version == "" {
		return "", ""
	}
	bin := u.stageBinaryPath()
	if st, err := os.Stat(bin); err != nil || st.IsDir() {
		return "", ""
	}
	return bin, meta.Version
}

// PersistStage 把就绪的二进制收入持久暂存区。copy 而非 rename：源常在另一
// 文件系统（os.MkdirTemp 的 /var/folders、/tmp），跨设备 rename 会失败。
func (u *Updater) PersistStage(srcBinary, version string) (string, error) {
	if err := os.MkdirAll(u.stageDir(), 0755); err != nil {
		return "", fmt.Errorf("create stage dir: %w", err)
	}
	dst := u.stageBinaryPath()
	if err := copyFile(srcBinary, dst, 0755); err != nil {
		return "", fmt.Errorf("stage binary: %w", err)
	}
	meta, _ := json.Marshal(map[string]string{"version": version})
	if err := os.WriteFile(u.stageVersionPath(), meta, 0644); err != nil {
		return "", fmt.Errorf("write stage version: %w", err)
	}
	u.logf("staged update v%s at %s", version, dst)
	return dst, nil
}

// StageFromArchive 从本地归档（MindX 壳随包的 tar.gz/zip）直接暂存，跳过
// 网络 Check/Download——归档就是目标版本的同一份产物，重新下载是浪费且把
// 升级可用性押在 GitHub 可达性上。
func (u *Updater) StageFromArchive(archivePath, version string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "mindx-stage-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	var bin string
	bin, err = extractTarGz(archivePath, tmpDir)
	if err != nil {
		return "", fmt.Errorf("extract archive: %w", err)
	}
	if err := os.Chmod(bin, 0755); err != nil {
		return "", fmt.Errorf("chmod binary: %w", err)
	}
	return u.PersistStage(bin, version)
}

// ClearStage 清理暂存版本标记（二进制已被 InstallFromStage 消费后调用）。
func (u *Updater) ClearStage() {
	_ = os.Remove(u.stageVersionPath())
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// InstallFromStage swaps the installed binary with the staged one. It is a
// second-long, atomic-ish rename and never stops or starts services — the
// caller orchestrates stop/start around it with the user's consent.
func (u *Updater) InstallFromStage(stageBinary, version string) error {
	// 版本不得倒退：目标版本低于当前版本时拒绝安装，防止把已修好的构建
	// 回滚成旧版本（本地归档 + 任意 --version 曾可做到）。
	if cmp := CompareVersions(strings.TrimPrefix(version, "v"), strings.TrimPrefix(u.currentVer, "v")); cmp < 0 {
		return fmt.Errorf("目标版本 v%s 低于当前版本 v%s，已拒绝降级安装", strings.TrimPrefix(version, "v"), strings.TrimPrefix(u.currentVer, "v"))
	}

	// 替换前再次确认暂存产物真的是可执行文件：暂存区可能来自磁盘上的旧文件
	// 或被截断的下载。
	if err := validateExecutable(stageBinary); err != nil {
		return fmt.Errorf("暂存的更新包不可用: %w", err)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	// Replace the current binary
	// On Unix we can rename over the running binary on the same filesystem.
	// But the binary might be running, so we use a two-step approach:
	//   1. Rename current binary to mindx.old
	//   2. Move new binary to the original path
	backupPath := execPath + ".old"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	if err := os.Rename(stageBinary, execPath); err != nil {
		// Try to restore
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("install new binary: %w", err)
	}

	// 刻意不删备份：新二进制还没被验证「真能起来」，此时删掉 .old 就没有回滚
	// 可用了。备份在下一次成功启动时清理（CleanupBinaryBackup）。
	u.logf("backup kept at %s until next successful start", backupPath)

	if err := u.saveConfig(version); err != nil {
		u.logf("warning: failed to update config version: %v", err)
	}

	u.logf("update to %s installed successfully at %s", version, execPath)
	return nil
}

// allowedDownloadHosts 是升级包允许的下载域（GitHub 官方资产与 API）。
var allowedDownloadHosts = map[string]bool{
	"github.com":                    true,
	"api.github.com":                true,
	"codeload.github.com":           true,
	"objects.githubusercontent.com": true,
}

// validateDownloadURL 校验下载地址：必须 https，且主机属于 GitHub 官方域。
// 地址来自 release API 响应，不校验 host 就等于允许被污染的响应把二进制
// 从任意站点拉进主机。
func validateDownloadURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("更新包地址无法解析: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("更新包地址必须是 https：%s", u.Scheme)
	}
	if !allowedDownloadHosts[strings.ToLower(u.Hostname())] {
		return fmt.Errorf("更新包地址不属于 GitHub 官方下载域（%s），已拒绝下载", u.Hostname())
	}
	return nil
}

func (u *Updater) downloadFile(ctx context.Context, url, destPath string) error {
	if err := validateDownloadURL(url); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	// Limit to ~200MB
	limited := io.LimitReader(resp.Body, 200*1024*1024)
	written, err := io.Copy(out, limited)
	if err != nil {
		return err
	}
	if written >= 200*1024*1024 {
		return fmt.Errorf("download exceeds 200MB limit")
	}

	return nil
}

// extractTarGz extracts a tar.gz archive and returns the path to the binary.
func extractTarGz(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer func() { _ = gzr.Close() }()

	tarr := tar.NewReader(gzr)
	for {
		header, err := tarr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Look for the binary
		name := filepath.Base(header.Name)
		if name != "mindx" {
			continue
		}
		// 只认真普通文件：symlink/hardlink/目录条目会被 io.Copy 写成 0 字节
		// 且不报错（探针实测），随后被 os.Rename 顶替正在运行的二进制。
		if header.Typeflag != tar.TypeReg {
			continue
		}

		outPath := filepath.Join(destDir, name)
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(out, tarr)
		closeErr := out.Close()
		if copyErr != nil {
			_ = os.Remove(outPath)
			return "", copyErr
		}
		if closeErr != nil {
			_ = os.Remove(outPath)
			return "", closeErr
		}
		// 内容校验：不是可执行魔数或体积异常偏小，一律不认这个条目
		if err := validateExecutable(outPath); err != nil {
			_ = os.Remove(outPath)
			return "", err
		}

		return outPath, nil
	}

	return "", fmt.Errorf("binary not found in archive")
}

// parseChecksumFile 从校验文件中取出目标资产的校验值。
// 支持 "<sha256>  <filename>" 行格式与纯 "<sha256>" 单行格式。
func parseChecksumFile(path, assetName string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		switch {
		case len(fields) >= 2 && fields[1] == assetName:
			return fields[0], nil
		case len(fields) == 1 && len(fields[0]) == 64:
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("校验文件中未找到 %s 对应的校验值", assetName)
}

// SHA256 computes the SHA-256 checksum of a file.
func SHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// minBinarySize 是认可的最小可执行文件体积。真实二进制在数十 MB 量级，
// 1MB 只用于挡「0 字节 / 被截断」这类明显异常，不参与精确判断。
const minBinarySize = 1 << 20

// validateExecutable 校验产物是真可执行文件：体积下限 + 可执行魔数。
// 不是「格式洁癖」——没有这一步，一个 0 字节文件就能顶替正在运行的二进制，
// 且不会有任何错误返回（见 extractTarGz 的条目类型检查）。
func validateExecutable(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("检查更新包失败: %w", err)
	}
	if st.IsDir() {
		return fmt.Errorf("更新包不是文件")
	}
	if st.Size() < minBinarySize {
		return fmt.Errorf("更新包体积异常（%d 字节），已拒绝使用", st.Size())
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开更新包失败: %w", err)
	}
	defer func() { _ = f.Close() }()

	head := make([]byte, 4)
	if _, err := io.ReadFull(f, head); err != nil {
		return fmt.Errorf("读取更新包头部失败: %w", err)
	}
	if isExecutableMagic(head) {
		return nil
	}
	return fmt.Errorf("更新包不是可执行文件（魔数不匹配）")
}

// isExecutableMagic 判断文件头是否为 ELF / Mach-O 魔数。
func isExecutableMagic(head []byte) bool {
	if len(head) < 4 {
		return false
	}
	// ELF: 0x7F 'E' 'L' 'F'
	if head[0] == 0x7f && head[1] == 'E' && head[2] == 'L' && head[3] == 'F' {
		return true
	}
	be := uint32(head[0])<<24 | uint32(head[1])<<16 | uint32(head[2])<<8 | uint32(head[3])
	le := uint32(head[3])<<24 | uint32(head[2])<<16 | uint32(head[1])<<8 | uint32(head[0])
	switch {
	case be == 0xfeedface, be == 0xfeedfacf, // Mach-O 32/64 大端
		le == 0xfeedface, le == 0xfeedfacf, // Mach-O 32/64 小端
		be == 0xcafebabe, be == 0xbebafeca, // fat binary
		be == 0xcafebabf, be == 0xbfbafeca:
		return true
	}
	return false
}

// CleanupBinaryBackup 删除上一次升级留下的 <exe>.old 备份。
// 调用时机：新二进制已完整启动（能走到 daemon 初始化完成）——这时才证明
// 替换成功，备份的使命结束。启动失败时备份仍在，可手工回滚。
func CleanupBinaryBackup() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.Remove(exe + ".old")
}
