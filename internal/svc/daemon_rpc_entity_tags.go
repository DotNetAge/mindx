package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goragindexer "github.com/DotNetAge/gorag/indexer"
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

type entityTagsSaveParams struct {
	Types []entityDefFileEntry `json:"types"`
}

// schemasDir 返回 ~/.mindx/schemas/ 目录。
func (d *Daemon) schemasDir() string {
	return filepath.Join(d.app.Settings().DataDir(), "..", "schemas")
}

// readSchemaFile 读取 schema 文件，返回文件内容（JSON Schema 文本）。
// 文件不存在时返回空字符串。
func (d *Daemon) readSchemaFile(category, name string) string {
	if category == "" {
		return ""
	}
	path := filepath.Join(d.schemasDir(), category, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (d *Daemon) handleEntityTagsSave(_ context.Context, params json.RawMessage) (any, error) {
	var p entityTagsSaveParams
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

	// ── 同步更新 LLMIndexer 的实体定义 ──────────────────────────
	if d.sharedMemory != nil && len(p.Types) > 0 {
		hybrid := d.sharedMemory.Indexer()
		if hybrid != nil {
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
				if raw, ok := hybrid.GetIndexer("llm"); ok {
					if llmIdx, ok := raw.(*goragindexer.LLMIndexer); ok {
						llmIdx.SetEntityDefs(defs)
						d.logger.Info("entity_tags: updated LLMIndexer entity defs", "count", len(defs))
					}
				}
			}
		}
	}

	return map[string]any{
		"status": "ok",
		"count":  len(p.Types),
	}, nil
}

// parseEntityDefsFromSavedTags 从 entity-defs.json 读取并解析为 EntityDef 列表。
// 用于初始化 LLMIndexer 时加载用户此前保存的实体标签。
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
