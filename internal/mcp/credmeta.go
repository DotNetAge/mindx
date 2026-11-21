package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// credentialMeta 是凭据的「展示元信息」，与密文分开存。
//
// 为什么元信息不进凭据库：凭据库（Keychain）只存 key→value，没有名字就只能在
// 界面上显示 `group:aliyun:ALIBABA_CLOUD_ACCESS_KEY_ID` 这种机器键，用户看不懂。
// 而展示名不是秘密，落一个普通 JSON 文件即可，也便于批量迁移与导出（不含值）。
type credentialMeta struct {
	Group     string            `json:"group"`
	Display   string            `json:"display"`
	Fields    map[string]string `json:"fields,omitempty"` // 字段键 → 中文名
	UpdatedAt time.Time         `json:"updated_at"`
}

type credentialMetaFile struct {
	Groups map[string]credentialMeta `json:"groups"`
}

// credentialMetaPath 与 mcp.json 同目录，跟着用户配置一起搬。
func credentialMetaPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "credentials.json")
}

func loadCredentialMeta(configPath string) (*credentialMetaFile, error) {
	m := &credentialMetaFile{Groups: map[string]credentialMeta{}}
	data, err := os.ReadFile(credentialMetaPath(configPath))
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}
	if m.Groups == nil {
		m.Groups = map[string]credentialMeta{}
	}
	return m, nil
}

func saveCredentialMeta(configPath string, m *credentialMetaFile) error {
	path := credentialMetaPath(configPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// SaveGroupMeta 记录一组凭据的展示名与字段中文名（值不落盘）。
func (m *Manager) SaveGroupMeta(group, display string, fields map[string]string) error {
	if group == "" {
		return nil
	}
	meta, err := loadCredentialMeta(m.configPath)
	if err != nil {
		return err
	}
	old := meta.Groups[group]
	if display == "" {
		display = old.Display
	}
	if display == "" {
		display = group
	}
	merged := map[string]string{}
	for k, v := range old.Fields {
		merged[k] = v
	}
	for k, v := range fields {
		if v != "" {
			merged[k] = v
		}
	}
	meta.Groups[group] = credentialMeta{
		Group:     group,
		Display:   display,
		Fields:    merged,
		UpdatedAt: time.Now(),
	}
	return saveCredentialMeta(m.configPath, meta)
}

// CredentialGroupView 是给前端看的凭据组视图（不含任何密钥值）。
type CredentialGroupView struct {
	Group   string                `json:"group"`
	Display string                `json:"display"`
	Fields  []CredentialFieldView `json:"fields"`
	UsedBy  []string              `json:"used_by"`
}

// CredentialFieldView 单个字段：中文名 + 是否已填。
type CredentialFieldView struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	HasValue bool   `json:"has_value"`
}

// ListCredentialGroups 汇总「有哪些组、每组用了哪些字段、被哪些连接器引用」。
// 只回展示名与是否已配置，不回值——设置页可以放心截图。
func (m *Manager) ListCredentialGroups() ([]CredentialGroupView, error) {
	meta, err := loadCredentialMeta(m.configPath)
	if err != nil {
		return nil, err
	}
	servers, err := LoadServers(m.configPath)
	if err != nil {
		return nil, err
	}
	used := map[string][]string{}
	for _, s := range servers {
		if s.CredentialGroup == "" {
			continue
		}
		used[s.CredentialGroup] = append(used[s.CredentialGroup], s.Name)
	}
	out := make([]CredentialGroupView, 0, len(used))
	for group, names := range used {
		gm := meta.Groups[group]
		display := gm.Display
		if display == "" {
			display = group
		}
		fieldSet := map[string]bool{}
		for _, s := range servers {
			if s.CredentialGroup != group {
				continue
			}
			for k := range s.Env {
				fieldSet[k] = true
			}
			for k := range s.Headers {
				fieldSet[k] = true
			}
		}
		fields := make([]CredentialFieldView, 0, len(fieldSet))
		for k := range fieldSet {
			label := gm.Fields[k]
			if label == "" {
				label = k
			}
			has := false
			if m.credStore != nil {
				if v, err := m.credStore.Get("group:" + group + ":" + k); err == nil && v != "" {
					has = true
				}
			}
			fields = append(fields, CredentialFieldView{Key: k, Label: label, HasValue: has})
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i].Label < fields[j].Label })
		sort.Strings(names)
		out = append(out, CredentialGroupView{Group: group, Display: display, Fields: fields, UsedBy: names})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Display < out[j].Display })
	return out, nil
}
