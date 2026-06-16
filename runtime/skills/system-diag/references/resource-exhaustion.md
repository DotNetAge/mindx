# 资源耗尽诊断

> 诊断 Daemon 进程的资源使用状况：CPU、内存、磁盘、文件描述符。

## 采集指标

```bash
# macOS 完整资源快照
echo "=== Process ==="
ps aux | grep "[m]indx daemon"

echo "=== Memory ==="
vm_stat | head -10
ps -o pid,rss,vsz,pcpu,etime,command -p $(pgrep -f "mindx daemon")

echo "=== File Descriptors ==="
lsof -p $(pgrep -f "mindx daemon") 2>/dev/null | wc -l
ulimit -n

echo "=== Disk ==="
df -h ~/.mindx/
du -sh ~/.mindx/*/ 2>/dev/null | sort -rh | head -10

echo "=== Log Sizes ==="
du -sh ~/.mindx/logs/* 2>/dev/null | sort -rh
```

## 判定阈值

| 指标 | 🟢 健康 | 🟡 注意 | 🔴 危险 |
|------|--------|--------|---------|
| **RSS 内存** | < 200 MB | 200-500 MB | > 500 MB |
| **CPU 占用** | < 5% (空闲) | 5-30% | > 30% 持续 |
| **FD 数量** | < 100 | 100-500 | > 500 |
| **磁盘使用率** | < 70% | 70-85% | > 85% |
| **日志总大小** | < 100 MB | 100MB-1GB | > 1 GB |
| **启动次数(24h)** | 0-1 次 | 2-5 次 | > 5 次 |

## 常见场景

### 场景 1: 内存持续增长（内存泄漏）

**症状：**
- RSS 从启动时的 ~100MB 逐步增长到 500MB+
- 增长速度不随请求量下降而减少
- 可能伴随 GC 压力（Go runtime 频繁 GC）

**可能根因：**

| 根因 | 证据特征 | 定位方法 |
|------|---------|---------|
| Session 数据堆积 | sessions/ 目录持续增长 | `du -sh ~/.mindx/sessions/*` 排序 |
| GraphRAG chunk 缓存膨胀 | memory/index 缓存未释放 | 检查 sharedMemory.Indexer() 的 Count 趋势 |
| bbolt mmap 增长 | KVStore/GraphDB 文件变大 | `ls -lh ~/.mindx/data/*.db` |
| Goroutine 泄漏 | goroutine 数量持续增长 | 通过 `/api/health` 或 pprof 检查 |

**修复操作：**
```bash
# 清理过期会话（保留最近7天）
find ~/.mindx/sessions/ -type d -mtime +7 -exec rm -rf {} + 2>/dev/null

# 清理旧的轮转日志（保留最近3天）
find ~/.mindx/logs/ -name "*.gz" -mtime +3 -delete 2>/dev/null

# 重启 daemon 释放内存
mindx restart
```

**预防措施：**
- 定期清理过期会话（建议加入 cron 或 scheduler 任务）
- 设置日志轮转策略（已内置 lumberjack MaxAge=30 天）
- 监控 RSS 趋势，超过 300MB 时预警

---

### 场景 2: CPU 持续偏高

**症状：**
- CPU > 30% 且不随请求结束下降
- 系统风扇高速运转
- 响应延迟增加

**可能根因：**

| 根因 | 证据特征 |
|------|---------|
| FileWatch 热循环 | `lsof -p {pid}` 显示大量 inotify/kqueue 监控 |
| 索引任务密集 | 日志中出现大量 indexing 相关 INFO |
| 死循环 / goroutine 泄漏 | CPU 高但无 I/O 活动 |

**修复操作：**
```bash
# 检查 goroutine 数量（如果有 pprof 端点）
curl http://localhost:{port}/debug/pprof/goroutine?debug=1 2>/dev/null | wc -l

# 如果 FileWatch 是元凶
# 通过 RPC 停止 filewatch，然后按需手动触发索引
```

---

### 场景 3: 磁盘空间不足

**症状：**
- `df -h` 显示 ~/.mindx/ 所在分区 > 85%
- 日志写入报错（虽然 simpleFileWriter 吞掉错误不暴露）
- bbolt 操作可能失败（mmap 需要磁盘空间）

**大文件嫌疑对象（按概率排序）：**

| 目录/文件 | 典型大小 | 清理安全度 |
|----------|---------|-----------|
| `logs/*.log.gz` | 几十 MB ~ 几 GB | ✅ 安全删除旧文件 |
| `sessions/` | 几 MB ~ 几百 MB | ⚠️ 可删过期的 |
| `data/models/` | ~169 MB（onnx 模型） | ❌ 不删 |
| `data/graph/*.db` | 几 MB ~ 几十 MB | ⚠️ 删除会丢失图数据 |
| `*.db`（KVStore） | 几 MB | ⚠️ 删除会丢失 score/token 数据 |

**修复操作：**
```bash
# 1. 安全清理：日志和过期会话
find ~/.mindx/logs/ -name "*.gz" -mtime +7 -delete
find ~/.mindx/sessions/ -type d -mtime +14 -exec rm -rf {} + 2>/dev/null

# 2. 如果还不够：检查模型缓存
du -sh ~/.mindx/data/models/*

# 3. 极端情况：清理 graph db（会丢失 GraphRAG 数据）
# 先备份！
cp -r ~/.mindx/data/graph/ ~/.mindx/data/graph-backup/
rm -rf ~/.mindx/data/graph/*.db
mindx restart
```

---

### 场景 4: 文件描述符耗尽

**症状：**
- 日志中出现 `too many open files`
- 新连接被拒绝
- FileWatch 失败

**诊断：**
```bash
# 当前 FD 使用量
pid=$(pgrep -f "mindx daemon")
echo "Open FDs: $(lsof -p $pid 2>/dev/null | wc -l)"
echo "Limit: $(ulimit -n)"

# FD 分布（哪些类型最多）
lsof -p $pid 2>/dev/null | awk '{print $NF}' | grep -E '^\.' | sed 's/.*\///' | sort | uniq -c | sort -rn | head -10
```

**常见 FD 消耗源：**

| 来源 | 典型 FD 数 | 解决方式 |
|------|-----------|---------|
| 日志文件句柄 | 2-5 | 正常 |
| bbolt 数据库文件 | 1-3 per db | 正常 |
| 网络连接 | 1 per active conn | 检查连接泄漏 |
| FileWatch 监控 | 1 per watched dir | 减少监控目录数 |
| **goroutine 泄漏** | **持续增长** | **见场景 1** |

**修复操作：**
```bash
# 提高系统限制（临时，重启后失效）
ulimit -n 65536
mindx restart

# 永久方案：写入 shell profile
echo "ulimit -n 65536" >> ~/.zshrc
```
