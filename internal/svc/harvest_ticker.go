package svc

// 收割轮询器（持久化收割的执行端）：无状态 ticker 周期扫描全部会话目录内
// 的 session-harvest.json 收割意图，每轮从磁盘重新推导全部状态——不持有
// 任何跨轮内存态，任何一轮卡死或 daemon 重启后下一轮自然恢复，无需专门的
// 启动重锚钩子（bega team 11-05 收割汇报蒸发事故的根治：收割执行不再依赖
// 任何进程内长命 goroutine）。
//
// 每轮处理：
//   - reporting 停滞超过 5 分钟（回灌轮中途崩溃）→ 回退 pending 重投；
//   - pending 逐目标独立判定（控制平面终态 > 落盘事实兜底）：目标落定或达
//     自身登记时间上限（30 分钟）即进入本批上报，未落定未超限的目标留在
//     意图中继续轮询——后合并的目标不被先登记目标的时长压缩窗口；
//   - 上报批次置 reporting → 经 runBackgroundTurn 回灌主会话（会话串行
//     队列，与用户消息天然顺序化）→ 成功移除批次条目（无余留则删除文件）；
//     失败 attempts+1 回 pending 重投（连续失败达上限放弃并回灌放弃说明）。
//
// 落定判定语义：子会话落定 = 控制平面无运行中句柄 + 尾消息为 assistant
//（有最终交付即算完成，消除「事后收割」对基线机制的死局）+ 子会话目录
// 无挂起 sidecar（未决提问/授权）。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goagent "github.com/DotNetAge/goagent"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/goharness/tools"
)

// 收割轮询器的周期与阈值常量。
const (
	// harvestCancelClientID 是收割回灌在 clientCancels 中的虚拟客户端键：
	// 收割回灌不属于任何客户端，message.cancel 按会话取消时需一并查此集合。
	harvestCancelClientID = "__harvest__"

	// harvestScanInterval 扫描周期：无状态轮询，每轮全量重推导，10s 足够
	// 及时且开销可忽略（每会话一次 os.ReadFile + os.Stat）。
	harvestScanInterval = 10 * time.Second

	// harvestTotalCap 收割总上限：超限即回灌「部分结果 + 诊断」汇报，
	// 绝不静默等待；回灌后意图文件删除，超时状态自然持久化（重启不重复等）。
	harvestTotalCap = 30 * time.Minute

	// harvestReportingStall reporting 停滞回退阈值：置 reporting 后超过此
	// 时长仍未见 onDone（回灌轮崩溃 / daemon 重启）即回退 pending 重投。
	harvestReportingStall = 5 * time.Minute

	// harvestMaxAttempts 回灌连续失败上限：达到即放弃并删除意图文件
	//（子任务结果保留在子会话落盘记录中，不丢失）。
	harvestMaxAttempts = 5
)

// harvestTicker 收割轮询主循环：随 daemon 生命周期启停，tick 自身 panic
// 不击穿循环（recover 收敛，下一轮继续）。
func (d *Daemon) harvestTicker(ctx context.Context) {
	ticker := time.NewTicker(harvestScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.harvestTick(ctx)
		}
	}
}

// harvestTick 单轮扫描：枚举全部会话，逐个探测收割意图文件并处理。
// 枚举或存储故障只记日志，下轮重试——单轮失败不影响后续轮次。
func (d *Daemon) harvestTick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Error("harvest ticker: tick panic", fmt.Errorf("%v", r))
		}
	}()
	sessDB := d.app.SessDB()
	if sessDB == nil {
		return
	}
	sessions, err := goharnesssession.ListSessions(ctx, sessDB)
	if err != nil {
		d.logger.Warn("harvest ticker: list sessions failed", "error", err)
		return
	}
	for _, info := range sessions {
		path := d.harvestIntentPath(info.SessionID)
		if path == "" {
			continue
		}
		intent := d.loadHarvestIntent(path)
		if intent == nil {
			continue // 无意图（文件不存在）或损坏（无恢复价值，等重写）
		}
		d.processHarvestIntent(ctx, intent, path)
	}
}

