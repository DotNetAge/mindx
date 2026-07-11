---
name: system-diag
description: >
  通过分析运行时日志、资源指标和守护进程状态诊断 MindX 系统健康，
  输出包含根因识别和修复建议的结构化诊断报告。当用户报告系统问题、
  守护进程崩溃、性能下降或请求主动健康检查时使用。
  作为 `mindx doctor`（静态检查）的补充，提供 AI 驱动的日志分析和
  跨组件关联分析。
allowed-tools: bash read sub-agent collect-results
metadata:
  name_zh: 系统诊断
  name_zh-tw: 系統診斷
  description_zh: 通过分析运行时日志、资源指标和守护进程状态诊断 MindX 系统健康，输出结构化诊断报告与修复建议
  description_zh-tw: 通過分析運行時日誌、資源指標和守護進程狀態診斷 MindX 系統健康，輸出結構化診斷報告與修復建議
---

## 触发判断

遇到以下情况时使用此技能：

- 用户要求检查系统健康、诊断问题或分析日志
- 用户报告守护进程崩溃、重启或无响应
- 用户遇到性能下降（响应缓慢、卡顿）
- 运维智能体执行例行健康检查

以下情况**不要使用**：
- 静态配置验证——改用 `mindx doctor`
- 单个 Bug 修复或代码问题——直接修复
- 功能开发——改用 `software-dev` 技能

**与 `mindx doctor` 的关系：**
| `mindx doctor`（CLI） | 本技能（AI） |
|------------------------|-------------|
| 静态规则检查（文件存在？进程在运行？） | 日志内容分析 + 模式识别 + 关联分析 |
| 表层：**什么**出了问题 | 深层：**为什么**发生 + **如何**修复 |

## 工作流程

### 阶段 1：收集证据

从 5 个维度收集数据。某个维度失败不会阻塞其他维度。

```
A: 运行时快照
   - mindx status                          → 安装/配置/守护进程状态
   - curl http://localhost:{port}/api/health → 服务组件健康状态
   - ps aux | grep "mindx daemon"          → 进程资源使用情况

B: 日志
   - {workspace}/logs/mindx.log            → 主日志（所有级别，JSON 格式）
   - {workspace}/logs/error.log            → 错误日志（ERROR 及以上）
   - {workspace}/logs/daemon.log           → 守护进程标准输出
   - {workspace}/logs/daemon.err.log       → 守护进程标准错误
   - *.log.gz                              → 轮转历史日志（用于回溯）

C: 存储状态
   - du -sh {workspace}/data/              → 数据目录大小
   - du -sh {workspace}/data/models/       → 模型文件大小
   - ls {workspace}/*.db                   → bbolt 数据库文件
   - df -h {workspace}                     → 磁盘空间

D: 系统资源
   - ulimit -n                            → 文件描述符限制
   - vm_stat (macOS) / free -h (Linux)    → 内存压力
   - launchctl list | grep mindx          → launchd 注册状态

E: 网络（可选）
   - lsof -i :{port} -P                  → 端口监听状态
```

> **注意：** `{workspace}` 在 macOS/Linux 上解析为 `~/.mindx`，在 Windows 上为 `%APPDATA%\mindx`。端口来自用户配置或默认值。

### 阶段 2：分析日志

MindX 日志使用 **uber-go/zap JSON 格式**。要阅读并关联分析，不要只是逐行扫描。

#### 2.1 重建时间线

按 `ts` 排序所有事件，映射守护进程生命周期的关键节点：

```
[启动] → [调度器?] → [网关] → [Web 服务器?] → [运行中...] → [异常?]
```

#### 2.2 错误频率与模式聚合

统计每个 ERROR/WARN 的出现次数和时间分布：

```
gateway start failed        ×1   (23:01:02)
Scheduler failed to start   ×3   (~每 2 小时)
knowledge-graph unavailable ×0
```

#### 2.3 跨组件关联

把看似独立的错误关联到单一根因：

