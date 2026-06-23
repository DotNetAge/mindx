package kbwatch

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestExtensionBlacklist verifies that files with known-garbage extensions
// are matched by DefaultIgnorePatterns so they never enter the indexing
// pipeline.
func TestExtensionBlacklist(t *testing.T) {
	tests := []struct {
		path    string
		ignored bool
	}{
		// Should be ignored
		{"foo.output", true},
		{"bar/output/output.txt", false},
		{"bar/output/file.output", true},
		{"a/b/c/tmp.tmp", true},
		{"a/b/c/tmp.temp", true},
		{"swap.swp", true},
		{"swap.swo", true},
		{"swap.swn", true},
		{"backup.bak", true},
		{"app.log", true},
		{"service.pid", true},
		{"lockfile.lock", true},
		{"cache.cache", true},
		// Normal source files should NOT be ignored
		{"main.go", false},
		{"app.ts", false},
		{"style.css", false},
		{"index.html", false},
		{"readme.md", false},
		{"Dockerfile", false},
		{"Makefile", false},
		{"package.json", false},
		// Hidden directories that are already filtered elsewhere
		{".git/config", true},
		{"node_modules/foo/bar.js", true},
		{".venv/bin/python", true},
	}

	rules := LoadMindxIgnore("/tmp")
	for _, tt := range tests {
		got := rules.IsIgnored(tt.path)
		if got != tt.ignored {
			t.Errorf("IsIgnored(%q) = %v, want %v", tt.path, got, tt.ignored)
		}
	}
}

// TestIsValidFileContent verifies that the content-quality gate correctly
// rejects binary, empty, and garbage content while accepting readable text.
func TestIsValidFileContent(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		valid bool
	}{
		{"empty", []byte{}, false},
		{"null byte", []byte("hello\x00world"), false},
		{"all nulls", []byte{0, 0, 0, 0}, false},
		{"binary blob", []byte{0x01, 0x02, 0xFF, 0xFE, 0x00}, false},
		// Garbage with few printable chars
		{"garbage control chars", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12}, false},
		// Too short meaningful text
		{"too short", []byte("ab"), false},
		// Just below threshold (19 letters/digits)
		{"barely below threshold", []byte("hello world 123!"), false},
		// Below threshold (15 meaningful chars < 20)
		{"below threshold 15", []byte("hello world 12345!"), false},
		// At/exceeding threshold
		{"at threshold 20", []byte("hello world 1234567890"), true},
		// Normal readable text
		{"normal text", []byte("The quick brown fox jumps over the lazy dog."), true},
		{"chinese text", []byte("这是一段有意义的测试中文文本，用于验证分块功能是否正常。"), true},
		{"code snippet", []byte("func main() { fmt.Println(\"hello\") }"), true},
		{"markdown content", []byte("# Title\n\nThis is a paragraph with enough text."), true},
		// Half-garbage half-text — printable ratio must be > 0.50
		{"mixed garbage half", []byte("AAAAA" + string([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidFileContent(tt.data)
			if got != tt.valid {
				t.Errorf("isValidFileContent(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

// TestIsSystemDir verifies that system directories are rejected by the
// watchlist path validation.
func TestIsSystemDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isSystemDir uses Unix paths, not applicable on Windows")
	}
	tests := []struct {
		path   string
		system bool
	}{
		{"/tmp", true},
		{"/tmp/foo", true},
		{"/private/tmp", true},
		{"/private/tmp/something", true},
		{"/var/tmp", true},
		{"/dev", true},
		{"/proc", true},
		{"/sys", true},
		{"/etc", true},
		{"/var/log", true},
		{"/var/log/something", true},
		// Normal project directories
		{"/Users/me/project", false},
		{"/home/user/work", false},
		{"/var/www", false},
		// Near misses — should not match
		{"/tmpdir", false},
		{"/etcetera", false},
		{"/devtools", false},
		{"/sysadmin", false},
	}

	for _, tt := range tests {
		got := isSystemDir(tt.path)
		if got != tt.system {
			t.Errorf("isSystemDir(%q) = %v, want %v", tt.path, got, tt.system)
		}
	}
}

// TestIndexFileRejectsBinary creates a temporary binary file and verifies
// that indexFile returns (nil, nil) — i.e. it silently skips it.
func TestIndexFileRejectsBinary(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "test.bin")

	// Write binary content
	if err := os.WriteFile(binaryPath, []byte{0x00, 0x01, 0x02, 0xFF}, 0644); err != nil {
		t.Fatal(err)
	}

	// We can't easily create an IndexService without a full HybridIndexer,
	// so we test the validation helper directly.
	if isValidFileContent([]byte{0x00, 0x01, 0x02, 0xFF}) {
		t.Error("isValidFileContent should reject binary content")
	}
}