// harvestIntentUnchangedSince 复核意图文件自本轮加载后未被并发修改
// （Record 合并新目标 / 上一轮回灌的 onDone 清理）。修改过或文件已消失
// 返回 false——本轮放弃写盘，下一轮以最新文件重新推导，避免覆盖丢失。
func (d *Daemon) harvestIntentUnchangedSince(path string, loadedAt time.Time) bool {
	cur := d.loadHarvestIntent(path)
	return cur != nil && cur.UpdatedAt.Equal(loadedAt)
}

// harvestBatchEqual 判断两个上报批次快照是否一致（同序同元素）：回灌回执
// 到达时用于校验意图文件中的批次仍是本轮回灌的那一批，防止迟到回执操作
// 新一轮上报的批次。
func harvestBatchEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// unregisterHarvestBatch 回收一批目标的控制平面条目：结果已送达主会话
// （或本批已落盘放弃）后条目使命完成，常驻 daemon 下无人回收会无限累积
// （第三轮审计）。无条件回收、不区分终态——在跑任务由自身执行循环独立
// 存活，登记条目只是结果缓存，移除不影响执行；若保留非终态条目，目标
// 移出意图后无人再回收，反成永久泄漏（第四轮审计：终态过滤是死逻辑）。
func (d *Daemon) unregisterHarvestBatch(ids []string) {
	mgr := goagent.DefaultRuntimeManager()
	for _, id := range ids {
		mgr.Unregister(id)
	}
}

