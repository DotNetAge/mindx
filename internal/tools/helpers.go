package tools

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/DotNetAge/gorag/v2/core"
)

// ── Parameter access helpers ───────────────────────────────────────────────────────

// getParam 从 params 中获取参数，支持 camelCase / snake_case 变体容错。
// 先精确匹配给定 key，若未命中则尝试所有 key 的常见变体。
func getParam(params map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if val, ok := params[key]; ok {
			return val, true
		}
	}
	for _, key := range keys {
		for _, variant := range paramKeyVariants(key) {
			if val, ok := params[variant]; ok {
				return val, true
			}
		}
	}
	return nil, false
}

// paramKeyVariants 生成 key 的 camelCase ↔ snake_case 变体。
func paramKeyVariants(key string) []string {
	var variants []string

	// camelCase → snake_case
	var sb strings.Builder
	for i, r := range key {
		if i > 0 && unicode.IsUpper(r) {
			sb.WriteRune('_')
			sb.WriteRune(unicode.ToLower(r))
		} else {
			sb.WriteRune(unicode.ToLower(r))
		}
	}
	if s := sb.String(); s != key {
		variants = append(variants, s)
	}

	// snake_case → camelCase
	parts := strings.Split(key, "_")
	if len(parts) > 1 {
		sb.Reset()
		for i, p := range parts {
			if i == 0 {
				sb.WriteString(p)
			} else if len(p) > 0 {
				sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
			}
		}
		if s := sb.String(); s != key {
			variants = append(variants, s)
		}

		// 也尝试 PascalCase 变体（首字母大写）
		sb.Reset()
		for _, p := range parts {
			if len(p) > 0 {
				sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
			}
		}
		if s := sb.String(); s != key {
			variants = append(variants, s)
		}
	}

	return variants
}

// ── Metadata helpers ──────────────────────────────────────────────────────────────

func hitSummary(hit *core.Hit) string {
	if v, ok := hit.Metadata["summary"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if hit.Title != "" {
		return hit.Title
	}
	if v, ok := hit.Metadata["title"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func hitSourceFile(hit *core.Hit) string {
	if v, ok := hit.Metadata["source_file"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if hit.DocID != "" {
		return hit.DocID
	}
	if v, ok := hit.Metadata["doc_id"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func hitLineRange(hit *core.Hit) (int, int) {
	if v, ok := hit.Metadata["chunk_meta"]; ok {
		if m, ok := v.(map[string]any); ok {
			sp, _ := m["start_pos"].(float64)
			ep, _ := m["end_pos"].(float64)
			return int(sp), int(ep)
		}
	}
	return 0, 0
}

func hitTags(hit *core.Hit) []string {
	v, ok := hit.Metadata["tags"]
	if !ok {
		return nil
	}
	switch tags := v.(type) {
	case []string:
		return tags
	case []any:
		result := make([]string, 0, len(tags))
		for _, t := range tags {
			if s, ok := t.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func hitChunkType(hit *core.Hit) string {
	if v, ok := hit.Metadata["chunk_type"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func hitParentID(hit *core.Hit) string {
	if v, ok := hit.Metadata["parent_id"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ── Filters ───────────────────────────────────────────────────────────────────────

func filterHitsByTags(hits []core.Hit, tags []string) []core.Hit {
	if len(tags) == 0 {
		return hits
	}
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}
	filtered := make([]core.Hit, 0, len(hits))
	for _, h := range hits {
		ht := hitTags(&h)
		for _, t := range ht {
			if tagSet[t] {
				filtered = append(filtered, h)
				break
			}
		}
	}
	return filtered
}

func filterHitsByEntityLabels(hits []core.Hit, labels []string) []core.Hit {
	if len(labels) == 0 {
		return hits
	}
	labelSet := make(map[string]bool, len(labels))
	for _, l := range labels {
		labelSet[l] = true
	}
	for i := range hits {
		if len(hits[i].Entities) == 0 {
			continue
		}
		filtered := make([]*core.Node, 0, len(hits[i].Entities))
		for _, e := range hits[i].Entities {
			for _, l := range e.Labels {
				if labelSet[l] {
					filtered = append(filtered, e)
					break
				}
			}
		}
		if len(filtered) == 0 {
			hits[i].Entities = nil
		} else {
			hits[i].Entities = filtered
		}
	}
	return hits
}

// ── Entity formatting ─────────────────────────────────────────────────────────────

func entityName(entities []*core.Node, id string) string {
	for _, e := range entities {
		if e.ID == id {
			return e.Name
		}
	}
	return ""
}

func formatEntityTable(entities []*core.Node) string {
	if len(entities) == 0 {
		return ""
	}

	propKeys := make([]string, 0)
	seen := make(map[string]bool)
	for _, e := range entities {
		for k := range e.Properties {
			if !seen[k] {
				seen[k] = true
				propKeys = append(propKeys, k)
			}
		}
	}
	sort.Strings(propKeys)

	var tb strings.Builder
	tb.WriteString("| ID | Name | Type")
	for _, k := range propKeys {
		tb.WriteString(fmt.Sprintf(" | %s", k))
	}
	tb.WriteString(" |\n")

	tb.WriteString("|---|------|------")
	for range propKeys {
		tb.WriteString("|------")
	}
	tb.WriteString("|\n")

	for _, e := range entities {
		label := "-"
		if len(e.Labels) > 0 {
			label = e.Labels[0]
		}
		eid := e.ID
		if eid == "" {
			eid = "-"
		}
		ename := e.Name
		if ename == "" {
			ename = "-"
		}
		tb.WriteString(fmt.Sprintf("| %s | %s | %s", eid, ename, label))
		for _, k := range propKeys {
			val := propertyValue(e.Properties[k])
			tb.WriteString(fmt.Sprintf(" | %s", val))
		}
		tb.WriteString(" |\n")
	}
	return tb.String()
}

func propertyValue(v any) string {
	if v == nil {
		return "-"
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return "-"
		}
		return val
	case float64:
		if val == 0 {
			return "-"
		}
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	case []any:
		strs := make([]string, 0, len(val))
		for _, item := range val {
			strs = append(strs, fmt.Sprintf("%v", item))
		}
		if len(strs) == 0 {
			return "-"
		}
		return strings.Join(strs, ",")
	case []string:
		if len(val) == 0 {
			return "-"
		}
		return strings.Join(val, ",")
	default:
		return fmt.Sprintf("%v", val)
	}
}
