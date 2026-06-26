package indexing

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// DefaultIgnorePatterns lists paths excluded by default from project indexing.
var DefaultIgnorePatterns = []string{
	".git/",
	".gitignore",
	"node_modules/",
	".venv/",
	"venv/",
	"__pycache__/",
	"*.pyc",
	"vendor/",
	"dist/",
	"build/",
	".DS_Store",
	"*.exe",
	"*.dll",
	"*.so",
	"*.dylib",
	"*.bin",
	".mindxignore",
	".mindx/",
	// 临时文件/日志 — 不可能产生有意义的文本
	"*.tmp",
	"*.temp",
	"*.swp",
	"*.swo",
	"*.swn",
	"*.bak",
	"*.log",
	"*.output",
	"*.pid",
	"*.lock",
	"*.cache",
}

// IgnoreRules evaluates exclusion rules for project file scanning.
type IgnoreRules struct {
	patterns []string
}

// LoadMindxIgnore loads .mindxignore from dir. If the file doesn't exist,
// only default rules are used. Patterns follow .gitignore conventions:
//   - Lines starting with # are comments
//   - Trailing / indicates a directory match
//   - Leading / anchors to project root
func LoadMindxIgnore(projectDir string) *IgnoreRules {
	r := &IgnoreRules{}
	r.patterns = append(r.patterns, DefaultIgnorePatterns...)

	f, err := os.Open(filepath.Join(projectDir, ".mindxignore"))
	if err != nil {
		return r
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r.patterns = append(r.patterns, line)
	}
	return r
}

// IsIgnored returns true if the relative path should be excluded from indexing.
// path must be a relative path under the project root.
func (r *IgnoreRules) IsIgnored(relPath string) bool {
	for _, pattern := range r.patterns {
		if matchIgnore(pattern, relPath) {
			return true
		}
	}
	return false
}

// matchIgnore checks a single pattern against a relative path.
// Supports:
//   - directory/ — matches directory or any path under it
//   - *.ext — matches files with extension at any depth
//   - name — matches any path component named "name"
func matchIgnore(pattern, relPath string) bool {
	// Convert to clean, forward-slash path for matching
	clean := filepath.ToSlash(filepath.Clean(relPath))
	p := filepath.ToSlash(pattern)

	// Directory match: pattern ends with /
	if strings.HasSuffix(p, "/") {
		dir := strings.TrimSuffix(p, "/")
		if clean == dir || strings.HasPrefix(clean, dir+"/") {
			return true
		}
	}

	// Wildcard like *.ext
	if strings.HasPrefix(p, "*.") {
		ext := p[1:] // e.g., ".pyc"
		if strings.HasSuffix(clean, ext) {
			return true
		}
		// Also match extension anywhere in path (e.g., dir/file.pyc)
		base := filepath.Base(clean)
		if strings.HasSuffix(base, ext[1:]) {
			return true
		}
	}

	// Exact match (any component)
	if strings.HasSuffix(clean, "/"+p) || clean == p {
		return true
	}

	// Prefix match for directories
	if strings.HasPrefix(clean, p+"/") {
		return true
	}

	return false
}
