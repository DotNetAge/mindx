package svc

// 收割意图落盘器（持久化收割的受理端，实现 goharness tools.HarvestIntentRecorder）。
//
// 收割状态必须独立于任何线程/进程的生命周期存在（bega team 11-05 实证：
// 进程内存中的收割 goroutine 会无声消失，且无法从 daemon 重启中恢复）。
// 仿挂起提问 sidecar 先例（goharness session/pending_ask.go：会话目录内
// 独立 JSON、失败容错不阻断主流程），CollectResults 的收割意图以
// session-harvest.json 落在主会话目录内：
//   - 写入方：StartHarvest 经本文件 Record 落盘后立即返回回执；
//   - 扫描方：无状态 ticker（harvest_ticker.go）周期读取，从磁盘重新推导
//     落定状态并回灌主会话，daemon 重启后下一轮扫描自然捡起 pending 意图。
//
// 去重语义由落盘状态承载：去重键是任务句柄——文件已存在时 Record 把新增
// 目标按 task_id 合并登记（收割在飞时新派发的任务不得丢失），全部目标均已
// 登记才返回 ErrHarvestAlreadyActive，进程重启后依然有效。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DotNetAge/goharness/tools"
)

// harvestIntentFileName 是主会话目录内的收割意图 sidecar 文件名。
// 与挂起提问（session-ask-user.json）、待处理授权（session-permission.json）
// 同一持久化先例：独立 JSON、懒恢复、失败容错。
const harvestIntentFileName = "session-harvest.json"

// 收割意图状态机取值：pending（等待落定判定）→ reporting（回灌中）→
// 回灌成功删除文件；reporting 停滞（回灌轮崩溃）由 ticker 回退 pending。
const (
	harvestStatusPending   = "pending"
	harvestStatusReporting = "reporting"
)

// harvestIntentSub 是收割意图中的单个子任务条目：跟踪句柄（控制平面查询
// 入口）+ 子会话锚点（跨重启落盘事实查询入口）+ 回灌报告展示信息。
// RegisteredAt 是该目标自身的登记时间：收割总上限按它起算而非文件级
// CreatedAt——回灌进行中合并进来的新目标拥有完整的收割窗口（第三轮审计：
// 沿用文件级 CreatedAt 曾让临近上限合并的新任务被立即误标 timeout）。
type harvestIntentSub struct {
	TaskID       string    `json:"task_id"`
	SessionID    string    `json:"session_id,omitempty"`
	AgentName    string    `json:"agent_name,omitempty"`
	Title        string    `json:"title,omitempty"`
	RegisteredAt time.Time `json:"registered_at,omitempty"`
}

