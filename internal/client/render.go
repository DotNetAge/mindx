package client

import (
	"fmt"

	"charm.land/bubbles/v2/table"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/mattn/go-runewidth"
)

func renderTableEnvelope(env *gateway.ResponseEnvelope, availWidth int) string {
	data, ok := env.Data.(map[string]any)
	if !ok {
		return env.Title
	}
	rawHeaders, _ := data["headers"].([]any)
	rawRows, _ := data["rows"].([]any)
	if len(rawHeaders) == 0 {
		return env.Title
	}
	cols := len(rawHeaders)
	colWidths := make([]int, cols)
	headers := make([]string, cols)
	for i, h := range rawHeaders {
		headers[i] = fmt.Sprintf("%v", h)
		colWidths[i] = runewidth.StringWidth(headers[i]) + 2
	}
	var rows []table.Row
	for _, row := range rawRows {
		cells, _ := row.([]any)
		if len(cells) == 0 {
			continue
		}
		rowCells := make(table.Row, cols)
		for j := 0; j < cols && j < len(cells); j++ {
			cell := fmt.Sprintf("%v", cells[j])
			maxCell := availWidth / cols
			if maxCell < 10 {
				maxCell = 80
			}
			if runewidth.StringWidth(cell) > maxCell {
				cell = runewidth.Truncate(cell, maxCell-3, "...")
			}
			rowCells[j] = cell
			if w := runewidth.StringWidth(cell) + 2; w > colWidths[j] {
				colWidths[j] = w
			}
		}
		rows = append(rows, rowCells)
	}
	var columns []table.Column
	for i, h := range headers {
		columns = append(columns, table.Column{Title: h, Width: colWidths[i]})
	}
	t := table.New()
	t.SetColumns(columns)
	t.SetRows(rows)
	totalWidth := 0
	for _, w := range colWidths {
		totalWidth += w
	}
	maxTableWidth := availWidth
	if maxTableWidth <= 0 || maxTableWidth > 120 {
		maxTableWidth = 120
	}
	if totalWidth > maxTableWidth {
		totalWidth = maxTableWidth
	}
	t.SetWidth(totalWidth)
	return ThemeTitleStyle.Render(env.Title) + "\n" + t.View()
}

func renderTodoEnvelope(env *gateway.ResponseEnvelope, availWidth int) string {
	data, ok := env.Data.(map[string]any)
	if !ok {
		return env.Title
	}
	todos, _ := data["todos"].([]any)
	if len(todos) == 0 {
		return env.Title
	}
	var cols []table.Column
	var rows []table.Row
	contentWidth := availWidth - 28 // 状态8 + 负责人16 + padding4
	if contentWidth < 20 {
		contentWidth = 60
	}
	cols = append(cols, table.Column{Title: "状态", Width: 8})
	cols = append(cols, table.Column{Title: "负责人", Width: 16})
	cols = append(cols, table.Column{Title: "内容", Width: contentWidth})
	for _, item := range todos {
		todo, ok := item.(map[string]any)
		if !ok {
			continue
		}
		completed, _ := todo["completed"].(bool)
		content := fmt.Sprintf("%v", todo["content"])
		if runewidth.StringWidth(content) > contentWidth {
			content = runewidth.Truncate(content, contentWidth-3, "...")
		}
		assignee, _ := todo["assignee"].(string)
		status := "○"
		if completed {
			status = "✓"
		}
		if assignee != "" {
			rows = append(rows, table.Row{status, assignee, content})
		} else {
			rows = append(rows, table.Row{status, "", content})
		}
	}
	t := table.New()
	t.SetColumns(cols)
	t.SetRows(rows)
	tableWidth := 8 + 16 + contentWidth
	if tableWidth > availWidth && availWidth > 0 {
		tableWidth = availWidth
	}
	t.SetWidth(tableWidth)
	return ThemeTitleStyle.Render(env.Title) + "\n" + t.View()
}

func renderOptionsEnvelope(env *gateway.ResponseEnvelope, availWidth int) string {
	data, ok := env.Data.(map[string]any)
	if !ok {
		return env.Title
	}
	items, _ := data["data"].([]any)
	if len(items) == 0 {
		return env.Title
	}
	optionWidth := availWidth - 24 // #号4 + 角色20
	if optionWidth < 20 {
		optionWidth = 40
	}
	roleWidth := 20
	if roleWidth > availWidth/3 {
		roleWidth = availWidth / 3
	}
	var cols []table.Column
	var rows []table.Row
	cols = append(cols, table.Column{Title: "#", Width: 4})
	cols = append(cols, table.Column{Title: "选项", Width: optionWidth})
	cols = append(cols, table.Column{Title: "角色", Width: roleWidth})
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		label, _ := m["label"].(string)
		role, _ := m["role"].(string)
		if runewidth.StringWidth(label) > optionWidth {
			label = runewidth.Truncate(label, optionWidth-3, "...")
		}
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", i+1),
			label,
			role,
		})
	}
	t := table.New()
	t.SetColumns(cols)
	t.SetRows(rows)
	tableWidth := 4 + optionWidth + roleWidth
	if tableWidth > availWidth && availWidth > 0 {
		tableWidth = availWidth
	}
	t.SetWidth(tableWidth)
	return ThemeTitleStyle.Render(env.Title) + "\n" + t.View()
}

// renderActionStart 从 ResponseEnvelope 提取 ActionStartData，生成动作日志所需的内容字符串。
// 格式: "toolName|predictedTokens"
func renderActionStart(env *gateway.ResponseEnvelope, _ int) string {
	data, ok := env.Data.(map[string]any)
	if !ok {
		return ""
	}
	toolName, _ := data["tool_name"].(string)
	if toolName == "" {
		return ""
	}
	predictedTokens := 0
	if pt, ok := data["predicted_tokens"].(float64); ok {
		predictedTokens = int(pt)
	}
	return fmt.Sprintf("%s|%d", toolName, predictedTokens)
}

// renderActionResult 从 ResponseEnvelope 提取 ActionResultData，生成动作结果字符串。
// 格式: "success|duration|resultText" 或 "failed|errorText"
func renderActionResult(env *gateway.ResponseEnvelope, _ int) string {
	data, ok := env.Data.(map[string]any)
	if !ok {
		return ""
	}
	success, _ := data["success"].(bool)
	duration, _ := data["duration"].(string)
	resultText, _ := data["result"].(string)
	errorText, _ := data["error"].(string)

	if success {
		if resultText != "" {
			return "success|" + duration + "|" + truncateStr(resultText, 200)
		}
		return "success|" + duration
	}
	if errorText != "" {
		return "failed|" + truncateStr(errorText, 200)
	}
	return "failed|未知错误"
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}