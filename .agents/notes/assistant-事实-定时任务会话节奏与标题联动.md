# 定时任务的会话节奏与标题联动

## 规则（用户拍板）

- 重复任务（cron_expr）：绑定一条任务专属会话，每期在同一会话续跑，历史连续。
- 一次性任务（scheduled_at / one_shot）：不在创建时预建会话，到点执行时新开一条。

## 实现要点

- `ScheduleEntry.Title`：任务名，同时是绑定会话的标题；创建会话时经
  `ensureScheduleSession(..., preferredTitle)` 直接落成会话标题，绕过模型提炼。
- 编辑改名：`renameScheduleSessionIfSafe` 仅在会话仍是任务名镜像（无消息、标题为空
  或等于旧任务名）时调用 `RenameSession`，用户手改过的会话名保留。
- 一次性任务不预建会话，避免「刚建完任务就多出一个空会话」；执行侧 `OneShot`
  分支强制 `ensureScheduleSession(..., "", ...)` 新开。
- Agent 的 Cron 工具直写存储会留下 `"new"` 哨兵：执行侧解析出会话后必须
  `BindSession` 写回（`updateRunStatus` 读改写，避免覆盖执行统计），否则重复任务
  每期都会新开会话。
- 新建时前端绝不传 `session_id`：曾把用户当前对话绑给任务，导致任务和进行中的
  会话串线（实证：创建后会话标题仍是旧对话名）。
- 有效期 `ExpiresAt`：调度器 `addJob` 跳过已过期条目、`executeJob` 二次兜底；
  条目保留，续期即可恢复。
