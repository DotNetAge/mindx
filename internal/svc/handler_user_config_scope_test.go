package svc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/stretchr/testify/require"
)

// askScopedCtx 构造一条「远程设备」连接上下文（scope=ask）。
//
// 路径与真实手机一致：新设备握手 → 主机弹窗批准 → 设备凭 token 再次握手。
// 不能直接往 sessions 里塞 map：那样测出来的是桩，不是生产准入链。
func askScopedCtx(t *testing.T, d *Daemon, clientID, deviceID string) context.Context {
	t.Helper()
	require.NotNil(t, d.devGate, "测试前提：设备闸应已装配")

	// 首次握手：未知设备 → 登记待审批请求
	res := d.devGate.Handshake(clientID, deviceID, "iPhone", "ios", "", "", nil)
	require.Equal(t, "pending", res.Status, "新设备首次握手应进入待审批")
	require.NotEmpty(t, res.RequestID)

	require.NoError(t, d.devGate.Approve(res.RequestID), "批准接入请求")

	// 二次握手：凭刚签发的 token 领取 → paired，作用域固定 ask
	res = d.devGate.Handshake(clientID, deviceID, "iPhone", "ios", "", "", nil)
	require.Equal(t, "paired", res.Status, "批准后应完成配对")

	require.Equal(t, "ask", d.devGate.ScopeOf(clientID), "远程设备作用域必须是 ask")

	return gateway.WithClientID(context.Background(), clientID)
}

// TestUserConfigReadableByRemoteDevice 远程设备必须能读到全局配置。
//
// 回归点（P0，真实故障）：user.config 曾整体登记在 managerMethods（管理面），
// 而手机经局域网配对进来的连接 scope=ask，每次读都被拒成
// 「这项操作涉及本机管理，请直接在主机上操作」。
//
// 客户端把该错误静默吞掉后，「以服务端为权威恢复当前模型」这条逻辑从未执行，
// 界面只能回落到本地缓存 → 实测表现就是「ChatFlow 输入框的模型与主机上完全
// 不同，像是从缓存读出来的」。同源逻辑必须同权限：CLI / mindx-work 跑在本机
// （scope=local）读得到，iOS 读不到，差的不是逻辑而是这条边界。
func TestUserConfigReadableByRemoteDevice(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	swapDeviceGateWithMemStore(t, d)

	// 预置一个可识别的当前模型，读回来必须是它（而不是空/默认）
	d.app.Config().LastModel = "dashscope/deepseek-v4.1-flash-1017"
	d.app.Config().DefaultModel = "dashscope/deepseek-v4.1-flash-1017"

	ctx := askScopedCtx(t, d, "conn-iphone", "dev-iphone")

	// 手机端的真实调用形态：空体读
	got, err := d.handleUserConfig(ctx, nil)
	require.NoError(t, err, "远程设备读全局配置必须放行")

	cfg, ok := got.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "dashscope/deepseek-v4.1-flash-1017", cfg["last_model"],
		"必须返回主机上真正生效的模型，客户端据此校正输入条")
}

// TestUserConfigEmptyObjectIsRead 空对象 `{}` 等同纯读，不得被拒。
//
// 桌面端 mindx-work 的调用形态是 `callAfterGate('user.config', {})`——带一个
// 空对象而不是无参数。若按「有 params 就当写」判定，会把桌面端的读也误杀。
func TestUserConfigEmptyObjectIsRead(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	swapDeviceGateWithMemStore(t, d)

	ctx := askScopedCtx(t, d, "conn-desktop", "dev-desktop")

	_, err := d.handleUserConfig(ctx, json.RawMessage(`{}`))
	require.NoError(t, err, "空对象是纯读，远程设备也必须放行")
}

// TestUserConfigWriteRejectedForRemoteDevice 远程设备不得改写全局配置。
//
// 写的是机器级安全态（中继地址、市场源 ID、邀请码、模型引用等），
// 只有本机主人能改。放开读不能顺带放开写。
func TestUserConfigWriteRejectedForRemoteDevice(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	swapDeviceGateWithMemStore(t, d)

	before := d.app.Config().KBAddr
	ctx := askScopedCtx(t, d, "conn-iphone", "dev-iphone")

	_, err := d.handleUserConfig(ctx, json.RawMessage(`{"kb_addr":"http://evil.invalid:9/kb"}`))
	require.Error(t, err, "远程设备写配置必须被拒")
	require.Contains(t, err.Error(), "请直接在主机上操作")
	require.Equal(t, before, d.app.Config().KBAddr, "被拒的写不得落盘")
}

// TestUserConfigWritableByLocal 本机直通连接读写皆可（CLI / mindx-work 场景）。
func TestUserConfigWritableByLocal(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	swapDeviceGateWithMemStore(t, d)

	ctx := localVerifiedCtx(t, d, "conn-local")

	// 读
	got, err := d.handleUserConfig(ctx, nil)
	require.NoError(t, err, "本机读配置必须放行")
	_, ok := got.(map[string]any)
	require.True(t, ok)

	// 写
	_, err = d.handleUserConfig(ctx, json.RawMessage(`{"kb_addr":"http://127.0.0.1:8899/kb"}`))
	require.NoError(t, err, "本机写配置必须放行")
	require.Equal(t, "http://127.0.0.1:8899/kb", d.app.Config().KBAddr)
}
