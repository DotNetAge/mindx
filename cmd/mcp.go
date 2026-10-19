package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// mcpCmd 让 MCP 服务可以像普通命令行工具一样被查、装、调：
// 技能（SKILL.md）里写清安装与调用方法后，Agent 无需在装配期注入任何
// 工具 schema，用到时按需 add → enable → call，token 开销为零。
var mcpCmd = &cobra.Command{
	Use:   "mcp list|add|tools|call|enable|disable|test",
	Short: "Manage and call MCP services as CLI",
	Long: `Manage MCP services from the command line.

Skills can treat an MCP service like a CLI tool: check whether it is
installed (list), install it on demand (add), enable it (enable), browse
its tools (tools), and invoke a tool (call). Nothing is injected into any
agent context — the service is only reached when the skill runs.

Examples:
  mindx mcp list --json
  mindx mcp add maps --command npx --args "-y,@amap/amap-maps-mcp-server" --env "AMAP_MAPS_API_KEY=xxx" --enable
  mindx mcp add travel --url https://example.com/mcp --enable
  mindx mcp tools maps
  mindx mcp call maps maps_weather --arg city=杭州
  mindx mcp test maps
  mindx mcp disable maps`,
}

// ── list ────────────────────────────────────────────────────────────────────

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List known MCP services",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		useJSON, _ := cmd.Flags().GetBool("json")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		raw, err := cl.CallWithTimeout("mcp.server.list", map[string]any{})
		if err != nil {
			return err
		}

		if useJSON {
			return printJSON(raw)
		}

		var entries []struct {
			Name    string `json:"name"`
			Title   string `json:"title"`
			Type    string `json:"type"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &entries); err != nil {
			return fmt.Errorf("解析服务清单失败: %w", err)
		}
		if len(entries) == 0 {
			fmt.Println("还没有任何服务，用 mindx mcp add 添加")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tENABLED\tTITLE")
		for _, e := range entries {
			enabled := "no"
			if e.Enabled {
				enabled = "yes"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Name, e.Type, enabled, e.Title)
		}
		return w.Flush()
	},
}

// ── add ─────────────────────────────────────────────────────────────────────

var mcpAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Install an MCP service (disabled by default)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		command, _ := cmd.Flags().GetString("command")
		url, _ := cmd.Flags().GetString("url")
		title, _ := cmd.Flags().GetString("title")
		idleTTL, _ := cmd.Flags().GetInt("idle-ttl")
		enable, _ := cmd.Flags().GetBool("enable")

		if command == "" && url == "" {
			return fmt.Errorf("本地服务用 --command，远程服务用 --url，至少给一个")
		}
		if command != "" && url != "" {
			return fmt.Errorf("--command 与 --url 只能二选一")
		}

		params := map[string]any{
			"name": name,
		}
		if command != "" {
			params["command"] = command
			params["type"] = "stdio"
		}
		if url != "" {
			params["url"] = url
			params["type"] = "http"
		}
		if title != "" {
			params["title"] = title
		}
		if idleTTL > 0 {
			params["idle_ttl_secs"] = idleTTL
		}
		if v, _ := cmd.Flags().GetString("args"); v != "" {
			params["args"] = splitCSV(v)
		}
		if headers, err := kvPairs(cmd, "header"); err != nil {
			return err
		} else if len(headers) > 0 {
			params["headers"] = headers
		}
		if env, err := kvPairs(cmd, "env"); err != nil {
			return err
		} else if len(env) > 0 {
			params["env"] = env
		}
		if creds, err := kvPairs(cmd, "credential"); err != nil {
			return err
		} else if len(creds) > 0 {
			params["credential"] = creds
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		if _, err := cl.CallWithTimeout("mcp.server.add", params); err != nil {
			return err
		}
		if enable {
			if _, err := cl.CallWithTimeout("mcp.server.set_enabled", map[string]any{"name": name, "enabled": true}); err != nil {
				return err
			}
			fmt.Printf("已添加并启用 %s\n", name)
		} else {
			fmt.Printf("已添加 %s（当前未启用）。运行 mindx mcp enable %s 启用后才能调用\n", name, name)
		}
		return nil
	},
}

// ── tools ───────────────────────────────────────────────────────────────────

var mcpToolsCmd = &cobra.Command{
	Use:   "tools <server>",
	Short: "List the tools an MCP service provides",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		useJSON, _ := cmd.Flags().GetBool("json")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		raw, err := cl.CallWithTimeout("mcp.server.discover", map[string]any{"name": args[0]})
		if err != nil {
			return err
		}

		if useJSON {
			return printJSON(raw)
		}

		var tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(raw, &tools); err != nil {
			return fmt.Errorf("解析工具清单失败: %w", err)
		}
		if len(tools) == 0 {
			fmt.Printf("%s 没有提供任何工具\n", args[0])
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION")
		for _, t := range tools {
			fmt.Fprintf(w, "%s\t%s\n", t.Name, firstLine(t.Description))
		}
		return w.Flush()
	},
}

// ── call ────────────────────────────────────────────────────────────────────

var mcpCallCmd = &cobra.Command{
	Use:   "call <server> <tool>",
	Short: "Invoke a tool on an MCP service",
	Long: `Invoke a tool on an enabled MCP service and print the result.

Simple key=value arguments:  mindx mcp call srv tool --arg city=杭州 --arg days=3
Typed/complex arguments:     mindx mcp call srv tool --args-json '{"days": 3, "ids": [1,2]}'`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		useJSON, _ := cmd.Flags().GetBool("json")
		timeout, _ := cmd.Flags().GetInt("timeout")
		argsJSON, _ := cmd.Flags().GetString("args-json")

		toolArgs := map[string]any{}
		kvs, err := kvPairs(cmd, "arg")
		if err != nil {
			return err
		}
		for k, v := range kvs {
			toolArgs[k] = v
		}
		if argsJSON != "" {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(argsJSON), &parsed); err != nil {
				return fmt.Errorf("--args-json 必须是合法的 JSON 对象: %w", err)
			}
			for k, v := range parsed {
				toolArgs[k] = v
			}
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		raw, err := cl.Call(ctx, "mcp.tool.call", map[string]any{
			"server": args[0],
			"tool":   args[1],
			"args":   toolArgs,
		})
		if err != nil {
			return err
		}
		if useJSON {
			return printJSON(raw)
		}

		var result struct {
			Result string `json:"result"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return fmt.Errorf("解析调用结果失败: %w", err)
		}
		fmt.Println(result.Result)
		return nil
	},
}

