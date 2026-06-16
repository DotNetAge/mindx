# MindX 错误模式速查表

> 本文件是 Phase 2 日志分析的核心参考。每个条目包含：错误特征 → 根因推断 → 修复方案 → 预防措施。

## 格式说明

每条模式包含：
- **触发日志**：实际出现在 mindx.log / error.log 中的典型消息
- **caller 位置**：源码位置（用于定位代码上下文）
- **严重度**：🔴 Critical / 🟡 Warning / ℹ️ Info
- **根因链**：从现象到根因的推导路径
- **修复操作**：具体可执行的命令/步骤
- **预防措施**：避免再次发生的建议

---

## 一、启动与生命周期

### 1.1 端口冲突

```
触发日志: "gateway start failed" + error 包含 "address already in use"
caller:   svc/daemon.go:618
严重度:    🔴 Critical（daemon 无法启动）
```

**根因链：**
```
gateway bind 失败
  → 端口已被占用
    → 上次 daemon 未正常退出（crash/kill -9）
      → launchd KeepAlive 未生效（或 fallback 模式无 KeepAlive）
        → 残留进程仍持有端口
          或: 其他程序占用了同一端口
```

**诊断步骤：**
```bash
# 1. 确认端口被谁占用
lsof -i :{port} -P -n | grep LISTEN

# 2. 检查残留的 mindx 进程
ps aux | grep "mindx daemon" | grep -v grep

# 3. 检查 launchd 状态
launchctl list | grep mindx
```

**修复操作：**
```bash
# 方案 A：杀掉残留进程后重启
pkill -9 -f "mindx daemon"; sleep 2; mindx start

# 方案 B：如果确认端口被其他程序占用，修改 MindX 端口
# 编辑 ~/.mindx/mindx.json 中的 port 配置
```

**预防措施：**
- 优先使用 launchd 管理模式（而非 fallback），确保 KeepAlive 自动重启
- 避免 `kill -9`，优先 `mindx stop` 正常关闭

---

### 1.2 Scheduler 启动失败

```
触发日志: "Scheduler failed to start" + error 详情
caller:   svc/daemon.go:585
严重度:    🟡 Warning（定时任务不可用，但 daemon 其余功能正常）
```

**根因链：**
```
scheduler.Start() 返回 error
  → 调度器存储初始化失败（最常见）
    → bbolt 数据库文件损坏或锁定
      或: 存储目录权限不足
```

**诊断步骤：**
```bash
# 1. 检查 scheduler 相关存储
ls -la ~/.mindx/data/
file ~/.mindx/data/*.db 2>/dev/null

# 2. 查看完整的 scheduler 错误上下文
grep -A2 "Scheduler failed to start" ~/.mindx/logs/error.log
```

**修复操作：**
```bash
# 如果数据库损坏：备份后删除让 daemon 重建
cp ~/.mindx/data/scheduler.db ~/.mindx/data/scheduler.db.bak
rm ~/.mindx/data/scheduler.db
mindx restart
```

---

### 1.3 Knowledge Graph DB 不可用

```
触发日志: "knowledge-graph database unavailable, graph RPC disabled"
caller:   svc/daemon.go:444
严重度:    🟡 Warning（GraphRAG 功能不可用）
```

**根因链：**
```
graph DB 初始化失败
  → bbolt 文件不存在/损坏/权限不足
    → data/graph/ 目录异常
      或: 首次运行未完成初始化
```

**诊断步骤：**
```bash
ls -la ~/.mindx/data/graph/ 2>/dev/null || echo "graph dir missing"
grep "initialize knowledge-graph" ~/.mindx/logs/error.log | tail -5
```

**修复操作：**
```bash
# 重建 graph 目录结构
mkdir -p ~/.mindx/data/graph
chmod 755 ~/.mindx/data/graph
mindx restart
```

---

### 1.4 KVStore 初始化失败

```
触发日志: "failed to initialize kvstore" + error 详情
caller:   svc/daemon.go:464
严重度:    🟡 Warning（KV 功能不可用：agent score、token usage 等依赖 KV）
```

**根因链：**
```
kvstore.Open() 失败
  → bbolt 文件损坏
    或: 磁盘空间不足（bbolt 需要 mmap）
      或: 文件锁冲突（两个 daemon 实例同时运行）
```

**诊断步骤：**
```bash
# 1. 检查磁盘空间
df -h ~/.mindx/

# 2. 检查是否有多个 daemon 进程
ps aux | grep "mindx daemon" | grep -v grep | wc -l

# 3. 查看 KV 文件状态
ls -la ~/.mindx/*.db 2>/dev/null
```

**修复操作：**
```bash
# 磁盘满：清理旧日志/会话数据
# 多进程：杀掉多余实例
# 文件损坏：备份后重建
cp ~/.mindx/mindx.kv.db ~/.mindx/mindx.kv.db.bak 2>/dev/null
rm ~/.mindx/mindx.kv.db 2>/dev/null
mindx restart
```

---

## 二、运行时错误

### 2.1 LLM 请求异常

```
触发日志: "defaultHandler: AskBuilder panic"
caller:   svc/daemon.go:932
严重度:    🟡 Warning（单次请求失败，不影响整体服务）
```

**根因链：**
```
AskBuilder 执行时 panic
  → LLM API 返回非预期格式
    → provider 配置错误（API key 过期、endpoint 变更）
      或: 请求参数构造异常（超长 prompt、非法 token）
        或: 网络中断导致响应截断
```

