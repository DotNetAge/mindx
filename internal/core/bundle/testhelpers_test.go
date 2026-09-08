package bundle

import (
	"archive/zip"
	"encoding/json"
	"os"
)

// buildZip 构造测试用 zip 包（条目名 → 内容）。
func buildZip(path string, files map[string][]byte) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write(content); err != nil {
			return err
		}
	}
	return zw.Close()
}

// jsonMarshal 序列化 JSON（测试辅助，避免测试文件散落 import）。
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