// processHarvestIntent 处理单个收割意图：状态机推进（pending → reporting →
// 删除）与落定判定。任何失败只记日志并保留/回退意图文件，下轮重试。
func (d *Daemon) processHarvestIntent(ctx context.Context, intent *harvestIntentFile, path string) {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Error("harvest ticker: process panic", fmt.Errorf("%v", r),
				"main_session", intent.MainSessionID)
		}
	}()
	now := time.Now()
	loadedAt := intent.UpdatedAt // 本轮加载时间戳：写盘前并发复核的基准

	// reporting 停滞回退：置 reporting 后超时未见 onDone（回灌轮崩溃或
	// daemon 重启）→ 回退 pending 重投。写盘成功才继续按 pending 处理。
	if intent.Status == harvestStatusReporting {
		if now.Sub(intent.UpdatedAt) < harvestReportingStall {
			return // 回灌轮进行中（onDone 未到），本轮跳过
		}
		intent.Status = harvestStatusPending
		intent.ReportingBatch = nil // 整批重新判定上报
		intent.UpdatedAt = now
		if !d.harvestIntentUnchangedSince(path, loadedAt) {
			return // 并发窗口：登记已被 Record 修改，下一轮重新推导
		}
		if err := d.writeHarvestIntent(path, intent); err != nil {
			d.logger.Error("harvest ticker: revert reporting failed", err,
				"main_session", intent.MainSessionID)
			return
		}
		d.logger.Info("harvest ticker: reporting stalled, reverted to pending",
			"main_session", intent.MainSessionID, "attempts", intent.Attempts)
		// CAS 基准刷新为本轮回退写盘后的时间戳：否则下方「置 reporting」复核
		// 仍以回退前的旧时间戳比对，必然失败返回，本轮回退白做、重投被推迟
		// 一个扫描周期。
		loadedAt = intent.UpdatedAt
	} else if intent.Status != harvestStatusPending {
		return // 未知状态：跳过（不删文件，留待人工排查）
	}

	// pending：逐目标独立判定（每轮从磁盘与控制平面重新推导，无缓存）。
	// 已落定或已达自身登记时间上限的目标进入上报批次；未落定未超限的目标
	// 留在意图中继续轮询——收割上限按各目标自身登记时间起算，后合并的
	// 新目标不被先登记目标的时长压缩窗口（第三轮审计：文件级 CreatedAt
	// 一刀切曾把临近上限合并的新任务误标 timeout 并随整批删除，结果无人收割）。
	for i := range intent.SubSessions {
		if intent.SubSessions[i].AgentName == "" && intent.SubSessions[i].SessionID != "" {
			// 跨重启恢复的登记缺展示信息：按子会话 meta 补全（仅内存，不回写），
			// 否则回灌报告输出「- （会话 xxx）：…」无法转述是哪个子代理。
			if meta, merr := d.getSessionMeta(intent.SubSessions[i].SessionID); merr == nil && meta != nil {
				intent.SubSessions[i].AgentName = meta.AgentName
			}
		}
	}
	statuses := make([]tools.HarvestStatus, len(intent.SubSessions))
	reportIdx := make([]int, 0, len(intent.SubSessions))
	for i := range intent.SubSessions {
		registeredAt := intent.SubSessions[i].RegisteredAt
		if registeredAt.IsZero() {
			registeredAt = intent.CreatedAt // 旧格式意图（无 registered_at）兜底
		}
		st, settled := d.classifyHarvestTarget(ctx, intent.SubSessions[i])
		statuses[i] = st
		if settled || now.Sub(registeredAt) >= harvestTotalCap {
			reportIdx = append(reportIdx, i)
		}
	}
	if len(reportIdx) == 0 {
		return // 全部未落定且均未达各自上限：继续轮询
	}
	// 上报集合中的未落定条目按 timeout 标注（绝不静默等待）；blocked 自带
	// 等待说明保留原样。classify 对未落定条目也填非空 Status（running/
	// blocked），跳过条件必须按终态枚举，不能按「Status 非空」（审计教训：
	// 回灌报告曾输出「未完成——」空原因）。
	reported := make([]tools.HarvestStatus, 0, len(reportIdx))
	for _, i := range reportIdx {
		switch statuses[i].Status {
		case "completed", "failed", "blocked":
		default:
			statuses[i].Status = "timeout"
			statuses[i].Error = fmt.Sprintf("收割时长超过 %s 仍未落定：该子任务可能仍在后台执行或已中断。"+
				"下一步：先到其子代理会话中查看进展；若确认已中断，向该会话发起继续（重试）让子代理从断点自动修复，不要原样重新派发任务", harvestTotalCap)
		}
		reported = append(reported, statuses[i])
	}

	// 幂等状态机：先置 reporting（写盘成功才继续回灌），再发起回灌轮。
	// 「置 reporting」与「删除文件」之间的崩溃窗口由停滞回退覆盖，最坏
	// 重复注入一次汇总轮，无数据损坏。批次快照（ReportingBatch）记录本次
	// 上报的任务集合：回灌进行中合并进来的新目标不属于本批，回灌成功后
	// 只移除批次条目、余下目标回 pending 继续收割。
	batch := make([]string, 0, len(reported))
	for _, st := range reported {
		batch = append(batch, st.TaskID)
	}
	intent.Status = harvestStatusReporting
	intent.ReportingBatch = batch
	intent.UpdatedAt = time.Now()
	if !d.harvestIntentUnchangedSince(path, loadedAt) {
		return // 并发窗口：登记已被 Record 合并，下一轮以完整视图重新判定
	}
	if err := d.writeHarvestIntent(path, intent); err != nil {
		d.logger.Error("harvest ticker: mark reporting failed", err,
			"main_session", intent.MainSessionID)
		return
	}

	sid := intent.MainSessionID
	// 执行 agent 按会话绑定裁定（与 defaultHandler 回退一致）：
	// 会话绑定 agent_name > 全局当前 agent。
	agentName := ""
	if meta, err := d.getSessionMeta(sid); err == nil && meta != nil {
		agentName = meta.AgentName
	}
	if agentName == "" {
		agentName = d.app.CurrentAgentName()
	}

	content := formatHarvestReport(tools.HarvestReport{MainSessionID: sid, Targets: reported})
	d.logger.Info("harvest ticker: feeding back to main session",
		"session_id", sid, "agent", agentName, "reported", len(reported),
		"total", len(intent.SubSessions), "attempts", intent.Attempts)

	d.runBackgroundTurn(sid, agentName, content, harvestCancelClientID, func(err error) {
		if err == nil {
			// 结果已送达：无条件回收本批目标的控制平面条目（见
			// unregisterHarvestBatch）——回收只依赖「送达成功」这一事实，
			// 与下方意图文件的清理成败无关。
			d.unregisterHarvestBatch(batch)
			// 迟到的回执防护：意图已不在 reporting（5 分钟停滞回退已兜底
			// 重投）或批次快照已被新一轮上报覆盖时，本次清理必须放弃——
			// 否则会把新一轮批次的条目误当本批移除，尚未送达的结果被静默
			// 丢弃（第四轮审计）。旧轮回的残留统一由停滞回退收敛。
			latest := d.loadHarvestIntent(path)
			if latest == nil {
				d.logger.Info("harvest ticker: harvest done",
					"session_id", sid, "targets", len(statuses))
				return
			}
			if latest.Status != harvestStatusReporting || !harvestBatchEqual(latest.ReportingBatch, batch) {
				d.logger.Info("harvest ticker: stale feedback ignored",
					"session_id", sid, "reported", len(batch))
				return
			}
			// 只移除本批已上报的目标——回灌进行中合并进来的新目标回
			// pending 继续收割，绝不随文件一起被静默丢弃。写盘/删除前以
			// 加载时 updated_at 做并发复核：Record 恰在窗口内合并过新目标
			// 则放弃本次清理（文件仍处 reporting，由停滞回退兜底重报一次），
			// 绝不覆盖丢登记（审计修复）。
			base := latest.UpdatedAt
			reported := make(map[string]bool, len(latest.ReportingBatch))
			for _, id := range latest.ReportingBatch {
				reported[id] = true
			}
			remaining := latest.SubSessions[:0]
			for _, sub := range latest.SubSessions {
				if !reported[sub.TaskID] {
					remaining = append(remaining, sub)
				}
			}
			if len(remaining) == 0 {
				if !d.harvestIntentUnchangedSince(path, base) {
					return // 并发窗口：放弃本次清理，意图仍处 reporting，由 5 分钟停滞回退兜底（最坏整批重报一次）
				}
				if rerr := d.deleteHarvestIntent(path); rerr != nil {
					d.logger.Error("harvest ticker: delete intent failed", rerr,
						"main_session", sid)
				} else {
					d.logger.Info("harvest ticker: harvest done",
						"session_id", sid, "targets", len(statuses))
				}
				return
			}
			latest.SubSessions = remaining
			latest.Status = harvestStatusPending
			latest.ReportingBatch = nil
			latest.Attempts = 0 // 新批次独立计数：Attempts 跨批次累计会让余留目标的失败余量被前期失败吃掉，达上限整文件放弃时连带丢掉从未上报的目标
			latest.UpdatedAt = time.Now()
			if !d.harvestIntentUnchangedSince(path, base) {
				return // 并发窗口：放弃本次改写，由 5 分钟停滞回退兜底（整批重报一次）
			}
			if werr := d.writeHarvestIntent(path, latest); werr != nil {
				d.logger.Error("harvest ticker: carry-over rewrite failed", werr,
					"main_session", sid, "remaining", len(remaining))
			} else {
				d.logger.Info("harvest ticker: batch reported, carry-over targets requeued",
					"session_id", sid, "reported", len(reported), "remaining", len(remaining))
			}
			return
		}
		// 回灌失败（含排队取消）：attempts+1 回 pending 重投；连续失败达
		// 上限即放弃——意图文件删除，结果保留在子会话落盘记录中。
		// 写盘/删除前并发复核同上（审计修复）。
		latest := d.loadHarvestIntent(path)
		if latest == nil {
			return // 意图已被并发处理/删除，无需重投
		}
		// 迟到的失败回执防护（与成功路径同源）：意图已不在 reporting 或批次
		// 快照已被新一轮上报覆盖时，本次失败不计入重试——旧轮回的失败会
		// 错误消耗新批次的失败余量，甚至误触发放弃说明（第四轮审计）。
		if latest.Status != harvestStatusReporting || !harvestBatchEqual(latest.ReportingBatch, batch) {
			return
		}
		base := latest.UpdatedAt
		latest.Attempts++
		if latest.Attempts >= harvestMaxAttempts {
			if !d.harvestIntentUnchangedSince(path, base) {
				return // 并发窗口：暂不放弃，意图仍处 reporting，由 5 分钟停滞回退兜底重投
			}
			// 放弃本批重投，但保留从未上报的目标继续独立收割（放弃只针对
			// 本批——整文件删除会连带丢掉从未轮到上报的目标）。
			gaveUp := make(map[string]bool, len(latest.ReportingBatch))
			for _, id := range latest.ReportingBatch {
				gaveUp[id] = true
			}
			kept := latest.SubSessions[:0]
			for _, sub := range latest.SubSessions {
				if !gaveUp[sub.TaskID] {
					kept = append(kept, sub)
				}
			}
			if len(kept) == 0 {
				if rerr := d.deleteHarvestIntent(path); rerr != nil {
					d.logger.Error("harvest ticker: delete intent failed", rerr,
						"main_session", sid)
				} else {
					d.unregisterHarvestBatch(batch) // 本批已落盘放弃：回收其控制平面条目
				}
			} else {
				latest.SubSessions = kept
				latest.Status = harvestStatusPending
				latest.ReportingBatch = nil
				latest.Attempts = 0
				latest.UpdatedAt = time.Now()
				if werr := d.writeHarvestIntent(path, latest); werr != nil {
					d.logger.Error("harvest ticker: give-up keep rewrite failed", werr,
						"main_session", sid, "kept", len(kept))
				} else {
					d.unregisterHarvestBatch(batch) // 本批已落盘放弃：回收其控制平面条目
				}
			}
			d.logger.Error("harvest ticker: feedback failed repeatedly, giving up this batch",
				err, "session_id", sid, "attempts", latest.Attempts)
			// 放弃不等于静默：强制回灌一条说明（尽力而为，失败仅记日志），
			// 兑现 CollectResults 回执「不会静默丢失」的承诺（第三轮审计：
			// 连续失败直接删文件会让已向用户承诺「结果自动送达」的主会话永久沉默）。
			notice := "[系统收割] 收割结果多次自动送达失败，已停止本轮自动送达。相关子任务的结果仍完整保留在其会话记录中，请直接查看对应子代理的对话获取结果；如需继续跟进可重新发起任务。"
			d.runBackgroundTurn(sid, agentName, notice, harvestCancelClientID, func(nerr error) {
				if nerr != nil {
					d.logger.Warn("harvest ticker: give-up notice delivery failed",
						"error", nerr, "session_id", sid)
				}
			})
			return
		}
		latest.Status = harvestStatusPending
		latest.UpdatedAt = time.Now()
		if !d.harvestIntentUnchangedSince(path, base) {
			return // 并发窗口：意图仍处 reporting，由 5 分钟停滞回退兜底重投
		}
		if werr := d.writeHarvestIntent(path, latest); werr != nil {
			d.logger.Error("harvest ticker: revert to pending failed", werr,
				"main_session", sid)
		} else {
			d.logger.Info("harvest ticker: feedback failed, reverted to pending",
				"session_id", sid, "attempts", latest.Attempts, "error", err)
		}
	})
}

