package svc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/devgate"
)

// memCredStore devgate.Store 的内存实现：设备闸测试换装用。
// 真实 CredentialStore 的写路径在测试环境会阻塞在钥匙串交互上，
// 设备闸单测必须用内存桩隔离持久化。
type memCredStore struct{ data map[string]string }

func newMemCredStore() *memCredStore { return &memCredStore{data: map[string]string{}} }

func (s *memCredStore) Get(k string) (string, error) {
	v, ok := s.data[k]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}
func (s *memCredStore) Set(k, v string) error { s.data[k] = v; return nil }

// swapDeviceGateWithMemStore 用内存桩重建设备闸（其余装配照抄 initDeviceGate）。
func swapDeviceGateWithMemStore(t *testing.T, d *Daemon) {
	t.Helper()
	mgr, err := devgate.NewManager(devgate.Config{
		Store:      newMemCredStore(),
		LocalToken: d.loadOrCreateDaemonToken,
		Logger:     d.logger,
		Notify:     func(string, any) {},
		Disconnect: func(string) {},
	})
	if err != nil {
		t.Fatalf("重建设备闸: %v", err)
	}
	d.devGate = mgr
}

// localVerifiedCtx 用本机握手令牌走一次 device.handshake，把指定连接登记为
// 本机直通，返回带该连接身份的 ctx。直投口/互信凭据等「仅本机」面的正向
// 用例都依赖它。
func localVerifiedCtx(t *testing.T, d *Daemon, clientID string) context.Context {
	t.Helper()
	token, err := d.loadOrCreateDaemonToken()
	if err != nil {
		t.Fatalf("loadOrCreateDaemonToken: %v", err)
	}
	res := d.devGate.Handshake(clientID, "test-device", "测试机", "macos", token, "", nil)
	if res.Status != "paired" {
		t.Fatalf("本机令牌握手未直通: %+v", res)
	}
	return gateway.WithClientID(context.Background(), clientID)
}

// TestDeviceGateAuthorized_NilGateFailsClosed 闸被人为置空时必须拒绝
// （fail-closed）：放行等于把工作面交给任意连接。
func TestDeviceGateAuthorized_NilGateFailsClosed(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	d.devGate = nil
	if d.deviceGateAuthorized("any-client") {
		t.Fatal("闸缺失时必须 fail-closed")
	}
}

// TestDeviceGateAuthorized_UnknownClient 未完成握手的连接不通过。
func TestDeviceGateAuthorized_UnknownClient(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if d.deviceGateAuthorized("no-handshake-client") {
		t.Fatal("未握手连接不应通过准入闸")
	}
}

// TestHandleDeviceStatus_GateMissingAndReady 状态查询不能比真实情况更松：
// 闸缺失报「未启用」，闸在位报允许状态。
func TestHandleDeviceStatus_GateMissingAndReady(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	d.devGate = nil
	out, err := d.handleDeviceStatus(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleDeviceStatus: %v", err)
	}
	m := out.(map[string]any)
	if m["available"] != false || m["allowed"] != false {
		t.Errorf("闸缺失应报未启用, got %v", m)
	}

	d2, cleanup2 := newTestDaemon(t)
	defer cleanup2()
	out2, err := d2.handleDeviceStatus(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleDeviceStatus: %v", err)
	}
	m2 := out2.(map[string]any)
	if _, ok := m2["allowed"]; !ok {
		t.Errorf("闸在位应含 allowed 字段, got %v", m2)
	}
}

// TestHandleDeviceAllowSet_MissingParamAndToggle 总开关切换：缺参报错，
// 正常切换后 Allowed 状态跟随。
func TestHandleDeviceAllowSet_MissingParamAndToggle(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	swapDeviceGateWithMemStore(t, d)
	if _, err := d.handleDeviceAllowSet(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("缺少 allowed 参数应报错")
	}

	if _, err := d.handleDeviceAllowSet(context.Background(), []byte(`{"allowed":false}`)); err != nil {
		t.Fatalf("关闭总开关: %v", err)
	}
	if d.devGate.Allowed() {
		t.Error("总开关应为关")
	}
	if _, err := d.handleDeviceAllowSet(context.Background(), []byte(`{"allowed":true}`)); err != nil {
		t.Fatalf("开启总开关: %v", err)
	}
	if !d.devGate.Allowed() {
		t.Error("总开关应为开")
	}
}

