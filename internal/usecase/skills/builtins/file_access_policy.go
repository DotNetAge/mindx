package builtins

import (
	"os"
	"path/filepath"
	"strings"

	"mindx/internal/config"
)

type fileAccessPolicy struct {
	enabled        bool
	workspace      string
	allowedEntries []allowedPathEntry
}

type allowedPathEntry struct {
	path  string
	isDir bool
}

func loadFileAccessPolicy(workspace string) (fileAccessPolicy, error) {
	policy := fileAccessPolicy{
		enabled:        true,
		workspace:      workspace,
		allowedEntries: nil,
	}

	cfg, err := config.LoadServerConfig()
	if err != nil {
		// Fail closed when config cannot be loaded: only workspace remains accessible.
		return policy, nil
	}

	policy.enabled = cfg.FileAccess.Enabled
	if !policy.enabled {
		return policy, nil
	}

	normalizedAllowed := make([]allowedPathEntry, 0, len(cfg.FileAccess.AllowedPaths))
	for _, p := range cfg.FileAccess.AllowedPaths {
		entry, ok := normalizeAllowedPathEntry(workspace, p)
		if !ok {
			continue
		}
		normalizedAllowed = append(normalizedAllowed, entry)
	}
	policy.allowedEntries = normalizedAllowed
	return policy, nil
}

func normalizeAllowedPathEntry(workspace, rawPath string) (allowedPathEntry, bool) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return allowedPathEntry{}, false
	}

	// Accept both slash styles from user config input regardless of current runtime OS.
	isDirBySuffix := strings.HasSuffix(rawPath, "/**") ||
		strings.HasSuffix(rawPath, "/") ||
		strings.HasSuffix(rawPath, "\\")
	normalized := strings.TrimSuffix(rawPath, "/**")
	if normalized == "" {
		return allowedPathEntry{}, false
	}

	if !filepath.IsAbs(normalized) {
		normalized = filepath.Join(workspace, normalized)
	}
	normalized = filepath.Clean(normalized)
	absPath, absErr := filepath.Abs(normalized)
	if absErr != nil {
		return allowedPathEntry{}, false
	}

	entry := allowedPathEntry{
		path:  absPath,
		isDir: isDirBySuffix,
	}
	if info, statErr := os.Stat(absPath); statErr == nil && info.IsDir() {
		entry.isDir = true
	}

	return entry, true
}

func (p fileAccessPolicy) isAllowed(targetPath string) bool {
	if !p.enabled {
		return true
	}

	cleanTarget := filepath.Clean(targetPath)
	if p.workspace != "" && isPathWithinWorkspace(p.workspace, cleanTarget) {
		return true
	}

	for _, allowed := range p.allowedEntries {
		if matchesAllowedPath(allowed, cleanTarget) {
			return true
		}
	}

	return false
}

func matchesAllowedPath(allowed allowedPathEntry, targetPath string) bool {
	cleanAllowed := filepath.Clean(allowed.path)
	cleanTarget := filepath.Clean(targetPath)

	if cleanTarget == cleanAllowed {
		return true
	}

	if allowed.isDir {
		return strings.HasPrefix(cleanTarget, cleanAllowed+string(filepath.Separator))
	}

	return false
}
