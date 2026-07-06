package svc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
)

// ---------------------------------------------------------------------------
// EntityTags JSON-RPC handlers
// Data stored in ~/.mindx/data/entity-defs.json
// ---------------------------------------------------------------------------

// entityDefFileEntry 是 entity-defs.json 中单个实体类型条目。
// Prompt 和 Schema 由保存时刻自动填充，不需前端传入。
type entityDefFileEntry struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Desc     string `json:"desc"`
	Category string `json:"category,omitempty"`
	Prompt   string `json:"prompt,omitempty"` // 保存时自动生成或读取
	Schema   string `json:"schema,omitempty"` // 保存时从 schema 文件嵌入
}

// entityTagsFile 是 entity-defs.json 的文件结构。
type entityTagsFile struct {
	Domain string               `json:"domain"`
	Title  string               `json:"title"`
	Types  []entityDefFileEntry `json:"types"`
}

// entityDefsPath 返回 entity-defs.json 的完整路径。
func (d *Daemon) entityDefsPath() string {
	return filepath.Join(d.app.Settings().DataDir(), "entity-defs.json")
}

// loadEntityDefs 从磁盘加载 entity-defs.json，文件不存在时返回空结构。
func (d *Daemon) loadEntityDefs() (*entityTagsFile, error) {
	path := d.entityDefsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &entityTagsFile{Domain: "user", Title: "自定义实体标签"}, nil
		}
		return nil, fmt.Errorf("read entity-defs: %w", err)
	}

	var f entityTagsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse entity-defs: %w", err)
	}
	if f.Domain == "" {
		f.Domain = "user"
	}
	if f.Title == "" {
		f.Title = "自定义实体标签"
	}
	return &f, nil
}

func (d *Daemon) handleEntityTagsGet(_ context.Context, _ json.RawMessage) (any, error) {
	f, err := d.loadEntityDefs()
	if err != nil {
		return nil, fmt.Errorf("entity_tags.get: %w", err)
	}
	return f, nil
}

// schemasDir 返回 ~/.mindx/schemas/ 目录。
func (d *Daemon) schemasDir() string {
	return filepath.Join(d.app.Settings().DataDir(), "..", "schemas")
}

// projectSchemasDir 返回 ~/.mindx/projects/<sha(projectDir)>/schemas/ 目录。
func (d *Daemon) projectSchemasDir(projectDir string) string {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(projectDir))))
	return filepath.Join(d.app.Settings().DataDir(), "..", "projects", hash, "schemas")
}

// entityDefsForProject 加载 entity-defs.json，按 project 过滤只保留该 project 使用的 Schema。
// 如果 projectSchemasDir 不存在或为空，返回全部 entityDefs（兼容无 project 设置的情况）。
func (d *Daemon) entityDefsForProject(projectDir string) []goragindexer.EntityDef {
	f, err := d.loadEntityDefs()
	if err != nil || len(f.Types) == 0 {
		return nil
	}

	// 检查 project 是否有独立的 schemas 目录
	projSchemasDir := d.projectSchemasDir(projectDir)
	hasProjectSchemas := false
	if info, statErr := os.Stat(projSchemasDir); statErr == nil && info.IsDir() {
		hasProjectSchemas = true
	}

	defs := make([]goragindexer.EntityDef, 0, len(f.Types))
	for _, t := range f.Types {
		if t.Name == "" {
			continue
		}

		// 如果 project 有独立 schemas 目录，检查该类型对应的 schema 文件是否存在
		if hasProjectSchemas && t.Category != "" {
			schemaPath := filepath.Join(projSchemasDir, t.Category, t.Name+".json")
			if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
				continue // 跳过，该 schema 未被复制到 project 目录
			}
		}

		prompt := t.Prompt
		if prompt == "" {
			prompt = "**" + t.Name + "** — " + t.Desc
		}
		schema := t.Schema
		if schema == "" {
			schema = d.readSchemaFileForProject(t.Category, t.Name, projectDir)
		}
		defs = append(defs, goragindexer.EntityDef{
			Prompt: prompt,
			Schema: schema,
		})
	}

	return defs
}

// readSchemaFile 读取 schema 文件，返回文件内容（JSON Schema 文本）。
// 优先从 projectSchemasDir 读取，不存在时回退到全局 schemasDir。
// projectDir 为空时直接从全局目录读取。
// 文件不存在时返回空字符串。
func (d *Daemon) readSchemaFile(category, name string) string {
	return d.readSchemaFileForProject(category, name, "")
}

