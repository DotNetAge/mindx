package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/mindx/internal/core/agentstore"
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

// mustWriteAgentDir 以目录格式写入一个最小 Agent（IDENTITY.md），供各测试复用。
func mustWriteAgentDir(t *testing.T, agentsDir, name, extraFrontmatter, soul string) {
	t.Helper()
	dir := filepath.Join(agentsDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("创建 %s 失败: %v", dir, err)
	}
	content := "---\nname: " + name + "\n" + extraFrontmatter + "---\n\n" + soul + "\n"
	if err := os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte(content), 0644); err != nil {
		t.Fatalf("写入 %s 失败: %v", dir, err)
	}
}

func TestAgentIsHired(t *testing.T) {
	testDeadline(t)

	cases := []struct {
		name  string
		agent *agentstore.Agent
		want  bool
	}{
		{"nil Agent 视为未雇佣", nil, false},
		{"缺省视为未雇佣", &agentstore.Agent{}, false},
		{"hired true", &agentstore.Agent{Meta: agentstore.AgentMeta{Hired: true}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AgentIsHired(c.agent); got != c.want {
				t.Fatalf("AgentIsHired(%v) = %v, 期望 %v", c.agent, got, c.want)
			}
		})
	}
}

func TestAgentCategory(t *testing.T) {
	testDeadline(t)

	cases := []struct {
		name  string
		agent *agentstore.Agent
		want  string
	}{
		{"nil Agent 返回空串", nil, ""},
		{"缺省返回空串", &agentstore.Agent{}, ""},
		{"一级字段直读", &agentstore.Agent{Meta: agentstore.AgentMeta{Category: "产品研发"}}, "产品研发"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AgentCategory(c.agent); got != c.want {
				t.Fatalf("AgentCategory(%v) = %q, 期望 %q", c.agent, got, c.want)
			}
		})
	}
}

func TestHiredAgents(t *testing.T) {
	testDeadline(t)

	dir := t.TempDir()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		mustWriteAgentDir(t, dir, name, "", "正文")
	}
	store, _, err := agentstore.Load(dir)
	if err != nil {
		t.Fatalf("agentstore.Load 失败: %v", err)
	}

	if err := SetAgentHired(store, "alpha", true); err != nil {
		t.Fatalf("SetAgentHired(alpha) 失败: %v", err)
	}
	if err := SetAgentHired(store, "beta", true); err != nil {
		t.Fatalf("SetAgentHired(beta) 失败: %v", err)
	}
	// gamma 保持未雇佣

	hired := store.Hired()
	if len(hired) != 2 {
		t.Fatalf("雇佣视图应含 2 个 Agent, 实际 %d: %v", len(hired), hired)
	}
	for _, a := range hired {
		if a.Meta.Name == "gamma" {
			t.Fatalf("未雇佣的 gamma 不应出现在雇佣视图中")
		}
	}
}

func TestSetAgentHiredEndToEnd(t *testing.T) {
	testDeadline(t)

	// 预置样例：带 exclude_tools 与复杂正文的 Agent 目录（模拟 agents/ 真实形态）
	dir := t.TempDir()
	identity := "---\nname: architect\nrole: 软件架构师\ndescription: 架构师\nexclude_tools:\n  - Sleep\n  - PowerShell\n---\n\n软件架构师，负责分层解耦。\n"
	agentDir := filepath.Join(dir, "architect")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "IDENTITY.md"), []byte(identity), 0644); err != nil {
		t.Fatalf("写入 IDENTITY.md 失败: %v", err)
	}
	soul := "## 核心准则\n\n分层解耦。\n"
	if err := os.WriteFile(filepath.Join(agentDir, "SOUL.md"), []byte(soul), 0644); err != nil {
		t.Fatalf("写入 SOUL.md 失败: %v", err)
	}

	store, _, err := agentstore.Load(dir)
	if err != nil {
		t.Fatalf("agentstore.Load 失败: %v", err)
	}

	// 雇佣：文件写入 hired: true、内存同步、exclude_tools 与 SOUL 不丢失
	if err := SetAgentHired(store, "architect", true); err != nil {
		t.Fatalf("SetAgentHired(true) 失败: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(agentDir, "IDENTITY.md"))
	if err != nil {
		t.Fatalf("读取 IDENTITY.md 失败: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "hired: true") {
		t.Fatalf("文件应包含 hired: true:\n%s", text)
	}
	if !strings.Contains(text, "- PowerShell") {
		t.Fatalf("全量序列化不得丢失 exclude_tools:\n%s", text)
	}
	if !strings.Contains(text, "软件架构师") {
		t.Fatalf("身份正文不得丢失:\n%s", text)
	}
	soulData, err := os.ReadFile(filepath.Join(agentDir, "SOUL.md"))
	if err != nil {
		t.Fatalf("读取 SOUL.md 失败: %v", err)
	}
	if !strings.Contains(string(soulData), "## 核心准则") {
		t.Fatalf("SOUL.md 不得丢失:\n%s", string(soulData))
	}
	if !AgentIsHired(store.Get("architect")) {
		t.Fatalf("内存注册表应同步为已雇佣")
	}
	if hired := store.Hired(); len(hired) != 1 {
		t.Fatalf("雇佣视图应含 1 个 Agent, 实际 %d", len(hired))
	}

	// 解雇：文件写入 hired: false、内存同步
	if err := SetAgentHired(store, "architect", false); err != nil {
		t.Fatalf("SetAgentHired(false) 失败: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(agentDir, "IDENTITY.md"))
	if err != nil {
		t.Fatalf("读取 IDENTITY.md 失败: %v", err)
	}
	if strings.Contains(string(data), "hired: true") {
		t.Fatalf("解雇后文件不应残留 hired: true:\n%s", string(data))
	}
	if AgentIsHired(store.Get("architect")) {
		t.Fatalf("内存注册表应同步为未雇佣")
	}

	// 不存在的 Agent 应报错
	if err := SetAgentHired(store, "ghost", true); err == nil {
		t.Fatalf("雇佣不存在的 Agent 应返回错误")
	}
}
