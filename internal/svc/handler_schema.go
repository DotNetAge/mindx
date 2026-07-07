package svc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Schema CRUD JSON-RPC handlers
// Schema files stored in:
//   - Global: ~/.mindx/schemas/{category}/{Name}.json
//   - Per-directory: ~/.mindx/projects/<sha(projectDir)>/schemas/{category}/{Name}.json
// ---------------------------------------------------------------------------

// schemaEntry 是 schema.list 返回的单个条目结构。
type schemaEntry struct {
	Category string `json:"category"`
	Name     string `json:"name"`
}

// schemaListResult 是 schema.list 的返回结构。
type schemaListResult struct {
	Schemas []schemaEntry `json:"schemas"`
}

// schemaFilePathForProject 返回 schema 文件路径。
// 若指定 projectDir，优先使用局部目录；否则使用全局目录。
func (d *Daemon) schemaFilePathForProject(category, name, projectDir string) string {
	if projectDir != "" {
		return filepath.Join(d.projectSchemasDir(projectDir), category, name+".json")
	}
	return filepath.Join(d.schemasDir(), category, name+".json")
}

func (d *Daemon) handleSchemaGet(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Category   string `json:"category"`
		Name       string `json:"name"`
		ProjectDir string `json:"projectDir,omitempty"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Category == "" || p.Name == "" {
		return nil, fmt.Errorf("schema.get: category and name are required")
	}

	// 优先读取局部 schema，不存在时回退全局
	path := d.schemaFilePathForProject(p.Category, p.Name, p.ProjectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 若请求的是局部目录且局部不存在，回退到全局
			if p.ProjectDir != "" {
				path = d.schemaFilePathForProject(p.Category, p.Name, "")
				data, err = os.ReadFile(path)
			}
		}
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("schema.get: schema not found: %s/%s", p.Category, p.Name)
			}
			return nil, fmt.Errorf("schema.get: read file: %w", err)
		}
	}

	// 解析为 JSON 对象以返回结构化 schema
	var schema any
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("schema.get: parse schema: %w", err)
	}

	return map[string]any{
		"category": p.Category,
		"name":     p.Name,
		"schema":   schema,
	}, nil
}

func (d *Daemon) handleSchemaSave(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Category   string          `json:"category"`
		Name       string          `json:"name"`
		Schema     json.RawMessage `json:"schema"`
		ProjectDir string          `json:"projectDir,omitempty"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Category == "" || p.Name == "" {
		return nil, fmt.Errorf("schema.save: category and name are required")
	}
	if len(p.Schema) == 0 {
		return nil, fmt.Errorf("schema.save: schema is required")
	}

	// 验证 schema 是合法的 JSON
	if !json.Valid(p.Schema) {
		return nil, fmt.Errorf("schema.save: schema is not valid JSON")
	}

	path := d.schemaFilePathForProject(p.Category, p.Name, p.ProjectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("schema.save: create dir: %w", err)
	}

	// 美化输出
	var buf bytes.Buffer
	if err := json.Indent(&buf, p.Schema, "", "  "); err != nil {
		return nil, fmt.Errorf("schema.save: indent json: %w", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return nil, fmt.Errorf("schema.save: write file: %w", err)
	}

	d.logger.Info("schema saved", "path", path, "category", p.Category, "name", p.Name, "projectDir", p.ProjectDir)

	return map[string]any{
		"status": "ok",
	}, nil
}

func (d *Daemon) handleSchemaList(_ context.Context, _ json.RawMessage) (any, error) {
	dir := d.schemasDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return schemaListResult{Schemas: []schemaEntry{}}, nil
		}
		return nil, fmt.Errorf("schema.list: read schemas dir: %w", err)
	}

	var schemas []schemaEntry

	for _, catEntry := range entries {
		if !catEntry.IsDir() {
			continue
		}
		catDir := filepath.Join(dir, catEntry.Name())
		files, err := os.ReadDir(catDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
				continue
			}
			schemas = append(schemas, schemaEntry{
				Category: catEntry.Name(),
				Name:     strings.TrimSuffix(f.Name(), ".json"),
			})
		}
	}

	if schemas == nil {
		schemas = []schemaEntry{}
	}

	return schemaListResult{Schemas: schemas}, nil
}
