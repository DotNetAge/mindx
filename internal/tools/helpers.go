package tools

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
)

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
