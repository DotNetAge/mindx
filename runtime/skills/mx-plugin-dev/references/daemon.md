# daemon RPC 数据通道（插件取数唯一通路）

界面席位契约（contract.md）之外，插件的真实数据都来自 daemon RPC 与宿主桥。本文全部实证自插件真码（chatflow / video-editor / gitgraph / explorer / markdown），形状坑每条都是实踩教训。

## 1. 服务获取与调用

连接服务注册名 `daemon.connection`，类型**消费侧本地声明形状**（插件间禁止 import，TS 结构化类型各声明所需字段）：

```ts
// 服务结构契约（只声明自己用到的形状）
interface DaemonConnection {
  readonly state: string                                  // 连接状态（响应式值）
  call<T>(method: string, params?: unknown, timeoutMs?: number): Promise<T>
  notify(method: string, params?: unknown): boolean       // 通知无响应；未连接返回 false
  onNotification(method: string, cb: (params: unknown) => void): () => void
}
```

获取时机（对齐 contract.md §10 的 inject 硬边界）：

- **组件 setup / store setup 函数体**：`useService<DaemonConnection>('daemon.connection')`。
- **非组件上下文**（事件回调深处、异步续体）：用捕获的引用，或 `theShell().services.use<DaemonConnection>('daemon.connection')`（内核方法无 inject 依赖）。

调用：`const list = await daemon.call<FsEntry[]>('fs.list', { path: target })`。

## 2. fs.* 方法与形状坑（军规）

| 方法 | 入参 | 返回 | 备注 |
| --- | --- | --- | --- |
| `fs.home` | — | `{ path: string }` | **返回对象不是字符串**（实证 handler_fs.go）：当字符串拼进 fs.list 报 `cannot unmarshal object`，正确写法 `homeDir.value = home?.path ?? '/'` |
| `fs.list` | `{ path }` | `FSEntry[]`（name/path/size/is_dir/mod_time） | 目录不存在等错误直接 reject，调用方处理 |
| `fs.stat` | `{ path }` | 条目形状 | 探测存在性 |
| `fs.read_base64` / `fs.write_base64` | `{ path }` / `{ path, content }` | base64 串 | 二进制与大文本通道 |
| `fs.write` | `{ path, content }` | — | 纯文本小写入 |
| `fs.rm` | `{ path, recurse? }` | — | recurse 删除目录 |

- **bool 参数必须显式传值**：daemon 侧 Go 结构体 bool 非 `*bool`，JSON 缺字段反序列化为零值 false 落盘——「停用」事故的根因。调用带 bool 字段的 RPC 逐一核对 Go 结构体显式传。
- **RPC 返回字段名以实测响应为准**，禁止凭移植来源的类型声明推断（实证：usage 条目字段是 `input_tokens`/`output_tokens`，按 desktop 类型写 `prompt_tokens` 渲染即崩）。新对接的 RPC 先发真实探针核对响应 JSON，再写 interface。

## 3. base64 文本解码禁用 atob（军规）

`atob` 按 Latin-1 解码，UTF-8 中文每个字变 3 个乱码字符且不报错——打开即乱码，再保存即永久损坏。凡承载文本内容的 base64 通道一律：

```ts
const text = new TextDecoder().decode(Uint8Array.from(b64, (c) => c.charCodeAt(0)))
```

## 4. dialog 与宿主桥（window.mxDesktop）

Electron 宿主注入 `window.mxDesktop`；**纯 Web 环境不存在**——宿主能力必须可选链 + 显式降级判断，禁止静默假装成功：

```ts
const picked = await window.mxDesktop?.dialog.saveFile(`${name}.webm`)
if (!picked) return // 用户取消或非宿主环境：显式返回，不写盘
```

## 5. terminal pty 桥（一次性命令的唯一定义式通道）

daemon RPC **没有通用 exec**；`window.mxDesktop.terminal` 是插件跑命令的唯一通道，但它是**交互 shell**（回显 + prompt + ANSI 全混在流里）。一次性命令必须用哨兵协议：

```bash
printf '__MXG''_BEGIN__\n'; <命令>; printf '\n__MXG''_END_%s__' "$?"
```

铁律（每条都是 gitgraph 实测两连坑）：

1. **先剥 ANSI 再匹配再切片**：哨兵 regex 的 exec index 是含 ANSI 的原始流偏移，与剥离后文本错位可达数千字节——不剥就切出垃圾。
2. **哨兵字面量拆段拼接**（`'__MXG''_BEGIN__'`）：命令串里的哨兵会被 zsh 原样回显，indexOf 命中回显；切片用 `lastIndexOf` 取真执行输出双保险。
3. git 在 tty 下自动上色，命令一律带 `--no-color`。
4. 桥形状（消费侧声明）：`create()` 建会话 + `write(id, data)` 写命令 + `onData(({id, data}) => …)` 收流返回退订函数。

## 6. mx-file:// 媒体流

本地文件经自定义协议流入渲染进程，URL 必须**逐段编码**：

```ts
const url = `mx-file://local${path.split('/').map(encodeURIComponent).join('/')}`
```

- `encodeURI` 不编码 `#`/`?`/`%`（视作 URL 结构字符），含 `#` 文件名会被截成 fragment——只有逐段 `encodeURIComponent` 完整还原。
- 渲染进程用 `<video>` / canvas `drawImage` 采样媒体帧，依赖 file-stream Response 的 `Access-Control-Allow-Origin: *` 头（已内建）；缺该头即跨源污染：缩略图抓帧全黑、导出抛错，报错晚至调用点难定位。

## 7. 验证与边界

- daemon 无连接时（纯 Web）插件取数全部 reject：功能入口须可读空态或显式提示，禁止裸崩。
- 验证 RPC 形状：起 daemon 后对 `ws://localhost:1314/ws` 直连发 JSON-RPC 探针，先看响应再写类型。
- 命令执行类需求先想 pty 哨兵协议（§5），不要在 daemon RPC 里找 exec（没有）。
