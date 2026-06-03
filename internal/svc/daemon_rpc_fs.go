package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	if runtime.GOOS == "windows" {
		systemDrive := os.Getenv("SystemDrive")
		if systemDrive == "" {
			systemDrive = "C:"
		}
		return filepath.Join(systemDrive, "mindx")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "mindx")
}

func (d *Daemon) handleFSHome(_ context.Context, _ json.RawMessage) (any, error) {
	return map[string]string{"path": defaultFSHome()}, nil
}
