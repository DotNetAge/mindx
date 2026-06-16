package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	goragindexer "github.com/DotNetAge/gorag/indexer"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// EntityTags JSON-RPC handlers
// Data stored in ~/.mindx/data/entity_tags.yml
// ---------------------------------------------------------------------------

// entityTagDef 是单个实体标签定义。
type entityTagDef struct {
	Name     string `yaml:"name" json:"name"`
	Title    string `yaml:"title" json:"title"`
	Desc     string `yaml:"desc" json:"desc"`
	Category string `yaml:"category,omitempty" json:"category,omitempty"`
}

// entityTagsFile 是 entity_tags.yml 的文件结构。
type entityTagsFile struct {
	Domain string         `yaml:"domain" json:"domain"`
	Title  string         `yaml:"title" json:"title"`
	Types  []entityTagDef `yaml:"types" json:"types"`
}

// entityTagsPath 返回 entity_tags.yml 的完整路径。
func (d *Daemon) entityTagsPath() string {
	return filepath.Join(d.app.Settings().DataDir(), "entity_tags.yml")
}

// loadEntityTags 从磁盘加载 entity_tags.yml，文件不存在时返回空结构。
func (d *Daemon) loadEntityTags() (*entityTagsFile, error) {
	path := d.entityTagsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &entityTagsFile{Domain: "user", Title: "自定义实体标签"}, nil
		}
		return nil, fmt.Errorf("read entity_tags: %w", err)
	}

	var f entityTagsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse entity_tags: %w", err)
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
	f, err := d.loadEntityTags()
	if err != nil {
		return nil, fmt.Errorf("entity_tags.get: %w", err)
	}
	return f, nil
}

type entityTagsSaveParams struct {
	Types []entityTagDef `json:"types"`
}

func (d *Daemon) handleEntityTagsSave(_ context.Context, params json.RawMessage) (any, error) {
	var p entityTagsSaveParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	// 构建文件结构
	f := entityTagsFile{
		Domain: "user",
		Title:  "自定义实体标签",
		Types:  p.Types,
	}

	// 序列化为 YAML
	data, err := yaml.Marshal(&f)
	if err != nil {
		return nil, fmt.Errorf("entity_tags.save: marshal yaml: %w", err)
	}

	// 写入文件
	path := d.entityTagsPath()
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
			// 从 Types 构建 entityDefs 字符串列表
			defs := make([]string, 0, len(p.Types))
			for _, t := range p.Types {
				if t.Name != "" {
					defs = append(defs, "**"+t.Name+"** — "+t.Desc)
				}
			}
			if len(defs) > 0 {
				// 通过 HybridIndexer 获取 LLMIndexer 并更新实体定义
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

// parseEntityDefsFromSavedTags 从 entity_tags.yml 读取并解析为 entityDefs 字符串列表。
// 用于初始化 LLMIndexer 时加载用户此前保存的实体标签。
//nolint:unused
func (d *Daemon) _parseEntityDefsFromSavedTags() []string {
	f, err := d.loadEntityTags()
	if err != nil {
		d.logger.Warn("entity_tags: failed to load saved tags, using defaults", "error", err)
		return nil
	}
	if len(f.Types) == 0 {
		return nil
	}
	defs := make([]string, 0, len(f.Types))
	for _, t := range f.Types {
		if t.Name != "" {
			defs = append(defs, "**"+t.Name+"** — "+t.Desc)
		}
	}
	d.logger.Info("entity_tags: loaded saved entity defs", "count", len(defs))
	return defs
}
