package devgate

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	goharnesslogging "github.com/DotNetAge/goharness/logging"
)

// memStore 内存持久化桩（模拟 CredentialStore 的最小行为）
type memStore struct{ data map[string]string }

func newMemStore() *memStore { return &memStore{data: map[string]string{}} }

func (s *memStore) Get(k string) (string, error) {
	v, ok := s.data[k]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}
func (s *memStore) Set(k, v string) error { s.data[k] = v; return nil }

// newTestManager 测试用管理器（本机令牌固定为 local-secret）
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	mgr, err := NewManager(Config{
		Store:      newMemStore(),
		LocalToken: func() (string, error) { return "local-secret", nil },
		Logger:     goharnesslogging.NewNopLogger(),
	})
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	return mgr
}

// TestGeneratePairCode 生成即有效：6 位数字、约 5 分钟时效、新码替换旧码。
func TestGeneratePairCode(t *testing.T) {
	mgr := newTestManager(t)
	code1, exp1, err := mgr.GeneratePairCode()
	if err != nil {
		t.Fatalf("生成配对码失败: %v", err)
	}
	if len(code1) != 6 || strings.Trim(code1, "0123456789") != "" {
		t.Fatalf("配对码应为 6 位数字，得到 %q", code1)
	}
	if d := time.Until(exp1); d < pairCodeTTL-time.Minute || d > pairCodeTTL+time.Minute {
		t.Fatalf("有效期应约 5 分钟，得到 %v", d)
	}
	code2, _, err := mgr.GeneratePairCode()
	if err != nil {
		t.Fatalf("再次生成失败: %v", err)
	}
	if !mgr.ValidPairCode(code2) {
		t.Fatal("新码应有效")
	}
	if mgr.ValidPairCode(code1) {
		t.Fatal("旧码应被新码替换失效")
	}
}

// TestValidPairCodeAttempts 错误尝试计数：连错 5 次码作废（正确码也不再通过）。
func TestValidPairCodeAttempts(t *testing.T) {
	mgr := newTestManager(t)
	code, _, err := mgr.GeneratePairCode()
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	for i := 0; i < pairCodeMaxAttempts-1; i++ {
		if mgr.ValidPairCode("999999") {
			t.Fatal("错误码不应通过")
		}
	}
	if !mgr.ValidPairCode(code) {
		t.Fatal("未达上限前正确码应仍有效")
	}
	_ = mgr.ValidPairCode("999999") // 第 5 次错误：作废
	if mgr.ValidPairCode(code) {
		t.Fatal("达到尝试上限后码应作废")
	}
}

// TestHandshakePairCode 短码握手闭环：消费→paired+token→会话登记；
// 一次性（同码第二次 denied+人话提示）；签发的 token 进设备表。
func TestHandshakePairCode(t *testing.T) {
	mgr := newTestManager(t)
	code, _, err := mgr.GeneratePairCode()
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	res := mgr.Handshake("conn-1", "dev-A", "积木君的 MacBook", "macos", "", code)
	if res.Status != "paired" || res.Token == "" {
		t.Fatalf("短码握手应 paired 并下发 token，得到 %+v", res)
	}
	if !mgr.Authorized("conn-1") {
		t.Fatal("握手成功后连接应已登记")
	}
	if !mgr.DeviceTokenExists(res.Token) {
		t.Fatal("签发的 token 应存在于设备表")
	}
	if mgr.DeviceTokenExists("deadbeef") || mgr.DeviceTokenExists("") {
		t.Fatal("未知/空 token 不应命中设备表")
	}
	res2 := mgr.Handshake("conn-2", "dev-B", "另一台", "macos", "", code)
	if res2.Status != "denied" || res2.Message == "" {
		t.Fatalf("配对码应一次性消费，得到 %+v", res2)
	}
	// 设备重连凭 token 免配对
	res3 := mgr.Handshake("conn-3", "dev-A", "积木君的 MacBook", "macos", res.Token, "")
	if res3.Status != "paired" {
		t.Fatalf("设备令牌重连应免配对，得到 %+v", res3)
	}
}