// classifyHarvestTarget 判定单个收割目标的当前状态。
// 返回（状态条目, 是否已落定）；未落定时条目 Status 为 running/blocked
// （仅用于超限标注与诊断）。
func (d *Daemon) classifyHarvestTarget(ctx context.Context, sub harvestIntentSub) (tools.HarvestStatus, bool) {
	st := tools.HarvestStatus{
		TaskID:    sub.TaskID,
		SessionID: sub.SessionID,
		AgentName: sub.AgentName,
		Title:     sub.Title,
	}

	// 主依据：控制平面运行实例（受理登记 → runTask 结算，进程内必有条目）。
	rt, ok := goagent.DefaultRuntimeManager().Get(sub.TaskID)
	if ok {
		switch rt.Status() {
		case goagent.StatusCompleted:
			st.Status = "completed"
			st.Result = rt.Result()
			return st, true
		case goagent.StatusPending, goagent.StatusRunning:
			if d.subSessionBlocked(sub.SessionID) {
				st.Status = "blocked"
				st.Error = "子代理在等待回答或授权。下一步：告知用户到对应子代理会话中回答问题或完成授权，子代理会自动继续"
			} else {
				st.Status = "running"
			}
			return st, false
		default:
			// Failed / Cancelled 及其余终态
			reason := rt.Reason()
			if reason == "" {
				reason = rt.Status().String()
			}
			st.Status = "failed"
			st.Error = reason
			return st, true
		}
	}

	// 句柄不在控制平面：daemon 重启（进程内登记全丢）或终态条目被清理。
	// 按 session_id 查落盘事实——子会话尾消息为 assistant 即有最终交付，
	// 最后一条 assistant 消息即结果（不再要求「收割启动后有新增」，
	// 消除事后收割对基线机制的死局；daemon 重启也不丢）。
	if sub.SessionID == "" {
		st.Status = "failed"
		// 无会话锚点时续跑无处着力，重新派发是唯一路径——文案直接给出该方案。
		st.Error = "任务句柄不存在且无子代理会话锚点，无法恢复或续跑；请以原任务描述重新派发 SubAgent 任务"
		return st, true
	}
	sessDB := d.app.SessDB()
	if sessDB == nil {
		st.Status = "running"
		return st, false
	}
	msgs, err := sessDB.Get(ctx, sub.SessionID)
	if err != nil {
		// 读取失败（含会话已被删除）：不落定，继续轮询，由总上限兜底。
		st.Status = "running"
		return st, false
	}
	last := ""
	for _, msg := range msgs {
		if msg.Role == "assistant" && msg.Content != "" {
			last = msg.Content
		}
	}
	if last != "" {
		st.Status = "completed"
		st.Result = last + "\n（结果自会话落盘记录恢复）"
		return st, true
	}
	// 会话无 assistant 落盘产出：先看是否在等待用户回答/授权（跨重启后
	// 控制平面句柄已丢，挂起 sidecar 是唯一证据），否则任务可能仍在执行
	//（终态条目被清理）或已中断。不落定、继续轮询，由总上限兜底。
	if d.subSessionBlocked(sub.SessionID) {
		st.Status = "blocked"
		st.Error = "子代理在等待回答或授权。下一步：告知用户到对应子代理会话中回答问题或完成授权，子代理会自动继续"
		return st, false
	}
	st.Status = "running"
	return st, false
}