// harvestIntentFile 是 session-harvest.json 的落盘结构。
// CreatedAt/UpdatedAt 驱动总上限兜底与 reporting 停滞回退；Attempts 记录
// 回灌失败重投次数（连续失败达上限即放弃，子任务结果保留在子会话落盘
// 记录中不丢失）。ReportingBatch 是回灌轮已纳入上报的任务句柄快照：
// 回灌进行中新派发并合并进来的目标不属于该批次，回灌成功后只移除批次
// 条目、余下目标回 pending 继续收割——收割在飞时新任务不会被静默丢弃
// （实机实证：回灌轮中主会话继续派发并调用 CollectResults，若按主会话
// 一刀切去重，新任务的登记会被拒绝，落定后无人收割）。
type harvestIntentFile struct {
	MainSessionID  string             `json:"main_session_id"`
	SubSessions    []harvestIntentSub `json:"sub_sessions"`
	Status         string             `json:"status"`
	Attempts       int                `json:"attempts,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	ReportingBatch []string           `json:"reporting_batch,omitempty"`
}

// Record 登记收割意图并立即返回（实现 tools.HarvestIntentRecorder）。
// 去重键是任务句柄而非主会话：同一主会话已有意图时按 task_id 合并
// （收割在飞时新派发的任务必须能登记进来），全部目标均已登记时返回
// tools.ErrHarvestAlreadyActive（工具层转为「已在进行中」回执）。
// 写盘前做并发复核并重试（与收割轮询器全部写盘路径对称，防互相覆盖），
// 重试耗尽返回错误。落盘失败返回错误（工具层转为引导性回执，LLM 可重试），
// 绝不 panic——本方法在工具调用线程上执行，不得击穿会话执行循环。
func (d *Daemon) Record(mainSessionID string, targets []tools.HarvestTarget) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// 必须用命名返回值承接：无名返回值经 recover 退出取零值 nil，
			// panic 会被误报为「已登记成功」，收割意图无声丢失（第三轮审计）。
			d.logger.Error("harvest intent: record panic", fmt.Errorf("%v", r),
				"main_session", mainSessionID)
			err = fmt.Errorf("收割意图落盘异常，请重新调用 CollectResults")
		}
	}()
	if mainSessionID == "" {
		return fmt.Errorf("收割意图缺少主会话 ID，无法落盘")
	}
	path := d.harvestIntentPath(mainSessionID)
	if path == "" {
		return fmt.Errorf("无法定位主会话目录（会话 %s），收割意图未落盘", mainSessionID)
	}

	now := time.Now()
	// 读取-合并-复核-写盘闭环重试：写盘前复核文件未被并发修改（回灌端
	// onDone 的清理/改写恰在窗口内落盘时放弃本次写入、重读最新状态重新
	// 合并）。回灌端所有写盘路径都有对称复核，两端都校验才真正成立
	//「绝不覆盖丢登记」；重试耗尽返回错误（工具层转引导性回执，LLM 可重试）。
	for attempt := 0; attempt < 3; attempt++ {
		existing := d.loadHarvestIntent(path)
		if existing == nil {
			// 首次登记（或上一轮已被回灌端清理删除）：文件损坏按不存在
			// 处理（覆盖重写，损坏状态没有恢复价值）。按 task_id 去重——
			// 单次调用传重复句柄时只登记一次，与合并路径语义一致。
			intent := harvestIntentFile{
				MainSessionID: mainSessionID,
				Status:        harvestStatusPending,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			known := make(map[string]bool, len(targets))
			for _, tgt := range targets {
				if known[tgt.TaskID] {
					continue
				}
				known[tgt.TaskID] = true
				intent.SubSessions = append(intent.SubSessions, harvestIntentSub{
					TaskID:       tgt.TaskID,
					SessionID:    tgt.SessionID,
					AgentName:    tgt.AgentName,
					Title:        tgt.Title,
					RegisteredAt: now,
				})
			}
			if err := d.writeHarvestIntent(path, &intent); err != nil {
				return fmt.Errorf("收割意图落盘失败: %w", err)
			}
			return nil
		}

		// 已有意图：按 task_id 合并新目标（收割在飞时的派发不得丢失）。
		added := false
		known := make(map[string]bool, len(existing.SubSessions))
		for _, sub := range existing.SubSessions {
			known[sub.TaskID] = true
		}
		for _, tgt := range targets {
			if known[tgt.TaskID] {
				continue
			}
			known[tgt.TaskID] = true
			added = true
			existing.SubSessions = append(existing.SubSessions, harvestIntentSub{
				TaskID:       tgt.TaskID,
				SessionID:    tgt.SessionID,
				AgentName:    tgt.AgentName,
				Title:        tgt.Title,
				RegisteredAt: now, // 新目标从现在起算自己的收割窗口
			})
		}
		if !added {
			return tools.ErrHarvestAlreadyActive
		}
		loadedAt := existing.UpdatedAt
		existing.UpdatedAt = now
		if !d.harvestIntentUnchangedSince(path, loadedAt) {
			continue // 并发窗口：回灌端刚清理/改写过，重读最新状态重新合并
		}
		if err := d.writeHarvestIntent(path, existing); err != nil {
			return fmt.Errorf("收割意图合并落盘失败: %w", err)
		}
		return nil
	}
	return fmt.Errorf("收割状态正被并发更新，请稍后重新调用 CollectResults")
}

// harvestIntentPath 返回主会话收割意图文件的绝对路径；会话目录无法解析
// （会话不存在 / 存储不可用）时返回空串。
func (d *Daemon) harvestIntentPath(mainSessionID string) string {
	dir := d.sessionDirOf(mainSessionID)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, harvestIntentFileName)
}

// sessionDirOf 解析会话的落盘目录（挂起提问等 sidecar 的定位同源）。
// 会话 ID 为空、存储不可用或目录无法解析时返回空串。
func (d *Daemon) sessionDirOf(sessionID string) string {
	if sessionID == "" || d.app == nil || d.app.SessDB() == nil {
		return ""
	}
	dir, err := d.app.SessDB().ResolveSessionDir(sessionID)
	if err != nil {
		return ""
	}
	return dir
}

// loadHarvestIntent 读取收割意图；文件不存在或损坏返回 nil（无意图等价）。
func (d *Daemon) loadHarvestIntent(path string) *harvestIntentFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var intent harvestIntentFile
	if json.Unmarshal(data, &intent) != nil {
		return nil
	}
	return &intent
}

// writeHarvestIntent 原子写收割意图（临时文件 + rename，避免读方读到半截
// JSON）；目录不存在时兜底创建（会话目录理论上先于意图存在）。
func (d *Daemon) writeHarvestIntent(path string, intent *harvestIntentFile) error {
	data, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// deleteHarvestIntent 删除收割意图文件（回灌成功即 done，整文件删除——
// 无需保留 done 终态：意图消失即「无待办」）。不存在时视为成功。
func (d *Daemon) deleteHarvestIntent(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// 编译期契约校验：Daemon 必须实现 goharness 收割意图落盘器。
var _ tools.HarvestIntentRecorder = (*Daemon)(nil)
