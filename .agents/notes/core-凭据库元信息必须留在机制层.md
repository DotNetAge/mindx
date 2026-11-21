# 凭据库元信息必须留在机制层

## 现象
连接器密钥面板需要展示「阿里云访问密钥」这类人读名字，第一版把展示名与字段中文名写进
`~/.mindx/settings/credentials.json`，与 Keychain 里的值分开存。

## 问题
1. `credentials.json` 把「钥匙串 / 加密文件」这一层实现细节泄露到了应用层：换平台、
   换后端、或迁移目录时，文件与值不同步，留下孤儿数据。
2. 元信息本身不是秘密，但它属于「凭据这一条记录」，凭据在哪它就该在哪；拆开存等于
   同一个概念有两份真源。
3. 枚举能力也缺：Keychain 无法按键前缀列举，上层只能靠旁挂文件猜。

## 现在的做法
`CredentialStore` 扩到「取值 / 写值 / 删除 / 写元信息 / 按键前缀枚举」：
- 元信息与该条凭据同库同条，语义（display、label）由应用层约定，机制层只搬运。
- 枚举靠库内索引项 `__mindx_credentials_index_v1`（同一条 Get/Set 通道），
  两个后端天然一致，不需要按平台各造一套。
- 删除变成接口能力（`security delete-generic-password`），旧键可以随轮换清理。

## 踩到的平台坑
macOS `security add-generic-password -w` 写入含非 ASCII 字符的值时，`find ... -w`
读回来是十六进制串。索引 JSON 里带中文展示名会立刻触发：写进去是 JSON，读回来是 hex。
处理：索引写盘前把所有非 ASCII 转成 `\uXXXX`（语义不变，纯 ASCII 传输），读取时
额外兼容一次 hex 解码，历史坏索引能自愈。

## Linux / Docker
Linux（含 Docker、WSL、Windows 走 Docker）没有钥匙串，用 `encryptedFileStore`：
AES-256-GCM 密文 `settings/.credentials` + 同目录随机密钥 `.credentials.key`。
- 写盘改成临时文件 + rename：容器被 kill 或磁盘满时旧密文仍完整，不会把凭据变成残片。
- `MINDX_CREDENTIALS_DIR` 可把这两个文件放到挂载卷（compose 的 `mindx-credentials` 卷
  就是干这个的），否则容器重建后密钥随镜像清空。
- 索引写入失败不得连坐主写入：老代码（模型 API Key、市场令牌、技能凭据）把 Set 失败
  当成「没存上」，索引只是附加能力。

## 判据
新增任何「与凭据有关的附加信息」时，先问：这是机制层能力还是应用约定？能力进
`CredentialStore`，约定用 Meta 字段表达；不得为某一平台单独落文件。
