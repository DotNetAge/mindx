package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/goharness/config"
)

// testDeadline 为测试函数附加 10 秒超时看门狗（项目测试硬性要求）。
func testDeadline(t *testing.T) {
	t.Helper()
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("测试超时（超过 10 秒）")
		}
	}()
}

func TestAgentMetaIsHired(t *testing.T) {
	testDeadline(t)

	cases := []struct {
		name string
		meta map[string]any
		want bool
	}{
		{"缺省视为未雇佣", nil, false},
		{"无 hired 键", map[string]any{"domains": []any{"coding"}}, false},
		{"bool true", map[string]any{"hired": true}, true},
		{"bool false", map[string]any{"hired": false}, false},
		{"字符串 true", map[string]any{"hired": "true"}, true},
		{"字符串 TRUE 大小写不敏感", map[string]any{"hired": "TRUE"}, true},
		{"字符串 false", map[string]any{"hired": "false"}, false},
		{"字符串乱值视为未雇佣", map[string]any{"hired": "yes"}, false},
		{"数字类型视为未雇佣", map[string]any{"hired": 1}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AgentMetaIsHired(c.meta); got != c.want {
				t.Fatalf("AgentMetaIsHired(%v) = %v, 期望 %v", c.meta, got, c.want)
			}
		})
	}
}

