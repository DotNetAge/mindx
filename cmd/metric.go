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
