package svc

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/core/bundle"
)

// TestHandleMailDeliver_LocalOnly 直投口闸：非本机直通连接一律拒绝
// （无 client_id / 外接设备 / 闸未装配均走拒绝分支，防伪造发信人投信）。
// 收信合法通道是 daemon 轮询（mailLoopOnce），直投口仅本机直通可用。
func TestHandleMailDeliver_LocalOnly(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	_, err := d.handleMailDeliver(context.Background(), []byte(`{"uid":"u1","body":"hi"}`))
	if err == nil {
		t.Fatal("expected rejection for non-local-verified connection, got nil")
	}

	external := gateway.WithClientID(context.Background(), "external-1")
	if _, err := d.handleMailDeliver(external, []byte(`{"uid":"u1","body":"hi"}`)); err == nil {
		t.Fatal("未握手外接设备应被拒绝")
	}

	d.devGate = nil
	if _, err := d.handleMailDeliver(context.Background(), []byte(`{"uid":"u1","body":"hi"}`)); err == nil {
		t.Fatal("闸未装配应被拒绝")
	}
}

// TestHandleMailDeliver_LocalVerifiedFullPath 本机直通全链：
// 信封不完整报错 → 合法投递落档 → 二次投递同编号幂等去重。
func TestHandleMailDeliver_LocalVerifiedFullPath(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	token, err := d.loadOrCreateDaemonToken()
	if err != nil {
		t.Fatalf("loadOrCreateDaemonToken: %v", err)
	}
	res := d.devGate.Handshake("local-1", "dev", "测试机", "macos", token, "", nil)
	if res.Status != "paired" {
		t.Fatalf("本机令牌握手未直通: %+v", res)
	}
	ctx := gateway.WithClientID(context.Background(), "local-1")

	// 信封不完整
	if _, err := d.handleMailDeliver(ctx, []byte(`{"body":"hi"}`)); err == nil {
		t.Error("缺少信件编号应报错")
	}
	if _, err := d.handleMailDeliver(ctx, []byte(`{"uid":"u1","body":"   "}`)); err == nil {
		t.Error("正文空白应报错")
	}

	out, err := d.handleMailDeliver(ctx, []byte(`{"uid":"u1","org_slug":"acme","from_persona":2,"subject":"s","body":"正文"}`))
	if err != nil {
		t.Fatalf("合法投递被拒: %v", err)
	}
	if out.(map[string]string)["status"] != "ok" {
		t.Errorf("status = %v, want ok", out)
	}
	if !d.mailSeen("u1") {
		t.Error("投递后应已留档")
	}

	// 幂等：同编号二次投递去重，不重复落档
	out, err = d.handleMailDeliver(ctx, []byte(`{"uid":"u1","org_slug":"acme","from_persona":2,"subject":"s2","body":"正文2"}`))
	if err != nil {
		t.Fatalf("重复投递报错: %v", err)
	}
	if out.(map[string]string)["dedup"] != "1" {
		t.Errorf("重复投递应返回 dedup=1, got %v", out)
	}
	if got := d.mailFind("u1").Subject; got != "s" {
		t.Errorf("去重后不应覆盖首封信, subject = %q", got)
	}
}

// TestMailArchiveRoundtrip 留档闭环：保存 → 见过 → 清单按收信时间倒序 → 按编号定位。
func TestMailArchiveRoundtrip(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if list := d.mailList(); len(list) != 0 {
		t.Fatalf("空信箱应返回空清单, got %d", len(list))
	}

	err := d.mailSave(&LocalMail{UID: "m1", OrgSlug: "acme", FromPersona: 1, Subject: "旧", ReceivedAt: "2026-11-14T10:00:00+08:00"})
	if err != nil {
		t.Fatalf("mailSave: %v", err)
	}
	err = d.mailSave(&LocalMail{UID: "m2", OrgSlug: "acme", FromPersona: 2, Subject: "新", ReceivedAt: "2026-11-15T10:00:00+08:00"})
	if err != nil {
		t.Fatalf("mailSave: %v", err)
	}

	if !d.mailSeen("m1") || d.mailSeen("nope") {
		t.Error("mailSeen 去重判定不准")
	}

	list := d.mailList()
	if len(list) != 2 || list[0].UID != "m2" || list[1].UID != "m1" {
		t.Errorf("清单应按收信时间倒序, got %+v", list)
	}

	found := d.mailFind("m2")
	if found == nil || found.Subject != "新" {
		t.Errorf("mailFind 定位失败: %+v", found)
	}
	if d.mailFind("nope") != nil {
		t.Error("不存在的编号应返回 nil")
	}
}

