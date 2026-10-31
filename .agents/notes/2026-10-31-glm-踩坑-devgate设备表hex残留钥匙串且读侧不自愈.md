# 踩坑：devgate 设备表以 hex 残留钥匙串，读侧只告警不自愈

- 记录时间: 2026-10-31 17:50
- agent: glm
- 分类: 踩坑
- 仓库: mindx（修复 75a83b2，本地未推）

## 现象

daemon 每次启动 warn「已授权设备表损坏，忽略 invalid character 'b' after
top-level value」，持续一周无人处理。用户明确要求：出现此类损坏必须马上
自愈，不能只写日志。

## 根因

devgate 持久化用 `oauthCredStore`（darwin = macOS 钥匙串
service=mindx, account=key），不是 kv.db——排查时 mindx kv get 读
not found 造成误导（CLI kv 走 bbolt，两套存储）。损坏值实锤：
2026-10-17 连接冒烟残留，把设备表 JSON 以 hex 编码写入钥匙串
（`7b22736d6f6b...` = {"smoke-dev":...}）。读侧 json.Unmarshal 对 hex
串解析：`7` 是合法数字顶层值，后遇 `b` 炸——错误信息逐字吻合。
写入端 persistLocked 是纯 JSON，当前代码不会复发（一次性历史污染）。

## 修复（75a83b2）

NewManager 加载段：Unmarshal 失败 → warn 附 corrupt_head（前 120 字符
摘录）→ devices/blocked 置空表 → 立即 Set 回写 "{}" 覆盖坏值。不搞
hex 解码兼容（迁就坏数据正是数据源混乱之源，2026-10-30 用户定案）。
代价：外接设备需重新配对（本机令牌直通不受影响）。

## 排查路径（复用价值）

1. CLI kv get 读到 not found → 反推 devgate 存的不是 kv.db → 查装配点
   `Store: d.oauthCredStore` → darwin = 钥匙串
2. `security find-generic-password -s mindx -a devgate_devices -w` 读出
   hex 值 → 手工解码确认内容与来源时间（cdat 2026-10-17）
3. 修复验证：make restart 后日志出现「已重置为空表」+ 钥匙串值变 "{}"

## 教训

- 存储层报警只 warn 不处置 = 让每次启动重复踩同一个坑；持久化数据解析
  失败的自愈动作（重置+回写+降级代价说明）应与报警同时落地
- 冒烟测试写真实用户存储（本机钥匙串）必须用隔离 MINDX_HOME 或测试
  service 名，残留会变成生产环境的慢性病