**诊断步骤：**
```bash
# 1. 查看该 panic 的完整堆栈和上下文
grep -B5 -A10 "AskBuilder panic" ~/.mindx/logs/error.log | tail -20

# 2. 检查是否集中在某个 provider
grep "AskBuilder panic" ~/.mindx/logs/error.log | \
  python3 -c "import sys,json; [print(json.loads(l).get('msg','')) for l in sys.stdin]" | \
  sort | uniq -c | sort -rn
```

**修复操作：**
- 单次偶发 → 无需处理（网络抖动）
- 频繁出现 → 检查对应 provider 的 API key 和 endpoint 配置
- 总是同一个 agent 出问题 → 检查该 agent 的 system prompt 是否过长

---

### 2.2 通用请求失败

```
触发日志: "request failed" + error 详情
caller:   svc/daemon.go:1094
严重度:    取决于频率（单次=🟡，频繁=🔴）
```

**常见 error 内容及含义：

| error 关键词 | 含义 | 建议 |
|-------------|------|------|
| `context deadline exceeded` | LLM API 超时 | 检查网络/模型响应速度 |
| `429 Too Many Requests` | Provider 限流 | 降低并发或换模型 |
| `401 Unauthorized` | API key 无效 | 更新配置 |
| `connection refused` | 网络不通 | 检查代理/防火墙 |
| `EOF` | 连接意外断开 | 网络不稳定 |

**频率判断标准：**
- < 3次/小时 → 正常波动，无需关注
- 3-10次/小时 → 🟡 注意观察趋势
- > 10次/小时 → 🔴 需要排查

---

### 2.3 终端读取错误

```
触发日志: "terminal read error" + error
caller:   svc/daemon_rpc_terminal.go:118
严重度:    ℹ️ Info（终端会话正常断开）
```

**根因：** 用户关闭了终端窗口或 session 断开。这是正常行为，不是系统故障。

**判断标准：** 仅当大量集中出现（>20次/分钟）时才需要关注——可能意味着终端管理有 bug。

---

### 2.4 Token Usage 记录失败

```
触发日志: "failed to record token usage for translate/..."
caller:   svc/daemon_rpc_translate.go:94
严重度:    ℹ️ Info（统计功能受影响，核心功能正常）
```

**根因：** KVStore 写入失败（通常伴随 KVStore 初始化警告）。

**修复：** 先解决 KVStore 问题（见 1.4），token usage 会自动恢复。

---

## 三、资源类

### 3.1 WebUI 目录缺失

```
触发日志: "web directory does not exist, skipping WebUI server"
caller:   svc/web_server.go:70
严重度:    ℹ️ Info（WebUI 不可用，但 CLI/gRPC 不受影响）
```

**修复：**
```bash
mindx install --no-daemon  # 重新安装前端资源
```

---

### 3.2 FileWatch 服务异常

```
触发日志: "filewatch.start: service exited with error"
caller:   svc/daemon_rpc_memory.go:466
严重度:    🟡 Warning（文件监控不可用，手动索引仍可用）
```

**根因链：**
```
FileWatch 服务退出
  → fsnotify 监控目录被删除/移动
    或: 监控目录数量超过 OS 限制（kqueue/inotify 上限）
      或: 权限变更导致无法继续监控
```

**修复：**
```bash
# 重启 filewatch
# 通过 RPC 调用 filewatch.stop 然后 filewatch.start
# 或者直接 mindx restart
```

---

## 四、更新相关

### 4.1 自动更新检查失败

```
触发日志: "auto-update: check failed" / "auto-update: download and install failed"
caller:   svc/daemon.go:548 / 556
严重度:    ℹ️ Info（自动更新不工作，手动更新仍可用）
```

**常见原因：**
- 网络无法访问 GitHub Releases API（需要代理）
- 写权限不足（~/.mindx/bin/ 不可写）

**修复：**
```bash
# 手动更新
mindx update
# 或检查网络
curl -I https://api.github.com/repos/DotNetAge/mindx/releases/latest
```

---

## 五、日志轮转问题

### 5.1 lumberjack rotate 失败

```
触发日志: "WARNING: lumberjack write/rotate failed, using simple file writer"
严重度:    🟡 Warning（日志仍在写入，但不支持轮转）
```

**根因：** macOS sandbox/quarantine 属性阻止了文件重命名操作。

**修复：**
```bash
# 清理 quarantine 属性
xattr -dr com.apple.quarantine ~/.mindx/logs/
xattr -dr com.apple.provenance ~/.mindx/logs/
mindx restart
```

---

## 六、快速诊断决策树

```
Daemon 启动不了？
├── "address already in use"     → 见 1.1 端口冲突
├── "knowledge-graph unavailable"→ 见 1.3 Graph DB
├── "kvstore init failed"       → 见 1.4 KVStore
└── 其他                          → 检查 error.log 最后 50 行

Daemon 在跑但响应慢？
├── CPU > 80%                    → 见 references/resource-exhaustion.md
├── 内存持续增长                  → 见 references/resource-exhaustion.md
├── 大量 "request failed"        → 见 2.2 通用请求失败
└── "context deadline exceeded"  → 见 references/api-timeout.md

功能缺失？
├── GraphRAG 不工作              → 见 1.3 Knowledge Graph
├── 定时任务不执行                → 见 1.2 Scheduler
├── Token 统计为空               → 见 1.4 KVStore + 2.4
└── WebUI 打不开                 → 见 3.1 WebUI 目录
```