```
症状 A: "gateway start failed: bind: address already in use"
症状 B: 进程列表中发现过期 PID
推断: 之前的守护进程未正常退出，端口未释放
根因: launchd KeepAlive 未正确处理守护进程崩溃
```

#### 2.4 趋势分析

对比轮转日志中的退化信号：

```
今天：    12 个 WARN 事件
昨天：     3 个 WARN 事件
前天：     0 个 WARN 事件
→ 问题在恶化，需要关注
```

#### 2.5 已知错误模式

加载 `references/error-patterns.md` 获取完整的 错误→根因→修复 映射。

高频模式快速索引：

| 错误关键词 | 可能的根因 | 参考 |
|-----------|-----------|------|
| `address already in use` | 端口冲突 / 过期进程 | 参见参考文档 |
| `too many open files` | 文件描述符耗尽 | `references/resource-exhaustion.md` |
| `context deadline exceeded` | API 超时 / 网络问题 | 参见参考文档 |
| `knowledge-graph database unavailable` | 图数据库损坏 / 权限问题 | `references/storage.md` |
| `failed to initialize kvstore` | KVStore 损坏 / 磁盘已满 | `references/storage.md` |
| `Scheduler failed to start` | 调度器存储损坏 | 参见参考文档 |

### 阶段 3：生成诊断报告

用以下模板输出结构化报告：

```markdown
# MindX 系统诊断报告
**时间**：{timestamp}
**守护进程运行时长**：{uptime}
**日志覆盖范围**：{log_time_range}

---

## 严重（需立即处理）

### {N}. {问题标题}
- **症状**：{可观测行为}
- **证据**：`{logfile}:{line}` — `{原始日志摘录}`
- **根因**：{从证据推导出的逻辑结论}
- **影响**：{受影响的功能}
- **修复**：{具体步骤}
- **预防**：{避免再次发生的建议}

## 警告（尽快处理）

{与上述格式相同}

## 信息（优化建议）

{非问题，改进机会}

## 系统资源概览

| 指标 | 值 | 状态 |
|------|-----|------|
| 守护进程内存 | {RSS} MB | 🟢/🟡/🔴 |
| 守护进程 CPU | {cpu%} | 🟢/🟡/🔴 |
| 磁盘使用 | {used}/{total} ({pct}%) | 🟢/🟡/🔴 |
| 日志大小 | {size} | 🟢/🟡/🔴 |
| 文件描述符 | {open}/{limit} | 🟢/🟡/🔴 |
| 重启次数（24h） | {count} | 🟢/🟡/🔴 |

## 总结

{一段话的整体健康评估 + 最优先的 1-2 项}
```

### 阶段 4：执行修复（需授权）

仅在用户明确要求时（`--fix` 或"修复它"）才执行：

1. **安全修复** — 直接执行：终止过期进程、释放端口、裁剪过大的日志
2. **配置变更** — 应用前展示差异
3. **存储操作** — 操作数据库/数据文件前先备份
4. **不确定的情况** — 标记为"需人工介入"，不要猜测

## 原则

1. **基于证据** — 每个结论必须引用具体的日志行；没有证据不要推测
2. **优先单一根因** — 先假设一个根因能解释所有症状，直到出现矛盾再考虑多因分析
3. **区分症状与原因** — "网关失败"是症状，"端口被占用"是原因，"崩溃遗留的过期进程"是根因
4. **时间敏感推理** — 短时间内密集出现的错误 > 零散的单个事件
5. **不要危言耸听** — 单个 WARN 不是系统故障；用频率 + 趋势来衡量严重程度
6. **可操作的输出** — 每个诊断必须包含具体的修复步骤；永远不要说"可能是个问题"

## 反模式

- 不要跳过证据收集仅凭假设诊断
- 不要将每个 WARN 都视为严重问题——结合频率和趋势上下文判断
- 不要给出模糊的建议如"检查一下配置"——具体说明检查什么以及为什么
- 未经用户明确授权不要修改数据文件或数据库
- 不要将相关性误认为因果——两个错误同时出现不一定有关联
- 不要省略总结部分——用户需要清晰的优先行动列表