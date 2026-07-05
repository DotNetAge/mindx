package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// kb.manifest.get — 获取项目目录的索引清单
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBManifestGet(_ context.Context, params json.RawMessage) (any, error) {
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

	m := d.manifestStore.LoadOrCreate(absDir)

	total, _, _, done, _ := m.Stats()

	// Build file list arrays in the format the dialog expects
	type fileEntry struct {
		Path         string  `json:"path"`
		State        string  `json:"state"`
		Error        string  `json:"error,omitempty"`
		InputTokens  int     `json:"input_tokens,omitempty"`
		OutputTokens int     `json:"output_tokens,omitempty"`
		CacheTokens  int     `json:"cache_tokens,omitempty"`
		Cost         float64 `json:"cost,omitempty"`
		Chunks       int     `json:"chunks,omitempty"`
		Nodes        int     `json:"nodes,omitempty"`
		ElapsedMs    int64   `json:"elapsed_ms,omitempty"`
		UpdatedAt    int64   `json:"updated_at,omitempty"`
	}

	allFiles := make([]fileEntry, 0, len(m.Files))
	pendingFiles := make([]string, 0)
	failedFiles := make([]fileEntry, 0)
	completedFiles := make([]fileEntry, 0)
	currentFile := ""

	for _, rec := range m.Files {
		fe := fileEntry{
			Path:         rec.Path,
			State:        rec.State,
			Error:        rec.Error,
			InputTokens:  rec.InputTokens,
			OutputTokens: rec.OutputTokens,
			CacheTokens:  rec.CacheTokens,
			Cost:         rec.Cost,
			Chunks:       rec.Chunks,
			Nodes:        rec.Nodes,
			ElapsedMs:    rec.ElapsedMs,
			UpdatedAt:    rec.UpdatedAt,
		}
		allFiles = append(allFiles, fe)

		switch rec.State {
		case "pending":
			pendingFiles = append(pendingFiles, rec.Path)
		case "processing":
			currentFile = rec.Path
		case "done":
			completedFiles = append(completedFiles, fe)
		case "error":
			failedFiles = append(failedFiles, fe)
		}
	}

	result := map[string]any{
		"project_dir":     absDir,
		"files":           allFiles,
		"processing":      m.Processing,
		"total_files":     total,
		"indexed_files":   done,
		"pending_files":   pendingFiles,
		"failed_files":    failedFiles,
		"completed_files": completedFiles,
		"current_file":    currentFile,
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// helper: collectFilesRecursive — 如果 path 是目录则递归收集所有文件
// ---------------------------------------------------------------------------

func collectFilesRecursive(absRoot, absPath string) []string {
	fi, err := os.Stat(absPath)
	if err != nil || !fi.IsDir() {
		// 单个文件，直接返回 relative path
		rel, err := filepath.Rel(absRoot, absPath)
		if err != nil {
			return nil
		}
		return []string{rel}
	}

	// 目录：递归遍历
	var files []string
	filepath.Walk(absPath, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(absRoot, walkPath)
		if rerr != nil {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	return files
}

// ---------------------------------------------------------------------------
// kb.manifest.add — 将文件或目录加入索引清单
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBManifestAdd(_ context.Context, params json.RawMessage) (any, error) {
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

	// Compute relative paths (supports directories — recursively expanded)
	var relFiles []string
	for _, f := range p.Files {
		absPath := filepath.Join(absDir, f)
		collected := collectFilesRecursive(absDir, absPath)
		relFiles = append(relFiles, collected...)
	}

	if len(relFiles) == 0 {
		return map[string]any{"status": "no_valid_files", "added": 0}, nil
	}

	// Add to manifest
	m := d.manifestStore.LoadOrCreate(absDir)
	added := m.AddFiles(relFiles)
	_ = d.manifestStore.Save(absDir)

	// Wake the FIFO worker so it picks up new files (if processing is active)
	d.wakeManifestWorker()

	pending := m.PendingCount()
	return map[string]any{
		"status":        "added",
		"added":         added,
		"total_pending": pending,
	}, nil
}

// ---------------------------------------------------------------------------
// kb.manifest.remove — 从索引清单移除文件
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBManifestRemove(_ context.Context, params json.RawMessage) (any, error) {
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

	// Compute relative paths (supports directories — recursively expanded)
	var relFiles []string
	for _, f := range p.Files {
		absPath := filepath.Join(absDir, f)
		collected := collectFilesRecursive(absDir, absPath)
		relFiles = append(relFiles, collected...)
	}

	m := d.manifestStore.LoadOrCreate(absDir)
	removed := m.RemoveFiles(relFiles)
	_ = d.manifestStore.Save(absDir)

	return map[string]any{"status": "removed", "removed": removed}, nil
}

// ---------------------------------------------------------------------------
// kb.manifest.start — 开始处理索引清单
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBManifestStart(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	absDir, _ := filepath.Abs(p.ProjectDir)

	m := d.manifestStore.LoadOrCreate(absDir)
	m.Processing = true
	_ = d.manifestStore.Save(absDir)

	// Signal the FIFO worker to resume
	d.resumeManifestWorker()

	return map[string]any{"status": "started"}, nil
}

// ---------------------------------------------------------------------------
// kb.manifest.stop — 暂停处理索引清单
// ---------------------------------------------------------------------------

func (d *Daemon) handleKBManifestStop(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string `json:"project_dir"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ProjectDir == "" {
		return nil, fmt.Errorf("project_dir is required")
	}

	absDir, _ := filepath.Abs(p.ProjectDir)

	m := d.manifestStore.LoadOrCreate(absDir)
	m.Processing = false
	_ = d.manifestStore.Save(absDir)

	return map[string]any{"status": "stopped"}, nil
}
