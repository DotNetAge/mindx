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
		enabled:        false,
		workspace:      workspace,
		allowedEntries: nil,
	}

	cfg, err := config.LoadServerConfig()
	if err != nil {
		// Default to unrestricted mode for backward compatibility when config cannot be loaded.
		return policy, nil
	}

	policy.enabled = cfg.FileAccess.Enabled
	if !policy.enabled {
		return policy, nil
	}

	normalizedAllowed := make([]allowedPathEntry, 0, len(cfg.FileAccess.AllowedPaths))
	for _, p := range cfg.FileAccess.AllowedPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(workspace, p)
		}
		p = filepath.Clean(p)
		absPath, absErr := filepath.Abs(p)
		if absErr != nil {
			continue
		}
		entry := allowedPathEntry{path: absPath}
		if info, statErr := os.Stat(absPath); statErr == nil {
			entry.isDir = info.IsDir()
		}
		normalizedAllowed = append(normalizedAllowed, entry)
	}
	policy.allowedEntries = normalizedAllowed
	return policy, nil
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