// sweepStaleWaitMarkers daemon 启动时清扫全部会话目录的陈旧等待标记
// （session-wait.json）：等待是进程内阻塞 goroutine，daemon 重启后必然
// 消亡，残留标记只会让收割轮询把已中断的会话误判为「等待用户输入」。
// 清扫必须挂在启动时点、不能挂在会话加载时——收割轮询器从不 Load 会话，
// 无人打开的会话残留永远清不掉；而 defaultHandler 对活跃等待中的会话
// Load 反而会误删正在生效的标记（两个方向都失准，第四轮审计）。启动
// 时点此刻任何等待都不可能存活，统一清除绝对安全。
func (d *Daemon) sweepStaleWaitMarkers(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Error("sweep wait markers: panic", fmt.Errorf("%v", r))
		}
	}()
	sessDB := d.app.SessDB()
	if sessDB == nil {
		return
	}
	sessions, err := goharnesssession.ListSessions(ctx, sessDB)
	if err != nil {
		d.logger.Warn("sweep wait markers: list sessions failed", "error", err)
		return
	}
	for _, info := range sessions {
		dir := d.sessionDirOf(info.SessionID)
		if dir == "" {
			continue
		}
		pp := filepath.Join(dir, "session-wait.json")
		if _, err := os.Stat(pp); err != nil {
			continue
		}
		if err := os.Remove(pp); err != nil && !os.IsNotExist(err) {
			d.logger.Warn("sweep wait markers: remove failed", "error", err, "path", pp)
		} else {
			d.logger.Info("sweep wait markers: removed stale marker", "path", pp)
		}
	}
}

