package svc

import (
	"context"
	"testing"

	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// teamTestDaemon 注册一个可充当负责人的 Agent，返回 daemon。
func teamTestDaemon(t *testing.T) *Daemon {
	t.Helper()
	d, cleanup := newTestDaemon(t)
	t.Cleanup(cleanup)

	leader := &agentstore.Agent{Meta: agentstore.AgentMeta{Name: "leader-a"}}
	if err := d.app.Agents().Save(leader); err != nil {
		t.Fatalf("save leader agent: %v", err)
	}
	return d
}

func TestHandleTeamList_Empty(t *testing.T) {
	d := teamTestDaemon(t)

	result, err := d.handleTeamList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleTeamList error = %v", err)
	}
	if list := result.([]teamDetailResult); len(list) != 0 {
		t.Errorf("list = %#v, want empty", list)
	}
}

func TestHandleTeamUpdateCreateAndGet(t *testing.T) {
	d := teamTestDaemon(t)
	ctx := context.Background()

	// 参数校验
	if _, err := d.handleTeamUpdate(ctx, mustParams(t, rpc.TeamUpdateParams{})); err == nil {
		t.Error("empty name: expected error")
	}
	// 新建必须任命负责人
	if _, err := d.handleTeamUpdate(ctx, mustParams(t, rpc.TeamUpdateParams{Name: "alpha"})); err == nil {
		t.Error("missing leader on create: expected error")
	}
	// 负责人必须是存在的 Agent
	if _, err := d.handleTeamUpdate(ctx, mustParams(t, rpc.TeamUpdateParams{Name: "alpha", Leader: strPtr("ghost-agent")})); err == nil {
		t.Error("unknown leader: expected error")
	}

	// 创建团队
	result, err := d.handleTeamUpdate(ctx, mustParams(t, rpc.TeamUpdateParams{
		Name:        "alpha",
		Leader:      strPtr("leader-a"),
		NickName:    strPtr("甲队"),
		Description: strPtr("描述"),
		Members:     []string{"leader-a", "b"},
		Icon:        strPtr("rocket"),
		Category:    strPtr("dev"),
		Profile:     strPtr(" 团队人设正文 "),
	}))
	if err != nil {
		t.Fatalf("create error = %v", err)
	}
	if m := result.(map[string]string); m["status"] != "ok" || m["team_name"] != "alpha" {
		t.Errorf("result = %#v", m)
	}

	// get：Profile 修剪后内联
	detail, err := d.handleTeamGet(ctx, mustParams(t, rpc.TeamGetParams{Name: "alpha"}))
	if err != nil {
		t.Fatalf("get error = %v", err)
	}
	got := detail.(teamDetailResult)
	if got.Name != "alpha" || got.Leader != "leader-a" || got.Profile != "团队人设正文" {
		t.Errorf("detail = %+v profile=%q", got.TeamMeta, got.Profile)
	}

	// get 校验
	if _, err := d.handleTeamGet(ctx, mustParams(t, rpc.TeamGetParams{})); err == nil {
		t.Error("empty name: expected error")
	}
	if _, err := d.handleTeamGet(ctx, mustParams(t, rpc.TeamGetParams{Name: "ghost"})); err == nil {
		t.Error("missing team: expected error")
	}

	// list：包含 Profile 投影
	result2, err := d.handleTeamList(ctx, nil)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	list := result2.([]teamDetailResult)
	if len(list) != 1 || list[0].Leader != "leader-a" || list[0].Profile != "团队人设正文" {
		t.Errorf("list = %#v", list)
	}
}

func TestHandleTeamUpdatePartial(t *testing.T) {
	d := teamTestDaemon(t)
	ctx := context.Background()

	if _, err := d.handleTeamUpdate(ctx, mustParams(t, rpc.TeamUpdateParams{
		Name: "alpha", Leader: strPtr("leader-a"), NickName: strPtr("旧昵称"),
	})); err != nil {
		t.Fatalf("create error = %v", err)
	}

	// 部分更新：只改描述，负责人沿用
	desc := "新描述"
	if _, err := d.handleTeamUpdate(ctx, mustParams(t, rpc.TeamUpdateParams{
		Name: "alpha", Description: &desc,
	})); err != nil {
		t.Fatalf("partial update error = %v", err)
	}
	detail, _ := d.handleTeamGet(ctx, mustParams(t, rpc.TeamGetParams{Name: "alpha"}))
	got := detail.(teamDetailResult)
	if got.Description != desc || got.Leader != "leader-a" || got.NickName != "旧昵称" {
		t.Errorf("detail = %+v", got)
	}

	// 换负责人（成员列表不动）
	if _, err := d.handleTeamUpdate(ctx, mustParams(t, rpc.TeamUpdateParams{
		Name: "alpha", Leader: strPtr("  leader-b  "),
	})); err == nil {
		t.Error("unknown leader-b: expected error")
	}
}

func TestHandleTeamHireFireAndDelete(t *testing.T) {
	d := teamTestDaemon(t)
	ctx := context.Background()

	if _, err := d.handleTeamUpdate(ctx, mustParams(t, rpc.TeamUpdateParams{
		Name: "alpha", Leader: strPtr("leader-a"),
	})); err != nil {
		t.Fatalf("create error = %v", err)
	}

	// hire/fire 校验与效果
	if _, err := d.handleTeamHire(ctx, mustParams(t, rpc.TeamHireParams{})); err == nil {
		t.Error("empty name: expected error")
	}
	result, err := d.handleTeamHire(ctx, mustParams(t, rpc.TeamHireParams{Name: "alpha"}))
	if err != nil {
		t.Fatalf("hire error = %v", err)
	}
	if m := result.(map[string]string); m["message"] != "team hired" {
		t.Errorf("hire message = %q", m["message"])
	}
	result, err = d.handleTeamFire(ctx, mustParams(t, rpc.TeamHireParams{Name: "alpha"}))
	if err != nil {
		t.Fatalf("fire error = %v", err)
	}
	if m := result.(map[string]string); m["message"] != "team fired" {
		t.Errorf("fire message = %q", m["message"])
	}

	// delete
	if _, err := d.handleTeamDelete(ctx, mustParams(t, rpc.TeamDeleteParams{})); err == nil {
		t.Error("empty name: expected error")
	}
	result, err = d.handleTeamDelete(ctx, mustParams(t, rpc.TeamDeleteParams{Name: "alpha"}))
	if err != nil {
		t.Fatalf("delete error = %v", err)
	}
	if m := result.(map[string]string); m["status"] != "ok" {
		t.Errorf("delete result = %#v", m)
	}
	// 删除后 get 报错
	if _, err := d.handleTeamGet(ctx, mustParams(t, rpc.TeamGetParams{Name: "alpha"})); err == nil {
		t.Error("team should be gone after delete")
	}
}

func strPtr(s string) *string { return &s }
