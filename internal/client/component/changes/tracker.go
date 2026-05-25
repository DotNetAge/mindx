// Package changes tracks file modifications triggered by agent tool execution.
// It snapshots file content before tool execution and generates unified diffs
// after the tool completes, regardless of which tool modified which file.
package changes

import (
	"os"
	"sync"

	"github.com/aymanbagabas/go-udiff"

	"github.com/DotNetAge/mindx/internal/client/data"
)

// pendingCheck holds a file path and its content before modification.
type pendingCheck struct {
	path    string
	content string
}

// Tracker monitors tool execution events to detect file changes.
// On ToolExecStart, files with known paths are snapshotted.
// On ToolExecEnd, all pending snapshots are compared against current file
// state and unified diffs are generated.
type Tracker struct {
	mu      sync.Mutex
	pending []pendingCheck
	changes []data.FileChange
	workDir string
}

// NewTracker creates a file change tracker for the given project directory.
func NewTracker(workDir string) *Tracker {
	return &Tracker{
		workDir: workDir,
	}
}

// ToolExecStart is called when a tool begins execution.
// If the tool params contain a file path, it snapshots the current file content.
func (t *Tracker) ToolExecStart(params map[string]any) {
	path := extractFilePath(params)
	if path == "" {
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return // file doesn't exist yet (e.g., new file being created)
	}

	t.mu.Lock()
	t.pending = append(t.pending, pendingCheck{path: path, content: string(content)})
	t.mu.Unlock()
}

// ToolExecEnd is called when a tool completes execution.
// It processes all pending snapshots, generates diffs for changed files,
// and records the results.
func (t *Tracker) ToolExecEnd() {
	t.mu.Lock()
	batch := t.pending
	t.pending = nil
	t.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	for _, pc := range batch {
		newContent, err := os.ReadFile(pc.path)
		if err != nil {
			continue // file was deleted
		}
		newStr := string(newContent)
		if pc.content == newStr {
			continue // no change
		}

		label := pc.path
		diff := udiff.Unified("a/"+label, "b/"+label, pc.content, newStr)
		adds, dels := countLines(diff)
		relPath := relativize(pc.path, t.workDir)

		t.mu.Lock()
		// Replace existing entry for this file, keep others
		filtered := make([]data.FileChange, 0, len(t.changes))
		for _, c := range t.changes {
			if c.File != relPath {
				filtered = append(filtered, c)
			}
		}
		t.changes = append(filtered, data.FileChange{
			File:      relPath,
			Additions: adds,
			Deletions: dels,
			Diff:      diff,
		})
		t.mu.Unlock()
	}
}

// Snapshot returns a copy of the current accumulated changes.
func (t *Tracker) Snapshot() []data.FileChange {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]data.FileChange, len(t.changes))
	copy(out, t.changes)
	return out
}

// Clear resets all tracked state.
func (t *Tracker) Clear() {
	t.mu.Lock()
	t.pending = nil
	t.changes = nil
	t.mu.Unlock()
}

func extractFilePath(params map[string]any) string {
	for _, key := range []string{"path", "file_path", "filepath"} {
		if v, ok := params[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func relativize(absPath, workDir string) string {
	if workDir == "" {
		return absPath
	}
	if len(absPath) > len(workDir) && absPath[:len(workDir)] == workDir {
		rel := absPath[len(workDir):]
		if len(rel) > 0 && rel[0] == '/' {
			rel = rel[1:]
		}
		return rel
	}
	return absPath
}

func countLines(diff string) (adds, dels int) {
	lines := 0
	inHeader := true
	for i := 0; i < len(diff); i++ {
		if diff[i] == '\n' {
			lines++
			inHeader = false
			continue
		}
		if lines == 0 {
			continue
		}
		if diff[i] == '+' && !inHeader {
			if i+1 < len(diff) && diff[i+1] != '+' && diff[i+1] != '-' {
				adds++
			}
		} else if diff[i] == '-' {
			if i+1 < len(diff) && diff[i+1] != '-' {
				dels++
			}
		}
	}
	return
}
