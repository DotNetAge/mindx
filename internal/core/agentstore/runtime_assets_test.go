package agentstore

import (
	"testing"
	"time"
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

// 预置 runtime/agents 目录化资源加载验证：预置 Agent 全部可加载，
// frontmatter 强类型字段就位，SOUL.md 承载行为规则，IDENTITY 正文留空走兜底。
// 注：预置数量随目录化改造收缩（10 个迁至 mindx-market，仅留 assistant），
// 断言不再写死数量，只要求非空（历史快照断言在数据迁移后会永久失败）。
func TestRuntimeAgentsLoad(t *testing.T) {
	testDeadline(t)
	store, report, err := Load("../../../runtime/agents")
	if err != nil {
		t.Fatalf("加载预置 agents 失败: %v", err)
	}
	for name, e := range report {
		t.Errorf("预置 agent %s 迁移报告含错误: %v", name, e)
	}
	agents := store.List()
	if len(agents) == 0 {
		t.Fatalf("预置 Agent 不应为空")
	}
	for _, a := range agents {
		if a.Meta.Name == "" || a.Meta.Role == "" || a.Meta.Description == "" {
			t.Errorf("%s: frontmatter 必填字段缺失", a.Meta.Name)
		}
		if a.Soul == "" {
			t.Errorf("%s: SOUL.md 为空（应承载核心准则）", a.Meta.Name)
		}
		if a.Meta.Introduction != "" {
			t.Errorf("%s: IDENTITY.md 正文应留空（走兜底角色定义）", a.Meta.Name)
		}
	}
	// 历史断言针对 architect（已迁至 mindx-market）；现预置仅剩 assistant，改存在性断言
	if store.Get("assistant") == nil {
		t.Error("assistant 未加载")
	}
}