// TestHandshakePairCodeExpired 过期码：denied 且提示人话。
func TestHandshakePairCodeExpired(t *testing.T) {
	mgr := newTestManager(t)
	code, _, err := mgr.GeneratePairCode()
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	mgr.mu.Lock()
	mgr.pairCode.expiresAt = time.Now().Add(-time.Second)
	mgr.mu.Unlock()
	res := mgr.Handshake("conn-1", "dev-A", "积木君的 MacBook", "macos", "", code)
	if res.Status != "denied" || res.Message == "" {
		t.Fatalf("过期码应 denied 并带提示，得到 %+v", res)
	}
}

// TestHandshakePairCodeClosed 总闸关闭：短码同样不放行。
func TestHandshakePairCodeClosed(t *testing.T) {
	mgr := newTestManager(t)
	if err := mgr.SetAllowed(false); err != nil {
		t.Fatalf("关闭开关失败: %v", err)
	}
	code, _, err := mgr.GeneratePairCode()
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	res := mgr.Handshake("conn-1", "dev-A", "积木君的 MacBook", "macos", "", code)
	if res.Status != "closed" {
		t.Fatalf("总闸关闭时短码握手应 closed，得到 %+v", res)
	}
}

// TestHandshakeLocalTokenBypass 本机令牌直通不受开关限制（既有语义回归）。
func TestHandshakeLocalTokenBypass(t *testing.T) {
	mgr := newTestManager(t)
	if err := mgr.SetAllowed(false); err != nil {
		t.Fatalf("关闭开关失败: %v", err)
	}
	res := mgr.Handshake("conn-local", "cli", "本机命令行", "cli", "local-secret", "")
	if res.Status != "paired" {
		t.Fatalf("本机令牌应直通，得到 %+v", res)
	}
}

// TestLocalVerified 本机直通与外接设备的区分：本机令牌握手 → true；
// 配对码/设备令牌握手 → false（本机专属凭据不下发给外接设备）。
func TestLocalVerified(t *testing.T) {
	mgr := newTestManager(t)
	code, _, err := mgr.GeneratePairCode()
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	if res := mgr.Handshake("conn-ext", "dev-A", "积木君的 MacBook", "macos", "", code); res.Status != "paired" {
		t.Fatalf("短码握手应 paired，得到 %+v", res)
	}
	if mgr.LocalVerified("conn-ext") {
		t.Fatal("外接设备（配对码路径）不应被视为本机直通")
	}
	if res := mgr.Handshake("conn-local", "cli", "本机命令行", "cli", "local-secret", ""); res.Status != "paired" {
		t.Fatalf("本机令牌应直通，得到 %+v", res)
	}
	if !mgr.LocalVerified("conn-local") {
		t.Fatal("本机令牌握手应被视为本机直通")
	}
	if mgr.LocalVerified("conn-none") || mgr.LocalVerified("") {
		t.Fatal("未握手/空连接不应被视为本机直通")
	}
}