// subSessionBlocked 判断子会话是否正处于挂起等待（三类信号任一即成立）：
//   - 未决提问 sidecar（session-ask-user.json）：主会话路径挂起即写盘；
//   - 未决授权 sidecar（session-permission.json）：授权处理开始前写盘；
//   - 等待标记（session-wait.json）：子会话提问/授权冒泡路径进入阻塞等待
//     时写盘、离开时删除（goharness waitForAskUserDecision /
//     waitForPermissionDecision）——等待期上述两个 sidecar 均不存在
//     （授权 sidecar 被 TakePendingPermission 取走、提问 sidecar 子会话
//     冒泡路径从不写），若无此标记，等待中的子会话会被误判 running。
//
// 挂起的子会话永远不会「完成」，判定为 blocked 继续轮询，由各目标登记
// 时间上限兜底提示用户处理。
func (d *Daemon) subSessionBlocked(sessionID string) bool {
	dir := d.sessionDirOf(sessionID)
	if dir == "" {
		return false
	}
	for _, name := range []string{"session-ask-user.json", "session-permission.json", "session-wait.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// formatHarvestReport 把收割报告格式化为「系统收割」消息正文：
// 内容进主会话上下文，LLM 在此轮才得知各子任务结局并向用户转述。
// 每个条目带子代理名称与会话锚点（session_id 落盘持久，LLM 后续轮次
// 可据此直达对应子代理对话）。
func formatHarvestReport(report tools.HarvestReport) string {
	var b strings.Builder
	b.WriteString("[系统收割] 以下子任务已结算（或达到各自收割时长上限），是它们的结果汇总。请基于这些结果继续推进，并把各任务的结局清楚地告知用户；仍有个别任务未列入的，其结果会在落定后继续自动送达。\n")
	for _, t := range report.Targets {
		name := t.AgentName
		if t.Title != "" {
			name = fmt.Sprintf("%s（%s）", t.AgentName, t.Title)
		}
		switch t.Status {
		case "completed":
			fmt.Fprintf(&b, "\n- %s（会话 %s）：已完成\n%s", name, t.SessionID, t.Result)
		case "failed":
			fmt.Fprintf(&b, "\n- %s（会话 %s）：失败，原因：%s。下一步：向该子代理会话发起继续（重试）可保留已有进度自动修复；"+
				"仅当确认任务需要全新开始时才重新派发", name, t.SessionID, t.Error)
		default:
			// blocked / timeout：未落定，条目的 Error 已含具体说明与下一步方案
			fmt.Fprintf(&b, "\n- %s（会话 %s）：未完成——%s", name, t.SessionID, t.Error)
		}
	}
	return b.String()
}
