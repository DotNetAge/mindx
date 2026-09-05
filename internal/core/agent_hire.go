package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goharnessconfig "github.com/DotNetAge/goharness/config"
)

// SetAgentHired 以文本级方式修改 Agent 文件 frontmatter 中 meta.hired 标记，
// 并同步更新内存注册表（无需 reload 即刻生效）。
//
// 之所以不走 AgentRegistry.SaveTo 全量重写：SaveTo 重建 frontmatter 时只序列化
// name/role/description/model/skills/meta，会丢失 exclude_tools 等手写字段；
// 本函数只精准插入或替换 hired 键，其余 frontmatter 内容原样保留。
// hired 缺省语义为 false（未雇佣），文件中显式写入 true/false 以固化状态。
func SetAgentHired(dir, name string, registry *goharnessconfig.AgentRegistry, hired bool) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("agent 名称不能为空")
	}
	if registry == nil {
		return fmt.Errorf("agent 注册表不可用")
	}
	if registry.Get(name) == nil {
		return fmt.Errorf("未找到智能体 %q，请确认名称是否正确", name)
	}

	filePath := filepath.Join(dir, strings.ToLower(name)+".md")
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("无法读取 Agent 文件 %s: %w", filePath, err)
	}

	updated, err := setFrontmatterHired(string(content), hired)
	if err != nil {
		return fmt.Errorf("修改 Agent %q 的雇佣标记失败: %w", name, err)
	}

	perm := os.FileMode(0644)
	if info, statErr := os.Stat(filePath); statErr == nil {
		perm = info.Mode().Perm()
	}
	if err := os.WriteFile(filePath, []byte(updated), perm); err != nil {
		return fmt.Errorf("无法写入 Agent 文件 %s: %w", filePath, err)
	}

	// 同步内存注册表：Get 返回内部指针，原地更新 Meta 即对全部消费方可见
	if cfg := registry.Get(name); cfg != nil {
		if cfg.Meta == nil {
			cfg.Meta = make(map[string]any)
		}
		cfg.Meta["hired"] = hired
	}
	return nil
}

// setFrontmatterHired 在 Markdown 文本的 YAML frontmatter 中插入或替换
// meta.hired 键，返回修改后的完整文本。frontmatter 结构要求：首行为 "---"，
// 随后为 YAML 文本，再以 "---" 结束；meta 块的子键使用两空格缩进。
func setFrontmatterHired(content string, hired bool) (string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return "", fmt.Errorf("文件缺少 YAML frontmatter")
	}

	// 定位 frontmatter 结束行
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return "", fmt.Errorf("frontmatter 未正确闭合")
	}

	// 在 frontmatter 中定位顶层 meta: 块（要求冒号后无键名混淆，精确匹配）
	metaIdx := -1
	for i := 1; i < end; i++ {
		if lines[i] == "meta:" {
			metaIdx = i
			break
		}
	}

	hiredLine := fmt.Sprintf("  hired: %t", hired)

	// 无 meta 块：在 frontmatter 末尾（结束行之前）插入新块
	if metaIdx == -1 {
		out := make([]string, 0, len(lines)+2)
		out = append(out, lines[:end]...)
		out = append(out, "meta:", hiredLine)
		out = append(out, lines[end:]...)
		return strings.Join(out, "\n"), nil
	}

	// 计算 meta 子键区域边界：(metaIdx, blockEnd) 之间为 meta 的子键与多行值
	blockEnd := end
	for i := metaIdx + 1; i < end; i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue // 空行不终止块
		}
		if !strings.HasPrefix(lines[i], " ") && !strings.HasPrefix(lines[i], "\t") {
			blockEnd = i // 遇到下一个顶层键，meta 块结束
			break
		}
	}

	// 块内已有 hired: 行（两空格缩进层级）则原位替换
	for i := metaIdx + 1; i < blockEnd; i++ {
		indent := len(lines[i]) - len(strings.TrimLeft(lines[i], " "))
		if indent == 2 && strings.HasPrefix(strings.TrimSpace(lines[i]), "hired:") {
			lines[i] = hiredLine
			return strings.Join(lines, "\n"), nil
		}
	}

	// 块内末尾插入 hired 键
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:blockEnd]...)
	out = append(out, hiredLine)
	out = append(out, lines[blockEnd:]...)
	return strings.Join(out, "\n"), nil
}
