package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	// minReadableContentChars is the minimum number of printable text characters
	// a file must contain to be considered indexable. Files below this threshold
	// (e.g., lock files, one-line logs, editor swaps) are skipped.
	minReadableContentChars = 20

	// minPrintableRatio is the minimum ratio of printable Unicode text characters
	// (letters, digits, CJK ideographs, etc.) above which file content is
	// considered non-binary / non-garbage. Files with too many control characters
	// or non-text bytes (e.g., binary blobs) are skipped.
	minPrintableRatio = 0.50
)

// walkProjectDir walks absDir and returns all non-excluded files as a map of
// relative path → os.FileInfo. Hidden / ignored directories are skipped.
func walkProjectDir(ctx context.Context, absDir string, ignore *IgnoreRules) (map[string]os.FileInfo, error) {
	currentFiles := make(map[string]os.FileInfo)
	err := filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		relPath, rErr := filepath.Rel(absDir, path)
		if rErr != nil {
			return nil
		}
		if info.IsDir() {
			if isDirIgnored(relPath, info, ignore) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignore.IsIgnored(relPath) {
			return nil
		}
		currentFiles[relPath] = info
		return nil
	})
	return currentFiles, err
}

// isValidFileContent performs lightweight content-quality checks on raw file
// content before it enters the indexing pipeline. Returns true when the
// content looks like readable text worth indexing.
//
// Checks performed:
//  1. Binary detection — null bytes indicate a non-text file.
//  2. Printable ratio — the fraction of printable Unicode categories in the
//     decoded string must meet minPrintableRatio.
//  3. Minimum meaningful characters — after stripping whitespace and symbols,
//     the remaining text must be at least minReadableContentChars long.
func isValidFileContent(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}

	// 1. Binary detection: null byte present → treat as binary
	for _, b := range raw {
		if b == 0 {
			return false
		}
	}

	// 2. Printable ratio check
	s := string(raw)
	totalRunes := 0
	printableRunes := 0

	for _, r := range s {
		totalRunes++
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsPunct(r) ||
			unicode.IsSymbol(r) || r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			printableRunes++
		}
	}

	if totalRunes == 0 {
		return false
	}

	ratio := float64(printableRunes) / float64(totalRunes)
	if ratio < minPrintableRatio {
		return false
	}

	// 3. Minimum meaningful characters (letters + digits)
	meaningful := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			meaningful++
		}
	}

	return meaningful >= minReadableContentChars
}

// isValidFileContentForScan performs a quick content check (size + null bytes)
// without reading the entire file. Returns false for binary or empty files.
func isValidFileContentForScan(baseDir, relPath string) bool {
	fullPath := filepath.Join(baseDir, relPath)
	header := make([]byte, 512)
	f, err := os.Open(fullPath)
	if err != nil {
		return false
	}
	defer f.Close()
	n, _ := f.Read(header)
	if n == 0 {
		return false
	}
	for _, b := range header[:n] {
		if b == 0 {
			return false
		}
	}
	return true
}

// isDirIgnored checks whether a directory should be skipped during walking.
// This is a package-level helper (no IndexService receiver needed).
func isDirIgnored(relPath string, info os.FileInfo, ignore *IgnoreRules) bool {
	if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
		if relPath != "." {
			return true
		}
	}
	if DefaultIgnoredDirs[info.Name()] {
		return true
	}
	if ignore.IsIgnored(relPath + "/") {
		return true
	}
	return false
}
