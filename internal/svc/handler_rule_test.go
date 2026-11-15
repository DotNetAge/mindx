package svc

import (
	"context"
	"testing"

	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/DotNetAge/mindx/pkg/rules"
)

func ruleTestDaemon(t *testing.T) *Daemon {
	t.Helper()
	d, cleanup := newTestDaemon(t)
	t.Cleanup(cleanup)
	return d
}

func TestHandleRuleList(t *testing.T) {
	d := ruleTestDaemon(t)

	result, err := d.handleRuleList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleRuleList error = %v", err)
	}
	m := result.(map[string]interface{})
	if m["count"].(int) != 0 || len(m["rules"].([]rules.Rule)) != 0 {
		t.Errorf("fresh registry = %#v, want empty", m)
	}
}

func TestHandleRuleCreateAndGet(t *testing.T) {
	d := ruleTestDaemon(t)
	ctx := context.Background()

	// 参数校验
	if _, err := d.handleRuleCreate(ctx, mustParams(t, rpc.RuleCreateParams{Intro: "no id"})); err == nil {
		t.Error("empty id: expected error")
	}
	if _, err := d.handleRuleCreate(ctx, mustParams(t, rpc.RuleCreateParams{ID: "r1"})); err == nil {
		t.Error("empty intro: expected error")
	}

	// 创建（scope 缺省落 global）
	if _, err := d.handleRuleCreate(ctx, mustParams(t, rpc.RuleCreateParams{ID: "r1", Intro: "第一条规则", Priority: 5})); err != nil {
		t.Fatalf("create error = %v", err)
	}

	// get
	result, err := d.handleRuleGet(ctx, mustParams(t, rpc.RuleGetParams{ID: "r1"}))
	if err != nil {
		t.Fatalf("get error = %v", err)
	}
	m := result.(map[string]interface{})
	if m["found"] != true {
		t.Fatalf("found = %v", m["found"])
	}
	rule := m["rule"].(*rules.Rule)
	if rule.Intro != "第一条规则" || rule.Scope != rules.ScopeGlobal || rule.Priority != 5 {
		t.Errorf("rule = %+v", rule)
	}

	// get 不存在 → found=false
	result, _ = d.handleRuleGet(ctx, mustParams(t, rpc.RuleGetParams{ID: "ghost"}))
	if result.(map[string]interface{})["found"] != false {
		t.Error("missing rule should report found=false")
	}
	// get 缺 id
	if _, err := d.handleRuleGet(ctx, mustParams(t, rpc.RuleGetParams{})); err == nil {
		t.Error("empty id: expected error")
	}

	// 重复创建同 ID → upsert 语义（覆盖不报错）
	if _, err := d.handleRuleCreate(ctx, mustParams(t, rpc.RuleCreateParams{ID: "r1", Intro: "dup"})); err != nil {
		t.Errorf("duplicate id upsert: unexpected error %v", err)
	}

	// list 计数
	result, _ = d.handleRuleList(ctx, nil)
	if m := result.(map[string]interface{}); m["count"].(int) != 1 {
		t.Errorf("count = %v, want 1", m["count"])
	}
}

func TestHandleRuleUpdate(t *testing.T) {
	d := ruleTestDaemon(t)
	ctx := context.Background()

	intro := "原始简介"
	prio := 1
	if _, err := d.handleRuleCreate(ctx, mustParams(t, rpc.RuleCreateParams{ID: "r1", Intro: intro, Priority: prio})); err != nil {
		t.Fatalf("create error = %v", err)
	}

	// 更新不存在的规则
	if _, err := d.handleRuleUpdate(ctx, mustParams(t, rpc.RuleUpdateParams{ID: "ghost"})); err == nil {
		t.Error("update missing: expected error")
	}
	// 缺 id
	if _, err := d.handleRuleUpdate(ctx, mustParams(t, rpc.RuleUpdateParams{})); err == nil {
		t.Error("empty id: expected error")
	}

	// 部分更新：只改 intro 与 enabled，其余保持
	newIntro := "更新后简介"
	enabled := true
	if _, err := d.handleRuleUpdate(ctx, mustParams(t, rpc.RuleUpdateParams{ID: "r1", Intro: &newIntro, Enabled: &enabled})); err != nil {
		t.Fatalf("update error = %v", err)
	}
	result, _ := d.handleRuleGet(ctx, mustParams(t, rpc.RuleGetParams{ID: "r1"}))
	rule := result.(map[string]interface{})["rule"].(*rules.Rule)
	if rule.Intro != newIntro || rule.Priority != prio || !rule.Enabled {
		t.Errorf("rule = %+v, want intro updated / priority kept / enabled=true", rule)
	}

	// scope 更新
	scope := string(rules.ScopeLocal)
	if _, err := d.handleRuleUpdate(ctx, mustParams(t, rpc.RuleUpdateParams{ID: "r1", Scope: &scope})); err != nil {
		t.Fatalf("scope update error = %v", err)
	}
	result, _ = d.handleRuleGet(ctx, mustParams(t, rpc.RuleGetParams{ID: "r1"}))
	if got := result.(map[string]interface{})["rule"].(*rules.Rule).Scope; got != rules.ScopeLocal {
		t.Errorf("scope = %v, want agent", got)
	}
}

func TestHandleRuleDelete(t *testing.T) {
	d := ruleTestDaemon(t)
	ctx := context.Background()

	if _, err := d.handleRuleCreate(ctx, mustParams(t, rpc.RuleCreateParams{ID: "r1", Intro: "x"})); err != nil {
		t.Fatalf("create error = %v", err)
	}

	// 校验分支
	if _, err := d.handleRuleDelete(ctx, mustParams(t, rpc.RuleDeleteParams{})); err == nil {
		t.Error("empty id: expected error")
	}
	if _, err := d.handleRuleDelete(ctx, mustParams(t, rpc.RuleDeleteParams{ID: "ghost"})); err == nil {
		t.Error("delete missing: expected error")
	}

	result, err := d.handleRuleDelete(ctx, mustParams(t, rpc.RuleDeleteParams{ID: "r1"}))
	if err != nil {
		t.Fatalf("delete error = %v", err)
	}
	if m := result.(map[string]any); m["deleted_id"] != "r1" {
		t.Errorf("result = %#v", m)
	}

	// 删除后 get found=false
	result, _ = d.handleRuleGet(ctx, mustParams(t, rpc.RuleGetParams{ID: "r1"}))
	if result.(map[string]interface{})["found"] != false {
		t.Error("rule should be gone after delete")
	}
}
