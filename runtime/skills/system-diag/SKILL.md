---
name: system-diag
description: >
  Diagnose MindX system health by analyzing runtime logs, resource metrics,
  and daemon status. Produces structured diagnosis reports with root cause
  identification and fix recommendations. Use when user reports system issues,
  daemon crashes, performance degradation, or requests proactive health checks.
  Complements `mindx doctor` (static checks) with AI-powered log analysis and
  cross-component correlation.
allowed-tools: bash read sub-agent collect-results
metadata:
  name_zh: 系统诊断
  name_zh-tw: 系統診斷
  description_zh: 通过分析运行时日志、资源指标和守护进程状态诊断 MindX 系统健康，输出结构化诊断报告与修复建议
  description_zh-tw: 通過分析運行時日誌、資源指標和守護進程狀態診斷 MindX 系統健康，輸出結構化診斷報告與修復建議
---

## Trigger Decision

Use this skill when:

- User asks to check system health, diagnose issues, or analyze logs
- User reports daemon crashes, restarts, or unresponsive behavior
- User experiences performance degradation (slow response, hangs)
- Sysops agent performs routine health checks

**Do NOT use** for:
- Static configuration validation — use `mindx doctor` instead
- Single bug fixes or code issues — fix directly
- Feature development — use `software-dev` skill

**Relationship to `mindx doctor`:**
| `mindx doctor` (CLI) | This Skill (AI) |
|----------------------|-----------------|
| Static rule checks (file exists? process running?) | Log content analysis + pattern recognition + correlation |
| Surface-level: **what** is wrong | Deep-level: **why** it happened + **how** to fix |

## Workflow

### Phase 1: Collect Evidence

Gather data across 5 dimensions. Each dimension failure does not block others.

```
A: Runtime Snapshot
   - mindx status                          → install/config/daemon state
   - curl http://localhost:{port}/api/health → service component health
   - ps aux | grep "mindx daemon"          → process resource usage

B: Logs
   - {workspace}/logs/mindx.log            → main log (all levels, JSON)
   - {workspace}/logs/error.log            → error log (ERROR+)
   - {workspace}/logs/daemon.log           → daemon stdout
   - {workspace}/logs/daemon.err.log       → daemon stderr
   - *.log.gz                              → rotated history (for backtracking)

C: Storage State
   - du -sh {workspace}/data/              → data directory size
   - du -sh {workspace}/data/models/       → model files size
   - ls {workspace}/*.db                   → bbolt database files
   - df -h {workspace}                     → disk space

D: System Resources
   - ulimit -n                            → file descriptor limit
   - vm_stat (macOS) / free -h (Linux)    → memory pressure
   - launchctl list | grep mindx          → launchd registration

E: Network (optional)
   - lsof -i :{port} -P                  → port listening state
```

> **Note:** `{workspace}` resolves to `~/.mindx` on macOS/Linux, `%APPDATA%\mindx` on Windows. The port comes from user config or defaults.

### Phase 2: Analyze Logs

MindX logs use **uber-go/zap JSON format**. Read and correlate — do not just scan line by line.

#### 2.1 Reconstruct Timeline

Sort all events by `ts`, map daemon lifecycle key nodes:

```
[startup] → [scheduler?] → [gateway] → [webserver?] → [running...] → [anomaly?]
```

#### 2.2 Error Frequency & Pattern Aggregation

Count each ERROR/WARN occurrence and time distribution:

```
gateway start failed        ×1   (23:01:02)
Scheduler failed to start   ×3   (~every 2 hours)
knowledge-graph unavailable ×0
```

#### 2.3 Cross-Component Correlation

Link seemingly independent errors to a single root cause:

```
Symptom A: "gateway start failed: bind: address already in use"
Symptom B: Stale PID found in process list
Inference: Previous daemon did not exit cleanly, port not released
Root cause: Daemon crash not handled by launchd KeepAlive correctly
```

#### 2.4 Trend Analysis

Compare rotated logs for degradation signals:

```
Today:   12 WARN events
Yesterday: 3 WARN events
Day before: 0 WARN events
→ Issue is worsening, needs attention
```

#### 2.5 Known Error Patterns

Load `references/error-patterns.md` for complete error→root cause→fix mapping.

Quick index for high-frequency patterns:

| Error Keyword | Likely Root Cause | Reference |
|---------------|-------------------|-----------|
| `address already in use` | Port conflict / stale process | See references |
| `too many open files` | File descriptor exhaustion | `references/resource-exhaustion.md` |
| `context deadline exceeded` | API timeout / network issue | See references |
| `knowledge-graph database unavailable` | Graph DB corruption / permissions | `references/storage.md` |
| `failed to initialize kvstore` | KVStore corruption / disk full | `references/storage.md` |
| `Scheduler failed to start` | Scheduler store corruption | See references |

### Phase 3: Produce Diagnosis Report

Output structured report using this template:

```markdown
# MindX System Diagnosis Report
**Time**: {timestamp}
**Daemon Uptime**: {uptime}
**Log Coverage**: {log_time_range}

---

## Critical (Immediate Action Required)

### {N}. {Issue Title}
- **Symptom**: {observable behavior}
- **Evidence**: `{logfile}:{line}` — `{raw log excerpt}`
- **Root Cause**: {logical deduction from evidence}
- **Impact**: {affected features}
- **Fix**: {concrete steps}
- **Prevention**: {recommendation to avoid recurrence}

## Warning (Address Soon)

{Same format as above}

## Info (Optimization)

{Non-issues, improvement opportunities}

## System Resource Overview

| Metric | Value | Status |
|--------|-------|--------|
| Daemon Memory | {RSS} MB | 🟢/🟡/🔴 |
| Daemon CPU | {cpu%} | 🟢/🟡/🔴 |
| Disk Usage | {used}/{total} ({pct}%) | 🟢/🟡/🔴 |
| Log Size | {size} | 🟢/🟡/🔴 |
| File Descriptors | {open}/{limit} | 🟢/🟡/🔴 |
| Restarts (24h) | {count} | 🟢/🟡/🔴 |

## Summary

{One-paragraph overall health assessment + top 1-2 priority items}
```

### Phase 4: Execute Fixes (If Authorized)

Only when user explicitly requests (`--fix` or "fix it"):

1. **Safe fixes** — execute directly: kill stale processes, free ports, trim oversized logs
2. **Config changes** — show diff before applying
3. **Storage operations** — backup first before touching databases/data files
4. **Uncertain cases** — mark as "requires manual intervention", do not guess

## Principles

1. **Evidence-based** — every conclusion must cite specific log lines; never speculate without evidence
2. **Single root cause first** — assume one root cause explains all symptoms until contradictions force multi-cause analysis
3. **Distinguish symptom from cause** — "gateway failed" is symptom, "port occupied" is cause, "stale process from crash" is root cause
4. **Time-sensitive reasoning** — clustered errors in short time > scattered single events
5. **No alarmism** — a single WARN is not a system failure; use frequency + trend to gauge severity
6. **Actionable output** — every diagnosis must include concrete fix steps; never say "might be an issue"

## Anti-Patterns

- Do not skip evidence collection and diagnose from assumptions alone
- Do not treat every WARN as critical — use frequency and trend context
- Do not produce vague recommendations like "check the config" — be specific about what to check and why
- Do not modify data files or databases without explicit user authorization
- Do not confuse correlation with causation — two errors occurring near each other are not necessarily related
- Do not omit the summary section — users need a clear prioritized action list