// TestMailboxKeeper_EmptyRegistry 注册表为空时负责人为空串，来信只暂存不投递。
func TestMailboxKeeper_EmptyRegistry(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if keeper := d.mailboxKeeper(); keeper != "" {
		t.Errorf("空注册表负责人应为空, got %q", keeper)
	}
	d.wakeForMail("u1") // 不应 panic
}

// TestResolveOrgSlug_ExplicitShortCircuit 显式指定组织时直取，不查远端。
func TestResolveOrgSlug_ExplicitShortCircuit(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	slug, err := d.resolveOrgSlug("my-org")
	if err != nil || slug != "my-org" {
		t.Errorf("resolveOrgSlug = %q, %v; want my-org, nil", slug, err)
	}
}

// TestFindMemberCard 成员名片精确匹配。
func TestFindMemberCard(t *testing.T) {
	cards := []bundle.OrgMemberCard{
		{DisplayName: "小秘", PersonaID: 1},
		{DisplayName: "阿Ray", PersonaID: 2},
	}
	if c := findMemberCard(cards, "阿Ray"); c == nil || c.PersonaID != 2 {
		t.Errorf("按显示名匹配失败: %+v", c)
	}
	if c := findMemberCard(cards, "不存在"); c != nil {
		t.Errorf("未命中应返回 nil, got %+v", c)
	}
}

// TestHandleMailSend_ValidationErrors 发信参数校验全部挡在网络调用之前。
func TestHandleMailSend_ValidationErrors(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	cases := []struct {
		params string
		what   string
	}{
		{`{}`, "缺收信人"},
		{`{"to":"小秘"}`, "缺主题"},
		{`{"to":"小秘","subject":"s"}`, "缺正文"},
		{`{"to":"小秘","subject":"s","body":"b","dead_line":"2026/11/15"}`, "截止时刻非法格式"},
	}
	for _, c := range cases {
		if _, err := d.handleMailSend(context.Background(), []byte(c.params)); err == nil {
			t.Errorf("%s 应报错", c.what)
		}
	}
}

// TestHandleMailReply_ValidationErrors 回信校验挡在网络调用之前：
// 正文必填、原信必须已留档。
func TestHandleMailReply_ValidationErrors(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleMailReply(context.Background(), []byte(`{"uid":"m1","body":"  "}`)); err == nil {
		t.Error("空正文应报错")
	}
	if _, err := d.handleMailReply(context.Background(), []byte(`{"uid":"nope","body":"内容"}`)); err == nil {
		t.Error("无原信应报错")
	}
}

// TestHandleMailInbox_ReturnsArchived 收件箱返回留档清单。
func TestHandleMailInbox_ReturnsArchived(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if err := d.mailSave(&LocalMail{UID: "m1", Subject: "hello", ReceivedAt: "2026-11-15T10:00:00+08:00"}); err != nil {
		t.Fatalf("mailSave: %v", err)
	}
	out, err := d.handleMailInbox(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleMailInbox: %v", err)
	}
	mails := out.(map[string]any)["mails"].([]LocalMail)
	if len(mails) != 1 || mails[0].UID != "m1" {
		t.Errorf("收件箱内容不符: %+v", mails)
	}
}

// TestWarnOrgBlocked_LogLevelsOnce 403 记 Warn 且每组织只提示一次，其余 Debug 不刷屏。
func TestWarnOrgBlocked_LogLevelsOnce(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 非 403：Debug 分支（不触发去重登记）
	d.warnOrgBlocked("acme", "收件箱拉取", context.DeadlineExceeded)

	// 403：首次 Warn 登记，二次静默
	err403 := errors.New("HTTP 403: subscription required")
	d.warnOrgBlocked("acme", "收件箱拉取", err403)
	d.warnOrgBlocked("acme", "收件箱拉取", err403) // 应被 subscriptionWarned 挡下，不 panic
}

// TestMemberNameOf_FallbackOnLookupFailure 名片查询失败回退编号写法。
func TestMemberNameOf_FallbackOnLookupFailure(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// app.Market 未登录/查询失败 → 回退「成员 #N」
	if got := d.memberNameOf("acme", 7); got == "" {
		t.Error("回退名不应为空串")
	}
}
