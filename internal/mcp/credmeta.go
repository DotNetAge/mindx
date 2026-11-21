package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 凭据的展示元信息（展示名、字段中文名）是应用层约定，存放位置由机制层决定：
// 一律写进 CredentialStore 的 Meta，不另建 credentials.json 之类的旁挂文件。
// 理由：钥匙串与加密文件只是同一机制的两种实现，上层若按存储形态落盘，
// 换平台或换后端就会留下孤儿文件，也把实现细节泄露到了应用层。

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

// groupCredentialKey 组内字段的存储键：group:<组名>:<字段名>。
func groupCredentialKey(group, field string) string {
	return "group:" + group + ":" + field
}

// splitGroupCredentialKey 拆出 (组名, 字段名)；非组键返回空。
func splitGroupCredentialKey(key string) (string, string, bool) {
	rest, ok := strings.CutPrefix(key, "group:")
	if !ok {
		return "", "", false
	}
	group, field, ok := strings.Cut(rest, ":")
	if !ok || group == "" || field == "" {
		return "", "", false
	}
	return group, field, true
}

// SaveGroupMeta 记录一组凭据的展示名与字段中文名（值不落盘）。
// 展示名写在每个字段的元信息里：枚举只回条目，机制层不做组聚合，
// 组语义留在这一层。
func (m *Manager) SaveGroupMeta(group, display string, fields map[string]string) error {
	if group == "" || m.credStore == nil {
		return nil
	}
	existing, err := m.credStore.List("group:" + group + ":")
	if err != nil {
		return err
	}
	old := map[string]string{}
	for _, e := range existing {
		if _, field, ok := splitGroupCredentialKey(e.Key); ok {
			old[field] = e.Meta["label"]
		}
		if display == "" {
			display = e.Meta["display"]
		}
	}
	if display == "" {
		display = group
	}

	keys := map[string]bool{}
	for k := range old {
		keys[k] = true
	}
	for k := range fields {
		keys[k] = true
	}
	for k := range keys {
		label := fields[k]
		if label == "" {
			label = old[k]
		}
		if err := m.credStore.SetMeta(groupCredentialKey(group, k), map[string]string{
			"display": display,
			"label":   label,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ListCredentialGroups 汇总「有哪些组、每组用了哪些字段、被哪些连接器引用」。
// 只回展示名与是否已配置，不回值——设置页可以放心截图。
func (m *Manager) ListCredentialGroups() ([]CredentialGroupView, error) {
	// 先搬迁旧版旁挂元信息，再枚举；否则首批展示名会退回组名。
	if err := m.migrateLegacyCredentialMeta(); err != nil {
		m.log.Warn("凭据元信息迁移失败（不影响使用）", "err", err)
	}

	servers, err := LoadServers(m.configPath)
	if err != nil {
		return nil, err
	}

	var entries []credentialEntryLike
	if m.credStore != nil {
		es, err := m.credStore.List("group:")
		if err != nil {
			return nil, err
		}
		for _, e := range es {
			if group, field, ok := splitGroupCredentialKey(e.Key); ok {
				entries = append(entries, credentialEntryLike{Key: e.Key, Group: group, Field: field, Meta: e.Meta})
			}
		}
	}

	used := map[string][]string{}
	declared := map[string]map[string]bool{}
	for _, s := range servers {
		if s.CredentialGroup == "" {
			continue
		}
		used[s.CredentialGroup] = append(used[s.CredentialGroup], s.Name)
		if declared[s.CredentialGroup] == nil {
			declared[s.CredentialGroup] = map[string]bool{}
		}
		for k := range s.Env {
			declared[s.CredentialGroup][k] = true
		}
		for k := range s.Headers {
			declared[s.CredentialGroup][k] = true
		}
	}

	groups := map[string]bool{}
	for g := range used {
		groups[g] = true
	}
	for _, e := range entries {
		groups[e.Group] = true
	}

	out := make([]CredentialGroupView, 0, len(groups))
	for group := range groups {
		fieldLabels := map[string]string{}
		display := ""
		for _, e := range entries {
			if e.Group != group {
				continue
			}
			fieldLabels[e.Field] = e.Meta["label"]
			if display == "" {
				display = e.Meta["display"]
			}
		}
		if display == "" {
			display = group
		}

		fieldSet := map[string]bool{}
		for k := range declared[group] {
			fieldSet[k] = true
		}
		for _, e := range entries {
			if e.Group == group {
				fieldSet[e.Field] = true
			}
		}

		fields := make([]CredentialFieldView, 0, len(fieldSet))
		for k := range fieldSet {
			label := fieldLabels[k]
			if label == "" {
				label = k
			}
			has := false
			if m.credStore != nil {
				if v, err := m.credStore.Get(groupCredentialKey(group, k)); err == nil && v != "" {
					has = true
				}
			}
			fields = append(fields, CredentialFieldView{Key: k, Label: label, HasValue: has})
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i].Label < fields[j].Label })
		names := used[group]
		sort.Strings(names)
		out = append(out, CredentialGroupView{Group: group, Display: display, Fields: fields, UsedBy: names})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Display < out[j].Display })

	return out, nil
}

// credentialEntryLike 是本地中间结构，避免把机制层的条目类型散到多处。
type credentialEntryLike struct {
	Key   string
	Group string
	Field string
	Meta  map[string]string
}

// migrateLegacyCredentialMeta 把旧版旁挂的 credentials.json 一次性搬进凭据库。
// 旧文件是分层的错误做法（元信息脱离机制层单独落盘），此处只做单向搬迁，
// 搬完改名，不再读也不再写。
func (m *Manager) migrateLegacyCredentialMeta() error {
	if m.credStore == nil {
		return nil
	}
	path := filepath.Join(filepath.Dir(m.configPath), "credentials.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var legacy struct {
		Groups map[string]struct {
			Group     string            `json:"group"`
			Display   string            `json:"display"`
			Fields    map[string]string `json:"fields"`
			UpdatedAt time.Time         `json:"updated_at"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	for name, g := range legacy.Groups {
		group := g.Group
		if group == "" {
			group = name
		}
		if err := m.SaveGroupMeta(group, g.Display, g.Fields); err != nil {
			return err
		}
	}
	return os.Rename(path, path+".migrated")
}
