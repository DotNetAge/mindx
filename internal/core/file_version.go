package core

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileVersionStore struct{}

func NewFileVersionStore() *FileVersionStore {
	return &FileVersionStore{}
}

func (s *FileVersionStore) pathHash(filePath string) string {
	h := sha256.Sum256([]byte(filePath))
	return fmt.Sprintf("%x", h[:16])
}

func (s *FileVersionStore) filesDir(sessionDir string) string {
	return filepath.Join(sessionDir, "files")
}

func (s *FileVersionStore) fileDir(sessionDir, filePath string) string {
	return filepath.Join(s.filesDir(sessionDir), s.pathHash(filePath))
}

func (s *FileVersionStore) Record(sessionDir, filePath string) error {
	dir := s.fileDir(sessionDir, filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	versions, _ := filepath.Glob(filepath.Join(dir, "v*"))
	nextVer := len(versions) + 1

	dst := filepath.Join(dir, fmt.Sprintf("v%d", nextVer))
	return copyFile(filePath, dst)
}

func (s *FileVersionStore) GetInitial(sessionDir, filePath string) (string, error) {
	dir := s.fileDir(sessionDir, filePath)
	entries, err := filepath.Glob(filepath.Join(dir, "v*"))
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("no versions found")
	}
	sort.Strings(entries)
	data, err := os.ReadFile(entries[0])
	return string(data), err
}

func (s *FileVersionStore) GetLatest(sessionDir, filePath string) (string, error) {
	dir := s.fileDir(sessionDir, filePath)
	entries, err := filepath.Glob(filepath.Join(dir, "v*"))
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("no versions found")
	}
	sort.Strings(entries)
	data, err := os.ReadFile(entries[len(entries)-1])
	return string(data), err
}

func (s *FileVersionStore) ListFiles(sessionDir string) ([]string, error) {
	fd := s.filesDir(sessionDir)
	entries, err := os.ReadDir(fd)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pathFile := filepath.Join(fd, e.Name(), ".path")
		data, err := os.ReadFile(pathFile)
		if err != nil {
			continue
		}
		versions, _ := filepath.Glob(filepath.Join(fd, e.Name(), "v*"))
		if len(versions) >= 2 {
			files = append(files, string(data))
		}
	}
	sort.Strings(files)
	return files, nil
}

func (s *FileVersionStore) CountChanges(sessionDir, filePath string) (additions, removals int, err error) {
	initial, err := s.GetInitial(sessionDir, filePath)
	if err != nil {
		return 0, 0, err
	}
	latest, err := s.GetLatest(sessionDir, filePath)
	if err != nil {
		return 0, 0, err
	}
	_, add, del := computeSimpleDiff(initial, latest)
	return add, del, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return err
	}
	pathFile := filepath.Join(filepath.Dir(dst), ".path")
	_ = os.WriteFile(pathFile, []byte(src), 0644)
	return nil
}

func computeSimpleDiff(before, after string) (diff string, additions, removals int) {
	bLines := strings.Split(before, "\n")
	aLines := strings.Split(after, "\n")

	bMap := make(map[string]int)
	for _, l := range bLines {
		bMap[l]++
	}
	aMap := make(map[string]int)
	for _, l := range aLines {
		aMap[l]++
	}

	for l, n := range aMap {
		if bn := bMap[l]; bn < n {
			additions += n - bn
		}
	}
	for l, n := range bMap {
		if an := aMap[l]; an < n {
			removals += n - an
		}
	}
	return "", additions, removals
}