// TestDenyAndBlock 拉黑闭环：拒绝并拉黑后同设备重试静默 denied（不再弹窗）；
// 解除后恢复「申请 → 弹窗审批」正常流程；黑名单随重启加载（持久化）。
func TestDenyAndBlock(t *testing.T) {
	store := newMemStore()
	mgr, err := NewManager(Config{
		Store:      store,
		LocalToken: func() (string, error) { return "local-secret", nil },
		Logger:     goharnesslogging.NewNopLogger(),
	})
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	// A 君首次申请 → pending 弹窗
	res := mgr.Handshake("conn-a", "dev-rude", "A 君的电脑", "macos", "", "")
	if res.Status != "pending" || res.RequestID == "" {
		t.Fatalf("首次申请应 pending 并广播弹窗，得到 %+v", res)
	}
	// B 小姐「拒绝且不再询问」
	if err := mgr.DenyAndBlock(res.RequestID); err != nil {
		t.Fatalf("拒绝并拉黑失败: %v", err)
	}
	// A 君重试：静默 denied，不新建请求
	res2 := mgr.Handshake("conn-a2", "dev-rude", "A 君的电脑", "macos", "", "")
	if res2.Status != "denied" || res2.Message == "" || res2.RequestID != "" {
		t.Fatalf("拉黑后重试应静默 denied，得到 %+v", res2)
	}
	// 状态摘要含拉黑名单
	st := mgr.Status()
	blocked, ok := st["blocked"].([]map[string]any)
	if !ok || len(blocked) != 1 || blocked[0]["device_id"] != "dev-rude" {
		t.Fatalf("Status 应含拉黑条目，得到 %v", st["blocked"])
	}
	// 重启（新管理器同存储）：拉黑仍生效
	mgr2, err := NewManager(Config{
		Store:      store,
		LocalToken: func() (string, error) { return "local-secret", nil },
		Logger:     goharnesslogging.NewNopLogger(),
	})
	if err != nil {
		t.Fatalf("重建管理器失败: %v", err)
	}
	if res3 := mgr2.Handshake("conn-a3", "dev-rude", "A 君的电脑", "macos", "", ""); res3.Status != "denied" {
		t.Fatalf("重启后拉黑应仍生效，得到 %+v", res3)
	}
	// B 小姐解除 → 恢复正常申请流程
	if err := mgr2.Unblock("dev-rude"); err != nil {
		t.Fatalf("解除拉黑失败: %v", err)
	}
	if res4 := mgr2.Handshake("conn-a4", "dev-rude", "A 君的电脑", "macos", "", ""); res4.Status != "pending" {
		t.Fatalf("解除后应恢复 pending 申请，得到 %+v", res4)
	}
}

// pairDevice 让设备走完整配对流程并完成握手（登记请求 → 批准 → 领 token），
// 返回签发的设备令牌。
func pairDevice(t *testing.T, mgr *Manager, clientID, deviceID string) string {
	t.Helper()
	if res := mgr.Handshake(clientID, deviceID, "测试设备", "macos", "", ""); res.Status != "pending" {
		t.Fatalf("首次握手应 pending，得到 %+v", res)
	}
	// 从登记请求里取 requestID（单活跃请求）
	mgr.mu.Lock()
	var requestID string
	for id, req := range mgr.requests {
		if req.DeviceID == deviceID && req.status == reqPending {
			requestID = id
		}
	}
	mgr.mu.Unlock()
	if requestID == "" {
		t.Fatal("未找到待审批请求")
	}
	if err := mgr.Approve(requestID); err != nil {
		t.Fatalf("批准失败: %v", err)
	}
	// 批准后设备在同一条连接上轮询领 token（真实链路如此），随后
	// 凭 token 重连完成正式登记
	res := mgr.Handshake(clientID, deviceID, "测试设备", "macos", "", "")
	if res.Status != "paired" || res.Token == "" {
		t.Fatalf("批准后应 paired 并下发 token，得到 %+v", res)
	}
	if res := mgr.Handshake(clientID, deviceID, "测试设备", "macos", res.Token, ""); res.Status != "paired" {
		t.Fatalf("凭 token 重连应 paired，得到 %+v", res)
	}
	return res.Token
}