func TestAgentMetaDomains(t *testing.T) {
	testDeadline(t)

	cases := []struct {
		name string
		meta map[string]any
		want []string
	}{
		{"缺省返回 nil", nil, nil},
		{"无 domains 键", map[string]any{"hired": true}, nil},
		{"YAML 解析产物 []any", map[string]any{"domains": []any{"Coding", " writing "}}, []string{"coding", "writing"}},
		{"[]string 形态", map[string]any{"domains": []string{"办公"}}, []string{"办公"}},
		{"去重", map[string]any{"domains": []any{"coding", "Coding"}}, []string{"coding"}},
		{"忽略非字符串与空项", map[string]any{"domains": []any{"coding", 42, "", nil}}, []string{"coding"}},
		{"非列表值返回 nil", map[string]any{"domains": "coding"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AgentMetaDomains(c.meta)
			if len(got) != len(c.want) {
				t.Fatalf("AgentMetaDomains(%v) = %v, 期望 %v", c.meta, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("AgentMetaDomains(%v)[%d] = %q, 期望 %q", c.meta, i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestHiredAgentsOf(t *testing.T) {
	testDeadline(t)

	dir := t.TempDir()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		content := "---\nname: " + name + "\n---\n正文\n"
		if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0644); err != nil {
			t.Fatalf("写入 %s 失败: %v", name, err)
		}
	}
	reg, err := config.LoadAgentsFrom(dir)
	if err != nil {
		t.Fatalf("LoadAgentsFrom 失败: %v", err)
	}

	if err := SetAgentHired(dir, "alpha", reg, true); err != nil {
		t.Fatalf("SetAgentHired(alpha) 失败: %v", err)
	}
	if err := SetAgentHired(dir, "beta", reg, true); err != nil {
		t.Fatalf("SetAgentHired(beta) 失败: %v", err)
	}
	// gamma 保持未雇佣

	hired := HiredAgentsOf(reg)
	if len(hired) != 2 {
		t.Fatalf("雇佣视图应含 2 个 Agent, 实际 %d: %v", len(hired), hired)
	}
	for _, a := range hired {
		if a.Name == "gamma" {
			t.Fatalf("未雇佣的 gamma 不应出现在雇佣视图中")
		}
	}
}

func TestSetFrontmatterHired(t *testing.T) {
	testDeadline(t)

	t.Run("无 meta 块时在 frontmatter 末尾插入", func(t *testing.T) {
		content := "---\nname: writer\nrole: Writer\n---\n\n正文内容\n"
		got, err := setFrontmatterHired(content, true)
		if err != nil {
			t.Fatalf("setFrontmatterHired 失败: %v", err)
		}
		if !strings.Contains(got, "meta:\n  hired: true\n---") {
			t.Fatalf("应在 frontmatter 末尾插入 meta 块:\n%s", got)
		}
	})

	t.Run("已有 meta 块时追加 hired", func(t *testing.T) {
		content := "---\nname: writer\nmeta:\n  domains:\n    - coding\n---\n\n正文\n"
		got, err := setFrontmatterHired(content, true)
		if err != nil {
			t.Fatalf("setFrontmatterHired 失败: %v", err)
		}
		if !strings.Contains(got, "    - coding\n  hired: true\n---") {
			t.Fatalf("应插入到 meta 块末尾:\n%s", got)
		}
	})

	t.Run("已有 hired 时原位替换", func(t *testing.T) {
		content := "---\nname: writer\nmeta:\n  hired: true\n  domains:\n    - coding\n---\n\n正文\n"
		got, err := setFrontmatterHired(content, false)
		if err != nil {
			t.Fatalf("setFrontmatterHired 失败: %v", err)
		}
		if !strings.Contains(got, "  hired: false") {
			t.Fatalf("应替换 hired 值:\n%s", got)
		}
		if strings.Contains(got, "  hired: true") {
			t.Fatalf("不应残留旧值 true:\n%s", got)
		}
	})

	t.Run("多行块标量后插入不破坏结构", func(t *testing.T) {
		content := "---\nname: pm\nmeta:\n  名称: 产品经理\n  描述: |\n    产品规划专家。\n---\n\n正文\n"
		got, err := setFrontmatterHired(content, true)
		if err != nil {
			t.Fatalf("setFrontmatterHired 失败: %v", err)
		}
		if !strings.Contains(got, "    产品规划专家。\n  hired: true\n---") {
			t.Fatalf("应插入在多行块标量之后:\n%s", got)
		}
	})

	t.Run("frontmatter 缺失时报错", func(t *testing.T) {
		if _, err := setFrontmatterHired("普通文本", true); err == nil {
			t.Fatalf("缺少 frontmatter 应返回错误")
		}
	})
}

func TestSetAgentHiredEndToEnd(t *testing.T) {
	testDeadline(t)

	// 预置样例：带 exclude_tools 与复杂正文的 Agent 文件（模拟 runtime/agents 真实形态）
	content := "---\nname: architect\nrole: 软件架构师\ndescription: 架构师\nexclude_tools:\n  - Sleep\n  - PowerShell\n---\n\n## 核心准则\n\n分层解耦。\n"
	dir := t.TempDir()
	filePath := filepath.Join(dir, "architect.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	reg, err := config.LoadAgentsFrom(dir)
	if err != nil {
		t.Fatalf("LoadAgentsFrom 失败: %v", err)
	}

	// 雇佣：文件写入 hired: true、内存同步、exclude_tools 不丢失
	if err := SetAgentHired(dir, "architect", reg, true); err != nil {
		t.Fatalf("SetAgentHired(true) 失败: %v", err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "hired: true") {
		t.Fatalf("文件应包含 hired: true:\n%s", text)
	}
	if !strings.Contains(text, "- PowerShell") {
		t.Fatalf("文本级修改不得丢失 exclude_tools:\n%s", text)
	}
	if !strings.Contains(text, "## 核心准则") {
		t.Fatalf("正文内容不得丢失:\n%s", text)
	}
	cfg := reg.Get("architect")
	if cfg == nil || !AgentIsHired(cfg) {
		t.Fatalf("内存注册表应同步为已雇佣")
	}
	if hired := HiredAgentsOf(reg); len(hired) != 1 {
		t.Fatalf("雇佣视图应含 1 个 Agent, 实际 %d", len(hired))
	}

	// 解雇：文件写入 hired: false、内存同步
	if err := SetAgentHired(dir, "architect", reg, false); err != nil {
		t.Fatalf("SetAgentHired(false) 失败: %v", err)
	}
	data, err = os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if strings.Contains(string(data), "hired: true") || !strings.Contains(string(data), "hired: false") {
		t.Fatalf("解雇后文件应包含 hired: false:\n%s", string(data))
	}
	if AgentIsHired(reg.Get("architect")) {
		t.Fatalf("内存注册表应同步为未雇佣")
	}

	// 不存在的 Agent 应报错
	if err := SetAgentHired(dir, "ghost", reg, true); err == nil {
		t.Fatalf("雇佣不存在的 Agent 应返回错误")
	}
}
