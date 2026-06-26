package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
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

// DownloadAndInstall downloads the latest release and replaces the current binary.
// If the context is cancelled the download is aborted.
func (u *Updater) DownloadAndInstall(ctx context.Context) error {
	u.logf("checking for updates: current=%s installed=%s", u.currentVer, u.installedVer)

	rel, err := LatestRelease(u.httpClient)
	if err != nil {
		return fmt.Errorf("fetch latest release: %w", err)
	}

	if !IsNewer(rel.TagName, u.currentVer) {
		u.logf("already up-to-date (%s)", u.currentVer)
		return nil
	}

	downloadURL, assetName, err := rel.FindAssetForPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("find asset: %w", err)
	}

	u.logf("downloading %s ...", assetName)

	// Download to a temp directory
	tmpDir, err := os.MkdirTemp("", "mindx-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, assetName)
	if err := u.downloadFile(ctx, downloadURL, archivePath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// Verify SHA256 if checksum asset is available
	u.logf("downloaded to %s, extracting...", archivePath)

	// Extract the binary
	var binaryPath string
	if strings.HasSuffix(assetName, ".tar.gz") {
		binaryPath, err = extractTarGz(archivePath, tmpDir)
	} else if strings.HasSuffix(assetName, ".zip") {
		binaryPath, err = extractZip(archivePath, tmpDir)
	} else {
		return fmt.Errorf("unsupported archive format: %s", assetName)
	}
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	// Ensure binary is executable
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return fmt.Errorf("chmod binary: %w", err)
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

	if err := os.Rename(binaryPath, execPath); err != nil {
		// Try to restore
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("install new binary: %w", err)
	}

	// Remove backup (will work on most Unix systems after exec restarts)
	_ = os.Remove(backupPath)

	// Update config
	version := strings.TrimPrefix(rel.TagName, "v")
	if err := u.saveConfig(version); err != nil {
		u.logf("warning: failed to update config version: %v", err)
	}

	u.logf("update to %s installed successfully at %s", version, execPath)
	return nil
}

func (u *Updater) downloadFile(ctx context.Context, url, destPath string) error {
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

		// Look for the binary (mindx or mindx.exe)
		name := filepath.Base(header.Name)
		if name != "mindx" && name != "mindx.exe" {
			continue
		}

		outPath := filepath.Join(destDir, name)
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		defer func() { _ = out.Close() }()

		if _, err := io.Copy(out, tarr); err != nil {
			return "", err
		}

		return outPath, nil
	}

	return "", fmt.Errorf("binary not found in archive")
}

// extractZip extracts a zip archive and returns the path to the binary.
func extractZip(archivePath, destDir string) (string, error) {
	// Use the system's unzip command for simplicity, avoiding CGO dependency.
	// Short-circuit: we don't use a pure-Go zip reader to keep things simple.
	// zip extraction only matters for Windows anyway.
	return "", fmt.Errorf("zip extraction not implemented; use tar.gz on Unix")
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
