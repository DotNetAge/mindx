# Cron 工具直写 "new" 哨兵：定时任务到点即失败（不是没调度）

- 日期：2026-11-08
- 现象：用户装好任务（大秘自己建的 18:05 一次性任务），重启电脑后到点毫无动静。用户判断「完全没执行」。

## 实证链（daemon.log）

- `17:36:28 scheduler started {"jobs": 1}`：重启后任务被正常重载，调度器工作正常。
- `18:05:00.066 schedule job fired {"id":"0060abc9"}`：准点触发，**「没执行」是假象**。
- `18:05:00.107 background turn: execution started {"session_id":"new"}`：绑定会话是字面量 `new`。
- `18:05:00.111 failed to load session`：`会话 "new" 未找到: 未找到对应角色的会话`，连模型都没调用。
- 任务文件落盘 `session_id: "new"`、`last_status: failed`，一次性任务随即自动禁用。

## 根因：两条创建通路的口径分裂

- GUI/CLI 走 RPC `schedule.add`，入口 `ensureScheduleSession` 会把空 ID 与 `"new"` 落实成真实会话再落库（handler_schedule.go）。
- Agent 的 `Cron` 工具（internal/tools/cron.go）**直接写 scheduler store**，`session_id` 原样保存；其参数说明还明确允许传 `"new"`（「为空或 new 时每次运行创建新会话」），模型照做 → `"new"` 落库。
- 执行侧 scheduled_exec.go 只兜底 `session_id == ""`，注释断言「"new" 绝不落库，此处不认」——该前提被工具直写打破，于是 `"new"` 被当作真会话 ID 去加载，必然失败。

## 修复与验收

- 修复：`runScheduledJob` 把 `""` 与 `"new"` 同视为未绑定，运行前 `ensureScheduleSession` 落实会话线；仅内存态、不回写存储。
- `make restart` 后用直写 store 的探针任务实跑（与 Cron 工具同一落库形态）：
  - 第一次探针漏了 `project_dir` → 报 `project_dir 不能为空`，说明哨兵分支已生效（不再报「会话 new 未找到」），同时暴露第二个坑；
  - 第二次探针带 `project_dir` → 18:39:18 触发，会话落实为真实 ULID，LLM 单轮完成，`last_status: success`。
- 验收后已删除探针任务文件与探针会话。

## 后续：project_dir 同类坑一并修掉

- 现象：`Cron` 工具不传 `project_dir` 时创建成功、到点才炸「project_dir 不能为空（工作目录分片存储要求会话归属明确）」。
- 修法（internal/tools/cron.go 创建入口）：`显式 project_dir > 当前会话 ProjectDir()（tools.GetToolContext(ctx).Session）` 回填；两者都没有就创建阶段直接拒绝，返回人话指引（「定时任务到点会新建专属会话，必须归属某个工作目录」）；update 时空串不再覆盖已有目录；参数说明同步改写。
- 回归测试 `internal/tools/cron_test.go`：缺目录拒绝、显式目录落库、会话目录回填（用 `mindxses.NewRoutedSessionStore` + `ghsession.New` 造真会话）三条；原有用例补 `project_dir`。
- 部署：`make restart`（21:07 新二进制上线，`vcs.modified=true`）。

## 下次我看到「某个定时任务到点没动静」，我将

先 grep `schedule job fired` 定位到点是「没调度」还是「触发后秒失败」；若是后者，看 `background turn` 行里的 session_id 是否为哨兵（`new`/空）或 project_dir 是否为空，再对照该任务是由 `Cron` 工具还是 RPC 创建。
