package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type fsListParams struct {
	Path string `json:"path"`
}

type FSEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
}

func (d *Daemon) handleFSList(_ context.Context, params json.RawMessage) (any, error) {
	var p fsListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	dirPath := p.Path
	if dirPath == "" {
		dirPath = defaultFSHome()
	}

	cleanPath := filepath.Clean(dirPath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			if mkdirErr := os.MkdirAll(absPath, 0755); mkdirErr != nil {
				return nil, fmt.Errorf("path does not exist and cannot create: %s: %w", cleanPath, mkdirErr)
			}
		} else {
			return nil, fmt.Errorf("access error: %w", err)
		}
	} else if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", cleanPath)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory: %w", err)
	}

	result := make([]FSEntry, 0, len(entries))
	for _, entry := range entries {
		fi, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, FSEntry{
			Name:    entry.Name(),
			Path:    filepath.Join(absPath, entry.Name()),
			Size:    fi.Size(),
			IsDir:   entry.IsDir(),
			Mode:    fi.Mode().String(),
			ModTime: fi.ModTime(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	parentPath := filepath.Dir(absPath)
	if parentPath != absPath {
		result = append([]FSEntry{{
			Name:    "..",
			Path:    parentPath,
			IsDir:   true,
			ModTime: time.Time{},
		}}, result...)
	}

	return result, nil
}

func defaultFSHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func (d *Daemon) handleFSHome(_ context.Context, _ json.RawMessage) (any, error) {
	return map[string]string{"path": defaultFSHome()}, nil
}

type fsReadParams struct {
	Path string `json:"path"`
}

type fsReadResult struct {
	Content string `json:"content"`
}

func (d *Daemon) handleFSRead(_ context.Context, params json.RawMessage) (any, error) {
	var p fsReadParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("is a directory: %s", p.Path)
	}
	if info.Size() > 100*1024*1024 {
		return nil, fmt.Errorf("file too large: %s (%.1f MB)", p.Path, float64(info.Size())/(1024*1024))
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}
	return fsReadResult{Content: string(data)}, nil
}

type fsWriteParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (d *Daemon) handleFSWrite(_ context.Context, params json.RawMessage) (any, error) {
	var p fsWriteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create parent directory: %w", err)
	}
	if err := os.WriteFile(absPath, []byte(p.Content), 0644); err != nil {
		return nil, fmt.Errorf("cannot write file: %w", err)
	}
	return map[string]string{"status": "ok"}, nil
}

// ── 新增：mkdir ──

type fsMkdirParams struct {
	Path string `json:"path"`
}

func (d *Daemon) handleFSMkdir(_ context.Context, params json.RawMessage) (any, error) {
	var p fsMkdirParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	return map[string]string{"status": "ok"}, nil
}

// ── 新增：rm ──

type fsRmParams struct {
	Path string `json:"path"`
}

func (d *Daemon) handleFSRm(_ context.Context, params json.RawMessage) (any, error) {
	var p fsRmParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	cleanPath := filepath.Clean(p.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path not found: %s", p.Path)
		}
		return nil, fmt.Errorf("cannot access path: %w", err)
	}
	if info.IsDir() {
		// Only remove empty directories to avoid accidental data loss
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return nil, fmt.Errorf("cannot read directory: %w", err)
		}
		if len(entries) > 0 {
			return nil, fmt.Errorf("directory not empty: %s", p.Path)
		}
		if err := os.Remove(absPath); err != nil {
			return nil, fmt.Errorf("cannot remove directory: %w", err)
		}
	} else {
		if err := os.Remove(absPath); err != nil {
			return nil, fmt.Errorf("cannot remove file: %w", err)
		}
	}
	return map[string]string{"status": "ok"}, nil
}

// ── 新增：mv ──

type fsMvParams struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

func (d *Daemon) handleFSMv(_ context.Context, params json.RawMessage) (any, error) {
	var p fsMvParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	srcPath := filepath.Clean(p.Source)
	dstPath := filepath.Clean(p.Target)
	absSrc, err := filepath.Abs(srcPath)
	if err != nil {
		return nil, fmt.Errorf("invalid source path: %w", err)
	}
	absDst, err := filepath.Abs(dstPath)
	if err != nil {
		return nil, fmt.Errorf("invalid target path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absDst), 0755); err != nil {
		return nil, fmt.Errorf("cannot create target parent: %w", err)
	}
	if err := os.Rename(absSrc, absDst); err != nil {
		return nil, fmt.Errorf("cannot move/rename: %w", err)
	}
	return map[string]string{"status": "ok"}, nil
}

// ── HTTP download ──

func (d *Daemon) handleFSDownload(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "missing path parameter", http.StatusBadRequest)
		return
	}
	cleanPath := filepath.Clean(filePath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	info, err := os.Stat(absPath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if info.IsDir() {
		http.Error(w, "is a directory", http.StatusBadRequest)
		return
	}
	if info.Size() > 200*1024*1024 {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, info.Name()))
	w.Header().Set("Content-Type", detectContentType(info.Name()))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	http.ServeFile(w, r, absPath)
}

func detectContentType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".mp4":
		return "video/mp4"
	case ".avi":
		return "video/x-msvideo"
	case ".mkv":
		return "video/x-matroska"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg", ".oga":
		return "audio/ogg"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	case ".rar":
		return "application/vnd.rar"
	case ".7z":
		return "application/x-7z-compressed"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".yaml", ".yml":
		return "text/yaml"
	default:
		return "application/octet-stream"
	}
}
