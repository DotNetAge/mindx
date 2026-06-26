package indexing

import (
	"os"
	"path/filepath"
	"strings"
)

// systemDirPrefixes lists path prefixes that should never be added to the file
// watchlist. Watching system-level directories can result in indexing garbage
// or transient data (temp files, logs, kernel pseudo-filesystems, …).
var systemDirPrefixes = []string{
	"/tmp",
	"/private/tmp",
	"/var/tmp",
	"/dev",
	"/proc",
	"/sys",
	"/etc",
	"/var/log",
}

// isSystemDir returns true when absPath is a system directory that should not
// be watched for file indexing. The check is case-sensitive (macOS /tmp is
// the only common form); Linux paths are already lowercase.
func isSystemDir(absPath string) bool {
	clean := filepath.Clean(absPath)
	for _, prefix := range systemDirPrefixes {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return true
		}
	}
	return false
}

// watchDir adds a directory to the fsnotify watcher.
// Subdirectories are also added to capture deep file changes.
func (s *FileWatchService) watchDir(absDir string) error {
	// Resolve symlinks so that filepath.Walk sees the real directory.
	// On macOS, /tmp is a symlink to /private/tmp; without this resolution
	// Walk treats the root as a non-directory (symlink) and skips everything.
	realDir, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		// Fallback: try adding the path directly (fsnotify can handle some symlinks)
		return s.watcher.Add(absDir)
	}
	return filepath.Walk(realDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if !info.IsDir() {
			return nil
		}
		// Skip hidden and ignored directories (same logic as index_service)
		if path != realDir {
			base := info.Name()
			if strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			if DefaultIgnoredDirs[base] {
				return filepath.SkipDir
			}
			// Also check .mindxignore for this directory
			// We use a lightweight check: only load ignore rules for the root dir
		}
		return s.watcher.Add(path)
	})
}

// watchNewDir adds a newly created directory and its subdirectories to the watcher.
func (s *FileWatchService) watchNewDir(absPath string) error {
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return s.watcher.Add(absPath)
	}
	return filepath.Walk(realPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		base := info.Name()
		if strings.HasPrefix(base, ".") {
			return filepath.SkipDir
		}
		if DefaultIgnoredDirs[base] {
			return filepath.SkipDir
		}
		return s.watcher.Add(path)
	})
}

// findRootDir finds which watched root directory contains the given absolute path.
// It handles symlinks by resolving both the input path and watchlist entries.
func (s *FileWatchService) findRootDir(absPath string) string {
	// Resolve the event path so we can match against resolved watchlist entries.
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		resolvedPath = absPath
	}
	for _, entry := range s.store.List() {
		// Resolve the watchlist entry for comparison (handles /tmp → /private/tmp)
		resolvedEntry, err := filepath.EvalSymlinks(entry.Dir)
		if err != nil {
			resolvedEntry = entry.Dir
		}
		if strings.HasPrefix(resolvedPath, resolvedEntry+string(filepath.Separator)) || resolvedPath == resolvedEntry {
			return entry.Dir // return ORIGINAL dir (used as key in pending map)
		}
		// Also check unresolved in case EvalSymlinks changes things unexpectedly
		if strings.HasPrefix(absPath, entry.Dir+string(filepath.Separator)) || absPath == entry.Dir {
			return entry.Dir
		}
	}
	return ""
}

// sanitizeDirName converts a filesystem path to a safe directory name.
func SanitizeDirName(absPath string) string {
	replacer := strings.NewReplacer(
		string(filepath.Separator), "_",
		":", "_",
		"~", "_",
	)
	name := replacer.Replace(absPath)
	if len(name) > 200 {
		name = name[len(name)-200:]
	}
	return name
}

// countFilesRecursive counts non-directory entries under root using os.ReadDir.
// It is much lighter than walkProjectDir (no stat calls, no ignore rules) and
// only serves to estimate total file count for progress reporting.
func countFilesRecursive(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}
