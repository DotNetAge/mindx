package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── metric parent ─────────────────────────────────────────────

// metricCmd 是决策量表命令（白皮书《决策量表：Agent 生态的绩效系统》第 5 章接口规范）。
// 输出为 JSON（LLM 消费格式），人读展示为摘要表格；确定性计算全部在
// daemon 聚合层完成，CLI 不做任何心算。
var metricCmd = &cobra.Command{
	Use:   "metric",
	Short: "Decision scorecard (requires daemon)",
	Long: `Query the decision scorecard from the daemon.

All operations require the daemon to be running (mindx start).

Examples:
  mindx metric report
  mindx metric report --window-days 30 --agent assistant --json`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(metricCmd)
}

// ── metric report ─────────────────────────────────────────────

var metricReportCmd = &cobra.Command{
	Use:     "report",
	Short:   "Aggregate the decision scorecard for a time window",
	Example: `  mindx metric report --window-days 7 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		windowDays, _ := cmd.Flags().GetInt("window-days")
		agent, _ := cmd.Flags().GetString("agent")
		skill, _ := cmd.Flags().GetString("skill")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MetricReport(rpc.MetricReportParams{
			WindowDays: windowDays,
			Agent:      agent,
			Skill:      skill,
		})
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		// 人读摘要：四因子指标逐行呈现，null（样本不足）显式标注。
		var data struct {
			Window struct {
				Since string `json:"since"`
				Until string `json:"until"`
			} `json:"window"`
			Samples struct {
				Rounds int `json:"rounds"`
				Judged int `json:"judged"`
			} `json:"samples"`
			Speed struct {
				SRounds   *statView `json:"s_rounds"`
				SDuration *statView `json:"s_duration"`
			} `json:"speed"`
			Effort struct {
				ETokens *statView `json:"e_tokens"`
				ECost   *statView `json:"e_cost"`
			} `json:"effort"`
			Quality struct {
				QRouteCorrection *statView `json:"q_route_correction"`
			} `json:"quality"`
			ByAgent []struct {
				Agent     string  `json:"agent"`
				Rounds    int     `json:"rounds"`
				TokensSum int     `json:"tokens_sum"`
				Coverage  float64 `json:"coverage"`
			} `json:"by_agent"`
			Notes []string `json:"notes"`
		}
		if err := json.Unmarshal(result, &data); err != nil {
			// 解析失败时退回原始 JSON，保证调用方总能拿到数据。
			fmt.Println(string(result))
			return nil
		}

		fmt.Printf("决策量表  窗口 %s ~ %s\n", data.Window.Since, data.Window.Until)
		fmt.Printf("样本：round 事件 %d，含判定 %d\n\n", data.Samples.Rounds, data.Samples.Judged)

		fmt.Println("Speed（决策时延）")
		printStat("  s_rounds    每任务循环数", data.Speed.SRounds)
		printStat("  s_duration  任务时长(ms)", data.Speed.SDuration)
		fmt.Println("Effort（资源消耗）")
		printStat("  e_tokens    每任务有效 token", data.Effort.ETokens)
		printStat("  e_cost      每任务成本", data.Effort.ECost)
		fmt.Println("Quality（决策质量）")
		printStat("  q_route_correction  相邻技能切换", data.Quality.QRouteCorrection)
		fmt.Println("Yield / Perceived：v2 判定层交付前为 null 占位")

		if len(data.ByAgent) > 0 {
			fmt.Println("\n按 Agent 下钻")
			for _, row := range data.ByAgent {
				fmt.Printf("  %-16s rounds=%-4d tokens=%-8d 判定覆盖率=%.0f%%\n",
					row.Agent, row.Rounds, row.TokensSum, row.Coverage*100)
			}
		}
		for _, note := range data.Notes {
			fmt.Printf("\n注：%s\n", note)
		}
		return nil
	},
}

type statView struct {
	Samples int     `json:"samples"`
	Median  float64 `json:"median"`
	P90     float64 `json:"p90"`
	Null    bool    `json:"null"`
}

func printStat(label string, s *statView) {
	if s == nil || s.Null {
		fmt.Printf("%s  null（样本 %d，不足阈值）\n", label, s.Samples)
		return
	}
	fmt.Printf("%s  median=%.1f p90=%.1f (n=%d)\n", label, s.Median, s.P90, s.Samples)
}

func init() {
	metricReportCmd.Flags().Int("window-days", 7, "统计窗口（天）")
	metricReportCmd.Flags().String("agent", "", "按 Agent 过滤")
	metricReportCmd.Flags().String("skill", "", "按技能过滤")
	metricReportCmd.Flags().Bool("json", false, "输出 JSON（LLM 消费格式）")
	metricCmd.AddCommand(metricReportCmd)
}

// ── metric baseline save / list ───────────────────────────────

var metricBaselineSaveCmd = &cobra.Command{
	Use:     "baseline save <name>",
	Short:   "Save current window scorecard as a named baseline",
	Example: `  mindx metric baseline save sprint-42 --window-days 7`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("baseline name is required")
		}
		windowDays, _ := cmd.Flags().GetInt("window-days")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MetricBaselineSave(rpc.MetricBaselineSaveParams{Name: args[0], WindowDays: windowDays})
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

var metricBaselineListCmd = &cobra.Command{
	Use:     "baseline list",
	Short:   "List saved baselines",
	Example: `  mindx metric baseline list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MetricBaselineList()
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── metric compare ────────────────────────────────────────────

var metricCompareCmd = &cobra.Command{
	Use:           "compare --baseline <name>",
	Short:         "Compare current window against a baseline (Pareto verdict)",
	SilenceErrors: true, // 非改善退出时静默返回非零码（ verdict 已在 stdout 呈现）
	Long: `Compare the current scorecard window against a saved baseline.

Verdict semantics (Pareto): any factor significantly regressed → "regressed";
otherwise any improved → "improved"; else "flat". A confounding list
(model/daemon changes during the window) is always attached — the scorecard
is a signal for review, not proof of causality.

Exit code 0 = improved, 1 = flat or regressed (for retrospective automation).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		baseline, _ := cmd.Flags().GetString("baseline")
		windowDays, _ := cmd.Flags().GetInt("window-days")
		jsonOut, _ := cmd.Flags().GetBool("json")
		if baseline == "" {
			return fmt.Errorf("--baseline is required")
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MetricCompare(rpc.MetricCompareParams{Baseline: baseline, WindowDays: windowDays})
		if err != nil {
			return err
		}

		var data struct {
			Verdict string `json:"verdict"`
			Factors []struct {
				Factor    string  `json:"factor"`
				Direction string  `json:"direction"`
				Baseline  float64 `json:"baseline"`
				Current   float64 `json:"current"`
				Change    string  `json:"change"`
			} `json:"factors"`
			Confoundings []struct {
				Kind   string `json:"kind"`
				Detail string `json:"detail"`
			} `json:"confoundings"`
		}
		if err := json.Unmarshal(result, &data); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if jsonOut {
			fmt.Println(string(result))
		} else {
			fmt.Printf("对比基线 %q：%s\n\n", baseline, data.Verdict)
			for _, f := range data.Factors {
				status := map[string]string{"improved": "↑", "regressed": "↓", "flat": "→", "null": "n/a"}[f.Change]
				fmt.Printf("  %-22s %8.3f → %8.3f  %s\n", f.Factor, f.Baseline, f.Current, status)
			}
			if len(data.Confoundings) > 0 {
				fmt.Println("\n窗口内混杂事件（归因解读前先看这里）：")
				for _, c := range data.Confoundings {
					fmt.Printf("  [%s] %s\n", c.Kind, c.Detail)
				}
			}
		}

		// 退出码语义：0 = 改善、1 = 持平/劣化（供复盘自动化）。
		if data.Verdict != "improved" {
			return errSilentExit
		}
		return nil
	},
}

// errSilentExit 让命令以非零码退出（compare 配 SilenceErrors 静默，verdict 已在 stdout）。
var errSilentExit = fmt.Errorf("verdict not improved")

// ── metric record ─────────────────────────────────────────────

// metricRecordCmd 让 Agent 在任务收尾时把成败判定写进量表。
//
// 判定归属分两种情形，由服务端按会话是否正在执行自动判别：
//   - 轮次内上报（会话执行中）→ 暂存，本轮 round 事件落盘后自动挂载。
//     这是唯一能做到「判定与它评的那一轮严格对齐」的方式：round 事件在
//     整轮结束的 defer 里才写，此刻就地定位必然落到上一轮上。
//   - 轮次外上报（补记历史 / 复盘对旧会话下判定）→ 就地挂到该会话最新一轮。
//
// outcome=rework 时服务端会把同 Agent、同任务意图的历史会话一并标记为返工——
// 这是「错误会改写历史判定」的机制实现，也是技能蒸馏的原料入口。
//
// 四段式 note 是本命令的用法要点：note 应当能填满技能履历的四个字段
// （问题/否决/修正/验证）。只写一句「失败了」的 note 对复盘无价值。
var metricRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Report the outcome judgment for a session (Agent self-report)",
	Long: `Record a success / fail / rework judgment onto a session's round event.

Judgment placement depends on whether the session is executing:

  * inside a round (session running) — the round event is written only after the
    whole round finishes, so the judgment is stashed and mounted automatically
    once that event lands. This keeps the judgment on the round it actually judges.
  * outside a round (backfilling history) — mounted immediately on the session's
    latest round event.

The judgment is appended to the event's version chain — history is never rewritten,
only extended. Recording "rework" additionally marks prior sessions with the same
task intent as reworked (same agent + identical task name).

Source priority governs which judgment wins (whitepaper 7.4): user feedback always
outranks agent reporting, regardless of timestamps. Within the same source tier the
latest timestamp wins, so a later "rework" overrides an earlier "success".

Use --note to carry the four-part evidence a retrospective needs:
problem / rejected alternatives / fix / verification.

Exit code 0 on success; non-zero when the daemon rejects the payload.`,
	Example: `  mindx metric record --session-id 01ABCD... --outcome fail --note "问题：… 否决：… 修正：… 验证：…"
  mindx metric record --session-id 01ABCD... --outcome rework`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session-id")
		outcome, _ := cmd.Flags().GetString("outcome")
		note, _ := cmd.Flags().GetString("note")
		jsonOut, _ := cmd.Flags().GetBool("json")

		if sessionID == "" {
			return fmt.Errorf("--session-id is required")
		}
		switch outcome {
		case "success", "fail", "rework":
		default:
			return fmt.Errorf("--outcome 必须是 success / fail / rework 之一")
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MetricRecord(rpc.MetricRecordParams{
			SessionID: sessionID,
			Outcome:   outcome,
			Note:      note,
		})
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var data struct {
			OK        bool `json:"ok"`
			Revisited int  `json:"revisited"`
			Deferred  bool `json:"deferred"`
		}
		if err := json.Unmarshal(result, &data); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if data.Deferred {
			// 轮次内上报：本轮 round 事件要等整轮结束才落盘，判定先暂存，
			// 由服务端在该轮事件写入后自动挂载。必须显式告知——否则调用方
			// 会以为判定已生效，而此刻它其实还没写进量表。
			fmt.Printf("判定已暂存：会话 %s → %s（本轮结束后自动挂载到本轮事件）\n", sessionID, outcome)
			return nil
		}
		fmt.Printf("判定已写入：会话 %s → %s\n", sessionID, outcome)
		if data.Revisited > 0 {
			// 回溯数是被改写的历史判定条数，必须显式告知——用户要知道
			// 过去的「成功」记录已被这次返工推翻。
			fmt.Printf("返工回溯：%d 个同任务意图的历史会话已标记为 rework\n", data.Revisited)
		}
		return nil
	},
}

func init() {
	metricBaselineSaveCmd.Flags().Int("window-days", 7, "统计窗口（天）")
	metricCompareCmd.Flags().String("baseline", "", "基线名称")
	metricCompareCmd.Flags().Int("window-days", 7, "统计窗口（天）")
	metricCompareCmd.Flags().Bool("json", false, "输出 JSON")
	metricRecordCmd.Flags().String("session-id", "", "会话 ID（必填）")
	metricRecordCmd.Flags().String("outcome", "", "判定结果：success | fail | rework（必填）")
	metricRecordCmd.Flags().String("note", "", "判定依据（四段式：问题/否决/修正/验证）")
	metricRecordCmd.Flags().Bool("json", false, "输出 JSON（LLM 消费格式）")
	metricCmd.AddCommand(metricBaselineSaveCmd, metricBaselineListCmd, metricCompareCmd, metricRecordCmd)
}