// ── enable / disable / test ─────────────────────────────────────────────────

var mcpEnableCmd = &cobra.Command{
	Use:   "enable <server>",
	Short: "Enable an MCP service so its tools can be called",
	Args:  cobra.ExactArgs(1),
	RunE:  runMCPSetEnabled(true),
}

var mcpDisableCmd = &cobra.Command{
	Use:   "disable <server>",
	Short: "Disable an MCP service",
	Args:  cobra.ExactArgs(1),
	RunE:  runMCPSetEnabled(false),
}

func runMCPSetEnabled(enabled bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		if _, err := cl.CallWithTimeout("mcp.server.set_enabled", map[string]any{"name": args[0], "enabled": enabled}); err != nil {
			return err
		}
		state := "已停用"
		if enabled {
			state = "已启用"
		}
		fmt.Printf("%s %s\n", args[0], state)
		return nil
	}
}

var mcpTestCmd = &cobra.Command{
	Use:   "test <server>",
	Short: "Test whether an MCP service can be reached",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		raw, err := cl.CallWithTimeout("mcp.server.test", map[string]any{"name": args[0]})
		if err != nil {
			return err
		}
		var result struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return fmt.Errorf("解析测试结果失败: %w", err)
		}
		if !result.OK {
			return fmt.Errorf("连接失败: %s", result.Error)
		}
		fmt.Printf("%s 连接成功\n", args[0])
		return nil
	},
}

// ── helpers ─────────────────────────────────────────────────────────────────

func init() {
	mcpCmd.AddCommand(mcpListCmd, mcpAddCmd, mcpToolsCmd, mcpCallCmd, mcpEnableCmd, mcpDisableCmd, mcpTestCmd)

	mcpListCmd.Flags().Bool("json", false, "输出 JSON（供程序消费）")
	mcpToolsCmd.Flags().Bool("json", false, "输出 JSON（供程序消费）")
	mcpCallCmd.Flags().Bool("json", false, "输出 JSON（供程序消费）")

	mcpAddCmd.Flags().String("command", "", "本地服务的启动命令（如 npx、uvx）")
	mcpAddCmd.Flags().String("url", "", "远程服务端点")
	mcpAddCmd.Flags().String("args", "", "启动命令参数，逗号分隔")
	mcpAddCmd.Flags().StringSlice("header", nil, "远程请求头，K=V，可重复")
	mcpAddCmd.Flags().StringSlice("env", nil, "服务进程环境变量，K=V，可重复")
	mcpAddCmd.Flags().StringSlice("credential", nil, "凭据键值（存入本机凭据库），K=V，可重复")
	mcpAddCmd.Flags().String("title", "", "显示名称")
	mcpAddCmd.Flags().Int("idle-ttl", 300, "空闲连接保留秒数")
	mcpAddCmd.Flags().Bool("enable", false, "添加后立即启用")

	mcpCallCmd.Flags().StringSlice("arg", nil, "工具参数，K=V，可重复（值均为文本）")
	mcpCallCmd.Flags().String("args-json", "", "工具参数 JSON 对象（与 --arg 可混用，类型精确）")
	mcpCallCmd.Flags().Int("timeout", 60, "调用超时秒数")
}

// kvPairs 解析 "K=V" 形式的 StringSlice flags 为 map。
func kvPairs(cmd *cobra.Command, flag string) (map[string]string, error) {
	values, _ := cmd.Flags().GetStringSlice(flag)
	if len(values) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(values))
	for _, v := range values {
		k, val, ok := strings.Cut(v, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("--%s 需要 K=V 形式，收到 %q", flag, v)
		}
		out[k] = val
	}
	return out, nil
}

// splitCSV 把逗号分隔的参数串拆成列表（保留空段检查交给服务端校验）。
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// firstLine 取描述首行（表格里放完整描述会挤成一团）。
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimRight(s[:i], "\r")
	}
	return s
}

// printJSON 缩进输出原始 JSON。
func printJSON(raw json.RawMessage) error {
	var pretty any
	if err := json.Unmarshal(raw, &pretty); err == nil {
		formatted, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(formatted))
		return nil
	}
	fmt.Println(string(raw))
	return nil
}