// TestScopeOfLegacyDeviceDefaultsToAsk 旧持久化设备（无作用域字段）加载后
// 归一为远程设备默认级 ask；本机直通为 local；未验证连接为空。
func TestScopeOfLegacyDeviceDefaultsToAsk(t *testing.T) {
	mgr := newTestManager(t)
	token := pairDevice(t, mgr, "conn-ext", "dev-legacy")
	if got := mgr.ScopeOf("conn-ext"); got != ScopeAsk {
		t.Fatalf("新配对设备作用域应为 ask，得到 %q", got)
	}
	if res := mgr.Handshake("conn-local", "cli", "本机命令行", "cli", "local-secret", ""); res.Status != "paired" {
		t.Fatalf("本机令牌应直通，得到 %+v", res)
	}
	if got := mgr.ScopeOf("conn-local"); got != ScopeLocal {
		t.Fatalf("本机直通作用域应为 local，得到 %q", got)
	}
	if got := mgr.ScopeOf("conn-ghost"); got != "" {
		t.Fatalf("未验证连接作用域应为空，得到 %q", got)
	}

	// 模拟旧版本持久化数据（无 scope 字段）重启加载
	mgr.mu.Lock()
	legacy := map[string]Device{"dev-legacy": {
		ID: "dev-legacy", Name: "旧设备", Platform: "macos", Token: token, ApprovedAt: time.Now(),
	}}
	data, _ := json.Marshal(legacy)
	if err := mgr.cfg.Store.Set(keyDevices, string(data)); err != nil {
		t.Fatalf("写入旧版持久化数据失败：%v", err)
	}
	mgr.mu.Unlock()
	mgr2, err := NewManager(Config{
		Store:      mgr.cfg.Store,
		LocalToken: func() (string, error) { return "local-secret", nil },
		Logger:     goharnesslogging.NewNopLogger(),
	})
	if err != nil {
		t.Fatalf("重启加载失败: %v", err)
	}
	if res := mgr2.Handshake("conn-old", "dev-legacy", "旧设备", "macos", token, ""); res.Status != "paired" {
		t.Fatalf("旧设备凭 token 应能重连，得到 %+v", res)
	}
	if got := mgr2.ScopeOf("conn-old"); got != ScopeAsk {
		t.Fatalf("旧记录作用域应归一为 ask，得到 %q", got)
	}
}

// TestRevokeDisconnects 吊销即断连：解除已授权设备时，其存活连接被
// 主动断开（Disconnect 回调收到对应 clientID）。
func TestRevokeDisconnects(t *testing.T) {
	var kicked []string
	mgr, err := NewManager(Config{
		Store:      newMemStore(),
		LocalToken: func() (string, error) { return "local-secret", nil },
		Logger:     goharnesslogging.NewNopLogger(),
		Disconnect: func(clientID string) { kicked = append(kicked, clientID) },
	})
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	pairDevice(t, mgr, "conn-ext", "dev-gone")
	if err := mgr.Revoke("dev-gone"); err != nil {
		t.Fatalf("吊销失败: %v", err)
	}
	if len(kicked) != 1 || kicked[0] != "conn-ext" {
		t.Fatalf("吊销应断开该设备的存活连接，实际 %v", kicked)
	}
	if mgr.Authorized("conn-ext") {
		t.Fatal("吊销后连接验证登记应已清除")
	}
}

// TestSetAllowedFalseDisconnects 关闭总闸即断开全部非本机存活连接，
// 本机直通连接不受影响。
func TestSetAllowedFalseDisconnects(t *testing.T) {
	var kicked []string
	mgr, err := NewManager(Config{
		Store:      newMemStore(),
		LocalToken: func() (string, error) { return "local-secret", nil },
		Logger:     goharnesslogging.NewNopLogger(),
		Disconnect: func(clientID string) { kicked = append(kicked, clientID) },
	})
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	pairDevice(t, mgr, "conn-ext", "dev-x")
	if res := mgr.Handshake("conn-local", "cli", "本机命令行", "cli", "local-secret", ""); res.Status != "paired" {
		t.Fatalf("本机令牌应直通，得到 %+v", res)
	}
	if err := mgr.SetAllowed(false); err != nil {
		t.Fatalf("关闭总闸失败: %v", err)
	}
	if len(kicked) != 1 || kicked[0] != "conn-ext" {
		t.Fatalf("关闸应只断开外接设备连接，实际 %v", kicked)
	}
	if !mgr.LocalVerified("conn-local") {
		t.Fatal("关闸不应影响本机直通连接")
	}
}
