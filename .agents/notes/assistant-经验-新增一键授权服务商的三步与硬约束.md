# 新增一家「一键授权」服务商要改哪里、不能踩什么

## 结论先说

连接器自动授权已经是配置驱动，新增一家**不需要改代码**（`internal/mcp/oauth_device.go` 里没有任何服务商分支）。要动的只有两处配置：

1. `mindx/runtime/settings/client-provider.yml` + 用户机上的 `~/.mindx/settings/client-provider.yml`，两边都要加（前者是安装源，后者是运行读取点）。
2. `mindx-work/plugins/src/library/connector-secrets.ts` 里对应连接器的 `SecretGuide.oauth` 填服务商 `name`。

## yml 条目的形状

```yaml
oauth_providers:
  - name: acme
    title: Acme                     # 界面按钮就是「Acme 授权」
    kind: device_code               # 只有这个 kind 会进自动授权清单
    device_url: https://acme.example/device/code
    token_url: https://acme.example/token
    userinfo_url: https://acme.example/user   # 可选，用来提示「已授权给 @xxx」
    user_name_field: login                    # 可选，默认 login
    client_id: <公开 client_id>
    scopes: [read, write]
    interaction: device_code        # 必须与 kind 派生值一致，否则校验直接报错
```

## 硬约束（破坏其中一条就等于埋雷）

- **client_secret 绝不进 yml**。`LoadClientProviders` 遇到非空 `client_secret` 会直接报错，这是有意的。设备码是公共客户端流程，只认 `client_id`。
- **服务商没有 `device_url` 就不能走设备码**。Gitee 实测 `POST https://gitee.com/oauth/device/code` 返回 404，也没有应用设置开关，所以 Gitee 只能留在手动填令牌那一类，不要给它编 device 条目。
- **`name` 唯一**，且前端 `SecretGuide.oauth` 用的就是这个 `name`，不是 title。
- **`~/.mindx/settings/client-provider.yml` 必须同步**，否则本机运行时读到的还是旧清单，界面上看起来像「代码没生效」。

## 前端只有一处要改

`connector-secrets.ts` 里给对应连接器的 guide 加 `oauth: '<name>'`。按钮文案、说明句、授权页标题都由 daemon 的 `mcp.oauth.providers` 返回的 `title` 拼出来，模板里没有服务商字面量。

## 验证顺序（照这个顺序跑，出错能立刻定位到哪一层）

```bash
# 1. 配置能被解析（条目字段错、interaction 不匹配都会在这里暴露）
go test ./internal/core -run ClientProviders

# 2. 清单能到界面 + 流程本身可用
go test ./internal/mcp -run "OAuthProviders|DeviceFlow"

# 3. 换二进制：不要用 mindx restart
cp <新构建的 mindx> ~/.mindx/bin/mindx
launchctl kickstart -k gui/$(id -u)/com.mindx.daemon
```

`mindx restart` 会用 `/Applications/MindX.app/Contents/Resources/mindx` 覆盖 `~/.mindx/bin/mindx`，把刚换上的开发版打回旧版（现象是新增的 CLI 标志突然「不存在」），排查时先 `md5 -q` 对一下二进制。

## 老路径不能碰

`CredentialStore` 同时承载各供应商的模型 API Key。这次扩接口只加不改，改完必须回归一次：

```bash
mindx secret get deepseek     # 老供应商 Key 仍能取回
mindx mcp tools github        # 组凭据解析、连接器仍能用
```

索引维护全部 best-effort，索引写失败不得让主写入报错。
