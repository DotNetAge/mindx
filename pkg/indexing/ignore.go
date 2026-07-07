package indexing

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// defaultIgnorePatterns lists paths excluded by default from project indexing.
var defaultIgnorePatterns = []string{
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
	// temporary / log files — no meaningful text
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

// ignoreRules evaluates exclusion rules for project file scanning.
// Not exported — loaded internally by Indexer at initialization time.
type ignoreRules struct {
	patterns []string
}

// loadIgnoreRules loads .mindxignore from dir. If the file doesn't exist,
// only default rules are used. Patterns follow .gitignore conventions:
//   - Lines starting with # are comments
//   - Trailing / indicates a directory match
//   - Leading / anchors to project root
func loadIgnoreRules(projectDir string) *ignoreRules {
	r := &ignoreRules{}
	r.patterns = append(r.patterns, defaultIgnorePatterns...)

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

// isIgnored returns true if the relative path should be excluded from indexing.
// path must be a relative path under the project root.
func (r *ignoreRules) isIgnored(relPath string) bool {
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
