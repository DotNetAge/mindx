package bundle

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// 单条目安全上限与包体总大小上限（技能包以文本为主，防 zip 炸弹）。
const (
	maxEntrySize = 50 << 20  // 50MB
	maxTotalSize = 200 << 20 // 200MB
)

// zipDir 将 srcDir 目录内容写入 zip 的 zipPrefix 前缀之下。
// 只打包常规文件（跳过符号链接等特殊条目），目录路径解包时按需重建。
func zipDir(zw *zip.Writer, srcDir, zipPrefix string) error {
	return filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		return zipFileEntry(zw, p, path.Join(zipPrefix, filepath.ToSlash(rel)))
	})
}

// zipFileEntry 将磁盘文件写入 zip 的指定路径。
func zipFileEntry(zw *zip.Writer, srcPath, zipPath string) error {
	w, err := zw.Create(zipPath)
	if err != nil {
		return fmt.Errorf("创建包内条目 %s 失败：%w", zipPath, err)
	}
	in, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("读取 %s 失败：%w", srcPath, err)
	}
	defer in.Close()

	if _, err := io.Copy(w, in); err != nil {
		return fmt.Errorf("写入包内条目 %s 失败：%w", zipPath, err)
	}
	return nil
}

// zipEntry 是解包时收集的单条目内容（zip 内路径 → 文件字节）。
type zipEntry struct {
	Name    string
	Content []byte
}

// readPackageEntries 读取分发包全部条目（跳过目录与空路径）。
// 条目路径做安全校验（拒绝绝对路径与 .. 穿越），并施加单条目与总量大小上限。
func readPackageEntries(pkgPath string) ([]zipEntry, error) {
	zr, err := zip.OpenReader(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("打开分发包 %s 失败：%w", pkgPath, err)
	}
	defer zr.Close()

	var (
		entries []zipEntry
		total   int64
	)
	for _, f := range zr.File {
		if f.Mode().IsDir() {
			continue
		}
		name, err := safeZipName(f.Name)
		if err != nil {
			return nil, err
		}
		if name == "" {
			continue
		}
		if f.UncompressedSize64 > maxEntrySize {
			return nil, fmt.Errorf("包内条目 %s 超过大小上限（%d 字节）", name, f.UncompressedSize64)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("读取包内条目 %s 失败：%w", name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("读取包内条目 %s 失败：%w", name, err)
		}
		total += int64(len(content))
		if total > maxTotalSize {
			return nil, fmt.Errorf("分发包解压后超过总大小上限（%d 字节）", maxTotalSize)
		}
		entries = append(entries, zipEntry{Name: name, Content: content})
	}
	return entries, nil
}

// safeZipName 校验 zip 条目路径安全并规范化为 / 分隔的相对路径。
// 拒绝绝对路径、盘符与反斜杠；Clean 后路径发生变化视为存在穿越企图（zip slip），一律拒绝。
func safeZipName(name string) (string, error) {
	name = strings.TrimSuffix(filepath.ToSlash(name), "/")
	if name == "" {
		return "", nil
	}
	if strings.HasPrefix(name, "/") || strings.Contains(name, "\\") ||
		strings.Contains(name, ":") || path.IsAbs(name) {
		return "", fmt.Errorf("包内存在非法条目路径：%q", name)
	}
	clean := path.Clean(name)
	if clean != name || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("包内存在非法或穿越条目路径：%q", name)
	}
	return clean, nil
}