// readSchemaFileForProject 从指定 project 的 schemas 目录读取 schema 文件。
// project 目录不存在时回退到全局目录。
func (d *Daemon) readSchemaFileForProject(category, name, projectDir string) string {
	if category == "" {
		return ""
	}
	// 优先从 project 目录读取
	if projectDir != "" {
		path := filepath.Join(d.projectSchemasDir(projectDir), category, name+".json")
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	// 回退到全局目录
	path := filepath.Join(d.schemasDir(), category, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (d *Daemon) handleEntityTagsSave(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Types      []entityDefFileEntry `json:"types"`
		ProjectDir string               `json:"projectDir,omitempty"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	// 补全 Prompt 和 Schema
	for i := range p.Types {
		t := &p.Types[i]
		if t.Prompt == "" && t.Name != "" {
			t.Prompt = "**" + t.Name + "** — " + t.Desc
		}
		if t.Schema == "" {
			t.Schema = d.readSchemaFile(t.Category, t.Name)
		}
	}

	// ── 如果指定了 projectDir，将 schema 文件写入 project 目录 ───────────
	if p.ProjectDir != "" {
		projSchemasDir := d.projectSchemasDir(p.ProjectDir)
		for _, t := range p.Types {
			if t.Category == "" || t.Name == "" || t.Schema == "" {
				continue
			}
			schemaPath := filepath.Join(projSchemasDir, t.Category, t.Name+".json")
			if err := os.MkdirAll(filepath.Dir(schemaPath), 0755); err != nil {
				d.logger.Warn("entity_tags.save: mkdir project schema dir", "path", schemaPath, "error", err)
				continue
			}
			if err := os.WriteFile(schemaPath, []byte(t.Schema), 0644); err != nil {
				d.logger.Warn("entity_tags.save: write project schema", "path", schemaPath, "error", err)
			}
		}
		d.logger.Info("entity_tags.save: wrote project schemas", "projectDir", p.ProjectDir, "count", len(p.Types))
	}

	// 构建文件结构
	f := entityTagsFile{
		Domain: "user",
		Title:  "自定义实体标签",
		Types:  p.Types,
	}

	// 序列化为 JSON
	data, err := json.MarshalIndent(&f, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("entity_tags.save: marshal json: %w", err)
	}

	// 写入文件
	path := d.entityDefsPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("entity_tags.save: create dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("entity_tags.save: write file: %w", err)
	}

	d.logger.Info("entity_tags saved", "path", path, "count", len(p.Types))

	// ── 同步更新 GraphIndexer 的实体定义 ──────────────────────────
	if d.graphIndexer != nil && len(p.Types) > 0 {
		defs := make([]goragindexer.EntityDef, 0, len(p.Types))
		for _, t := range p.Types {
			if t.Name != "" {
				defs = append(defs, goragindexer.EntityDef{
					Prompt: t.Prompt,
					Schema: t.Schema,
				})
			}
		}
		if len(defs) > 0 {
			if p.ProjectDir != "" {
				regionID := fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(p.ProjectDir))))
				d.graphIndexer.SetEntityDefsByRegion(regionID, defs)
				d.logger.Info("entity_tags: updated GraphIndexer region entity defs", "regionID", regionID, "count", len(defs))
			} else {
				d.graphIndexer.SetEntityDefs(defs)
				d.logger.Info("entity_tags: updated GraphIndexer entity defs", "count", len(defs))
			}
		}
	}

	return map[string]any{
		"status": "ok",
		"count":  len(p.Types),
	}, nil
}

// parseEntityDefsFromSavedTags 从 entity-defs.json 读取并解析为 EntityDef 列表。
// 用于初始化 GraphIndexer 时加载用户此前保存的实体标签。
//
//nolint:unused
func (d *Daemon) _parseEntityDefsFromSavedTags() []goragindexer.EntityDef {
	f, err := d.loadEntityDefs()
	if err != nil {
		d.logger.Warn("entity_tags: failed to load saved tags, using defaults", "error", err)
		return nil
	}
	if len(f.Types) == 0 {
		return nil
	}
	defs := make([]goragindexer.EntityDef, 0, len(f.Types))
	for _, t := range f.Types {
		if t.Name == "" {
			continue
		}
		defs = append(defs, goragindexer.EntityDef{
			Prompt: t.Prompt,
			Schema: t.Schema,
		})
	}
	d.logger.Info("entity_tags: loaded saved entity defs", "count", len(defs))
	return defs
}

// ---------------------------------------------------------------------------
// Schema Properties RPC — 从 runtime/schemas/*.json 提取每个实体类型的属性列表
// ---------------------------------------------------------------------------

// schemaPropertyMap 是 kb.schema_properties 的返回结构。
type schemaPropertyMap struct {
	// key = 实体类型名（如 "Topic", "CoreTheory", "Customer"）
	// value = 该实体类型在 Schema 中定义的所有属性 key 列表
	Schemas map[string][]string `json:"schemas"`
}

func (d *Daemon) handleSchemaProperties(_ context.Context, _ json.RawMessage) (any, error) {
	dir := d.schemasDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return schemaPropertyMap{Schemas: map[string][]string{}}, nil
		}
		return nil, fmt.Errorf("schema_properties: read schemas dir: %w", err)
	}

	result := make(map[string][]string)

	for _, catEntry := range entries {
		if !catEntry.IsDir() {
			continue
		}
		catDir := filepath.Join(dir, catEntry.Name())
		files, err := os.ReadDir(catDir)
		if err != nil {
			continue // 跳过无法读取的目录
		}
		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
				continue
			}
			entityType := strings.TrimSuffix(f.Name(), ".json")
			data, err := os.ReadFile(filepath.Join(catDir, f.Name()))
			if err != nil {
				continue
			}

			// 解析 JSON Schema 的 properties 字段
			var schema struct {
				Properties map[string]any `json:"properties"`
			}
			if json.Unmarshal(data, &schema) != nil {
				continue
			}

			keys := make([]string, 0, len(schema.Properties))
			for k := range schema.Properties {
				keys = append(keys, k)
			}
			result[entityType] = keys
		}
	}

	d.logger.Info("schema_properties: loaded", "types", len(result))
	return schemaPropertyMap{Schemas: result}, nil
}