// TestHandleDeviceApprovals_MissingParams 批准/拒绝/解除拉黑/吊销缺参报错，
// 不触达闸内部状态。
func TestHandleDeviceApprovals_MissingParams(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleDeviceApprove(context.Background(), []byte(`{}`)); err == nil {
		t.Error("approve 缺 request_id 应报错")
	}
	if _, err := d.handleDeviceDeny(context.Background(), []byte(`{}`)); err == nil {
		t.Error("deny 缺 request_id 应报错")
	}
	if _, err := d.handleDeviceUnblock(context.Background(), []byte(`{}`)); err == nil {
		t.Error("unblock 缺 device_id 应报错")
	}
	if _, err := d.handleDeviceRevoke(context.Background(), []byte(`{}`)); err == nil {
		t.Error("revoke 缺 device_id 应报错")
	}
}

// TestHandleDevicePairCode_Generated 配对码可用且闸侧确认有效。
func TestHandleDevicePairCode_Generated(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	out, err := d.handleDevicePairCode(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleDevicePairCode: %v", err)
	}
	m := out.(map[string]any)
	code, _ := m["code"].(string)
	if code == "" {
		t.Fatalf("配对码不应为空: %v", m)
	}
	if !d.devGate.ValidPairCode(code) {
		t.Error("刚生成的配对码应有效")
	}
}

// TestHandleDeviceHandshake_MissingClientID 无连接身份的握手拒绝
// （身份是登记会话的键，缺失等于无法追踪）。
func TestHandleDeviceHandshake_MissingClientID(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	params, _ := json.Marshal(map[string]string{"device_id": "d1", "token": "whatever"})
	if _, err := d.handleDeviceHandshake(context.Background(), params); err == nil {
		t.Fatal("缺连接身份应报错")
	}
}

// TestHandleDeviceHandshake_LocalTokenDirectPass 本机令牌直通全链：
// 握手 paired → 准入闸通过 → LocalVerified 成立。
func TestHandleDeviceHandshake_LocalTokenDirectPass(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	token, err := d.loadOrCreateDaemonToken()
	if err != nil {
		t.Fatalf("loadOrCreateDaemonToken: %v", err)
	}
	ctx := gateway.WithClientID(context.Background(), "local-cli-1")
	params, _ := json.Marshal(map[string]string{"device_id": "local", "token": token})
	out, err := d.handleDeviceHandshake(ctx, params)
	if err != nil {
		t.Fatalf("handleDeviceHandshake: %v", err)
	}
	if out.(devgate.HandshakeResult).Status != "paired" {
		t.Errorf("本机令牌应直通, got %v", out)
	}
	if !d.deviceGateAuthorized("local-cli-1") {
		t.Error("直通后准入闸应通过")
	}
	if !d.devGate.LocalVerified("local-cli-1") {
		t.Error("本机令牌握手后应 LocalVerified")
	}
}

// TestHandleDeviceTrustTicket_RejectedForNonLocal 互信凭据仅本机可领：
// 外接设备（未握手/外部令牌握手）调 trust.ticket 一律拒绝，防凭连接权代领。
func TestHandleDeviceTrustTicket_RejectedForNonLocal(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := d.handleDeviceTrustTicket(context.Background(), nil); err == nil {
		t.Error("缺连接身份应拒绝")
	}
	external := gateway.WithClientID(context.Background(), "external-1")
	if _, err := d.handleDeviceTrustTicket(external, nil); err == nil {
		t.Error("未握手外接设备应拒绝")
	}

	d.devGate = nil
	if _, err := d.handleDeviceTrustTicket(external, nil); err == nil {
		t.Error("闸缺失应拒绝")
	}
}

// TestHandleDeviceTrustTicket_LocalVerifiedButLoggedOut 本机直通但未登录社区：
// 闸放行后因无社区凭据拒发（闸与登录两层检查各自生效）。
func TestHandleDeviceTrustTicket_LocalVerifiedButLoggedOut(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	ctx := localVerifiedCtx(t, d, "local-1")
	_, err := d.handleDeviceTrustTicket(ctx, nil)
	if err == nil {
		t.Fatal("未登录社区应拒发互信凭据")
	}
}
