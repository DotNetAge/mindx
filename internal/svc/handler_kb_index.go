package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/mindx/pkg/indexing"
)

// ---------------------------------------------------------------------------
// kb.index.list — 获取项目目录的索引清单
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBIndexList(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	absDir, err := filepath.Abs(p.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project dir: %w", err)
	}

	pi, err := d.getIndexer(absDir)
	if err != nil {
		return nil, err
	}
	files, err := pi.ListAllFiles(ctx, absDir)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	status := pi.Status(ctx)

	// Build file list arrays in the format the dialog expects
	type fileEntry struct {
		Path         string  `json:"path"`
		State        string  `json:"state"`
		Error        string  `json:"error,omitempty"`
		Mtime        int64   `json:"mtime,omitempty"`
		Size         int64   `json:"size,omitempty"`
		InputTokens  int     `json:"input_tokens,omitempty"`
		OutputTokens int     `json:"output_tokens,omitempty"`
		CacheTokens  int     `json:"cache_tokens,omitempty"`
		Cost         float64 `json:"cost,omitempty"`
		Chunks       int     `json:"chunks,omitempty"`
		Nodes        int     `json:"nodes,omitempty"`
		ElapsedMs    int64   `json:"elapsed_ms,omitempty"`
		UpdatedAt    int64   `json:"updated_at,omitempty"`
	}

	allFiles := make([]fileEntry, 0, len(files))
	pendingFiles := make([]string, 0)
	failedFiles := make([]fileEntry, 0)
	completedFiles := make([]fileEntry, 0)
	currentFile := ""

	for _, meta := range files {
		stateStr := fileStateToString(meta.State)
		fe := fileEntry{
			Path:         meta.Path,
			State:        stateStr,
			Error:        meta.Error,
			Mtime:        meta.Mtime,
			Size:         meta.Size,
			InputTokens:  meta.InputTokens,
			OutputTokens: meta.OutputTokens,
			CacheTokens:  meta.CacheTokens,
			Cost:         meta.Cost,
			Chunks:       meta.Chunks,
			Nodes:        meta.Nodes,
			ElapsedMs:    meta.ElapsedMs,
			UpdatedAt:    meta.UpdatedAt,
		}
		allFiles = append(allFiles, fe)

		switch meta.State {
		case indexing.FilePending, indexing.FileEnqueued:
			pendingFiles = append(pendingFiles, meta.Path)
		case indexing.FileProcessing:
			currentFile = meta.Path
		case indexing.FileIndexed:
			completedFiles = append(completedFiles, fe)
		case indexing.FileFailed:
			failedFiles = append(failedFiles, fe)
		}
	}

	return map[string]any{
		"project_dir":     absDir,
		"files":           allFiles,
		"processing":      status.Processing,
		"total_files":     len(files),
		"indexed_files":   status.DoneCount,
		"pending_files":   pendingFiles,
		"failed_files":    failedFiles,
		"completed_files": completedFiles,
		"current_file":    currentFile,
	}, nil
}

// fileStateToString converts a FileState enum to a string for JSON serialization.
func fileStateToString(s indexing.FileState) string {
	switch s {
	case indexing.FilePending:
		return "pending"
	case indexing.FileEnqueued:
		return "enqueued"
	case indexing.FileProcessing:
		return "processing"
	case indexing.FileIndexed:
		return "done"
	case indexing.FileFailed:
		return "error"
	default:
		return "unknown"
	}
}

// expandDirFiles — 如果 path 是目录则展开为目录下所有文件列表
func expandDirFiles(absPath string) []string {
	fi, err := os.Stat(absPath)
	if err != nil {
		return nil
	}
	if !fi.IsDir() {
		return []string{absPath}
	}

	var files []string
	filepath.Walk(absPath, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, walkPath)
		return nil
	})
	return files
}

// ---------------------------------------------------------------------------
// kb.index.enqueue — 将 pending 文件入队等待索引（可选指定文件列表，不传则全部入队）
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBIndexEnqueue(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string   `json:"project_dir"`
		Files      []string `json:"files,omitempty"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	absDir, _ := filepath.Abs(p.ProjectDir)

	pi, err := d.getIndexer(absDir)
	if err != nil {
		return nil, err
	}

	var enqueued int
	if len(p.Files) == 0 {
		enqueued = pi.Enqueue(ctx)
		d.logger.Info("kb.index.enqueue all", "projectDir", absDir, "enqueued", enqueued)
	} else {
		enqueued = pi.Enqueue(ctx, p.Files...)
		d.logger.Info("kb.index.enqueue files", "projectDir", absDir, "files", p.Files, "enqueued", enqueued)
	}

	return map[string]any{"status": "enqueued", "enqueued": enqueued}, nil
}

// ---------------------------------------------------------------------------
// kb.index.add — 将文件或目录加入索引清单
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBIndexAdd(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string   `json:"project_dir"`
		Files      []string `json:"files"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}
	if len(p.Files) == 0 {
		return nil, fmt.Errorf("files is required")
	}

	absDir, err := filepath.Abs(p.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project dir: %w", err)
	}

	// Paths from frontend are already absolute; expand directories
	var absFiles []string
	for _, f := range p.Files {
		collected := expandDirFiles(f)
		absFiles = append(absFiles, collected...)
	}

	if len(absFiles) == 0 {
		return map[string]any{"status": "no_valid_files", "added": 0}, nil
	}

	// Add to manifest
	pi, err := d.getIndexer(absDir)
	if err != nil {
		return nil, err
	}
	added := pi.Add(ctx, absFiles...)

	status := pi.Status(ctx)
	return map[string]any{
		"status":        "added",
		"added":         added,
		"total_pending": status.PendingCount + status.Enqueued,
	}, nil
}

// ---------------------------------------------------------------------------
// kb.index.remove — 从索引清单中移除文件或目录
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBIndexRemove(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string   `json:"project_dir"`
		Files      []string `json:"files"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}
	if len(p.Files) == 0 {
		return nil, fmt.Errorf("files is required")
	}

	absDir, _ := filepath.Abs(p.ProjectDir)

	// Paths from frontend are already absolute; expand directories and track dirs
	var absFiles []string
	var dirPaths []string
	for _, f := range p.Files {
		collected := expandDirFiles(f)
		absFiles = append(absFiles, collected...)

		if fi, err := os.Stat(f); err == nil && fi.IsDir() {
			dirPaths = append(dirPaths, f)
		}
	}

	pi, err := d.getIndexer(absDir)
	if err != nil {
		return nil, err
	}
	removed := 0
	for _, absPath := range absFiles {
		if err := pi.RemoveFile(ctx, absPath); err != nil {
			d.logger.Warn("failed to remove file from index", "path", absPath, "error", err)
			continue
		}
		removed++
	}
	// Also clean up directory entries for explicitly removed directories
	for _, absPath := range dirPaths {
		if err := pi.RemoveFile(ctx, absPath); err != nil {
			d.logger.Warn("failed to remove directory from index", "path", absPath, "error", err)
			continue
		}
		removed++
	}
	stats, _ := pi.Count(ctx, "")
	var remaining int
	for _, c := range stats {
		remaining += c
	}
	d.logger.Info("kb.index.remove result", "removed", removed, "remaining", remaining)

	return map[string]any{"status": "removed", "removed": removed}, nil
}
