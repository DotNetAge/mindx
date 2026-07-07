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
// Data stored in:
//   - Global: ~/.mindx/data/entity-defs.json
//   - Per-directory: ~/.mindx/projects/<sha(projectDir)>/entity-defs.json
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

// regionIDForProject 返回 projectDir 的 region ID（sha256 hex）。
func regionIDForProject(projectDir string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(projectDir))))
}

// entityDefsPath 返回全局 entity-defs.json 的完整路径。
func (d *Daemon) entityDefsPath() string {
	return filepath.Join(d.app.Settings().DataDir(), "entity-defs.json")
}

// entityDefsPathForProject 返回指定目录的局部 entity-defs.json 路径。
func (d *Daemon) entityDefsPathForProject(projectDir string) string {
	hash := regionIDForProject(projectDir)
	return filepath.Join(d.app.Settings().UserPreferences(), "projects", hash, "entity-defs.json")
}

// loadEntityDefs 从磁盘加载全局 entity-defs.json，文件不存在时返回空结构。
func (d *Daemon) loadEntityDefs() (*entityTagsFile, error) {
	return d.loadEntityDefsFromPath(d.entityDefsPath())
}

// loadEntityDefsForProject 加载指定目录的局部 entity-defs.json；
// 局部文件不存在时回退到全局配置。
func (d *Daemon) loadEntityDefsForProject(projectDir string) (*entityTagsFile, error) {
	if projectDir == "" {
		return d.loadEntityDefs()
	}
	localPath := d.entityDefsPathForProject(projectDir)
	if _, err := os.Stat(localPath); err == nil {
		return d.loadEntityDefsFromPath(localPath)
	}
	return d.loadEntityDefs()
}

// loadEntityDefsFromPath 从指定路径加载 entity-defs.json。
func (d *Daemon) loadEntityDefsFromPath(path string) (*entityTagsFile, error) {
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

// writeEntityDefsFile 把 entityTagsFile 写入指定路径。
func (d *Daemon) writeEntityDefsFile(path string, f *entityTagsFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (d *Daemon) handleEntityTagsGet(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		ProjectDir string `json:"projectDir,omitempty"`
	}
	// 兼容旧调用：params 可能为空。
	if len(params) > 0 && string(params) != "null" {
		if err := unmarshalParams(params, &p); err != nil {
			return nil, err
		}
	}

	f, err := d.loadEntityDefsForProject(p.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("entity_tags.get: %w", err)
	}
	return f, nil
}

// schemasDir 返回 ~/.mindx/schemas/ 目录。
func (d *Daemon) schemasDir() string {
	return filepath.Join(d.app.Settings().UserPreferences(), "schemas")
}

// projectSchemasDir 返回 ~/.mindx/projects/<sha(projectDir)>/schemas/ 目录。
func (d *Daemon) projectSchemasDir(projectDir string) string {
	hash := regionIDForProject(projectDir)
	return filepath.Join(d.app.Settings().UserPreferences(), "projects", hash, "schemas")
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

// copySchemaToProject 将全局 schema 复制到 project 目录；
// 若 project 目录中已存在同名 schema，则保留现有文件以允许局部定制。
func (d *Daemon) copySchemaToProject(category, name, projectDir string) {
	if category == "" || name == "" || projectDir == "" {
		return
	}
	srcPath := filepath.Join(d.schemasDir(), category, name+".json")
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return
	}

	projSchemasDir := d.projectSchemasDir(projectDir)
	dstPath := filepath.Join(projSchemasDir, category, name+".json")
	if _, statErr := os.Stat(dstPath); statErr == nil {
		// 已存在局部 schema，保留用户定制。
		return
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		d.logger.Warn("entity_tags.save: mkdir project schema dir", "path", dstPath, "error", err)
		return
	}
	if err := os.WriteFile(dstPath, srcData, 0644); err != nil {
		d.logger.Warn("entity_tags.save: copy project schema", "path", dstPath, "error", err)
	}
}

// effectiveEntityDefs 返回指定目录生效的 EntityDef 列表。
// 若目录存在局部 entity-defs.json 则使用局部配置，否则回退全局。
// Schema 优先从局部目录读取，不存在时回退全局。
func (d *Daemon) effectiveEntityDefs(projectDir string) []goragindexer.EntityDef {
	f, err := d.loadEntityDefsForProject(projectDir)
	if err != nil || len(f.Types) == 0 {
		return nil
	}

	defs := make([]goragindexer.EntityDef, 0, len(f.Types))
	for _, t := range f.Types {
		if t.Name == "" {
			continue
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

func (d *Daemon) handleEntityTagsSave(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Types      []entityDefFileEntry `json:"types"`
		ProjectDir string               `json:"projectDir,omitempty"`
	}
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	// 补全 Prompt 和 Schema（全局 schema 作为来源）
	for i := range p.Types {
		t := &p.Types[i]
		if t.Prompt == "" && t.Name != "" {
			t.Prompt = "**" + t.Name + "** — " + t.Desc
		}
		if t.Schema == "" {
			t.Schema = d.readSchemaFile(t.Category, t.Name)
		}
	}

	// 构建文件结构
	f := entityTagsFile{
		Domain: "user",
		Title:  "自定义实体标签",
		Types:  p.Types,
	}

	if p.ProjectDir != "" {
		// ── 局部配置：保存到 ~/.mindx/projects/<sha(projectDir)>/entity-defs.json ──
		for _, t := range p.Types {
			if t.Category == "" || t.Name == "" {
				continue
			}
			// 将选中的 schema 从全局复制到局部（不覆盖已有局部定制）
			d.copySchemaToProject(t.Category, t.Name, p.ProjectDir)
		}

		localPath := d.entityDefsPathForProject(p.ProjectDir)
		if err := d.writeEntityDefsFile(localPath, &f); err != nil {
			return nil, fmt.Errorf("entity_tags.save: write project entity-defs: %w", err)
		}
		d.logger.Info("entity_tags saved", "path", localPath, "projectDir", p.ProjectDir, "count", len(p.Types))
	} else {
		// ── 全局配置：保存到 ~/.mindx/data/entity-defs.json ──
		if err := d.writeEntityDefsFile(d.entityDefsPath(), &f); err != nil {
			return nil, fmt.Errorf("entity_tags.save: write global entity-defs: %w", err)
		}
		d.logger.Info("entity_tags saved", "path", d.entityDefsPath(), "count", len(p.Types))
	}

	// ── 同步更新 GraphIndexer 的实体定义 ──────────────────────────
	if d.graphIndexer != nil {
		defs := d.effectiveEntityDefs(p.ProjectDir)
		if len(defs) > 0 {
			if p.ProjectDir != "" {
				regionID := regionIDForProject(p.ProjectDir)
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
