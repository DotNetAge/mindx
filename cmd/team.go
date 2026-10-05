package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/internal/core/teamstore"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── team parent ────────────────────────────────────────────────

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage agent teams",
	Long: `List, inspect, add, remove, hire or fire agent teams.

A team is a first-class entity (~/.mindx/teams/<name>/TEAM.md) that organizes
agents (leader + members, pure references) and carries its own profile.
Team sessions resolve to the leader agent — talking to a team IS talking to
its leader, and the system prompt stays agent-generated.

By default reads teams from local config files (hired only, use --all for all).
Use --json to query the daemon and output JSON (for LLM consumption).

Examples:
  mindx team list
  mindx team list --all
  mindx team list --json
  mindx team get alpha
  mindx team add alpha --leader architect --member architect --member flutter-dev
  mindx team hire alpha
  mindx team fire alpha
  mindx team rm alpha`,
}

// ── team list ──────────────────────────────────────────────────

var teamListCmd = &cobra.Command{
	Use:   "list",
	Short: "List hired teams (use --all to include all)",
	RunE: func(cmd *cobra.Command, args []string) error {
		useJSON, _ := cmd.Flags().GetBool("json")
		showAll, _ := cmd.Flags().GetBool("all")

		if useJSON {
			cl, err := rpc.Dial(daemonAddr)
			if err != nil {
				return fmt.Errorf("cannot connect to daemon: %w", err)
			}
			defer func() { _ = cl.Close() }()

			result, err := cl.TeamList()
			if err != nil {
				return err
			}

			var list []map[string]any
			if err := json.Unmarshal(result, &list); err != nil {
				fmt.Println(string(result))
				return nil
			}
			filtered := make([]map[string]any, 0, len(list))
			for _, m := range list {
				if !showAll && m["hired"] != true {
					continue
				}
				filtered = append(filtered, m)
			}
			formatted, _ := json.MarshalIndent(filtered, "", "  ")
			fmt.Println(string(formatted))
			return nil
		}

		store, _, err := teamstore.Load(teamsDir())
		if err != nil {
			return fmt.Errorf("cannot load teams: %w", err)
		}

		var teams []*teamstore.Team
		if showAll {
			teams = store.List()
		} else {
			teams = store.Hired()
		}
		if len(teams) == 0 {
			if showAll {
				fmt.Printf("No teams found in %s.\n", teamsDir())
			} else {
				fmt.Printf("暂无已雇佣的团队（目录：%s）。\n", teamsDir())
				fmt.Println("提示：使用 `mindx team hire <name>` 雇佣，或 `mindx team list --all` 查看全部。")
			}
			return nil
		}

		table := render.NewTable([]string{"Name", "Leader", "Members", "Description"}, 100)
		for _, t := range teams {
			desc := t.Meta.Description
			if desc == "" {
				desc = "—"
			}
			members := "—"
			if len(t.Meta.Members) > 0 {
				members = fmt.Sprintf("%s (%d)", t.Meta.Leader, len(t.Meta.Members))
			}
			table.AddRow([]string{t.Meta.Name, t.Meta.Leader, members, desc})
		}
		fmt.Println(table.Render())
		if showAll {
			fmt.Printf("\n%d team(s) configured.\n", len(teams))
		} else {
			fmt.Printf("\n%d hired team(s). Use --all to see all.\n", len(teams))
		}
		return nil
	},
}

// ── team get ───────────────────────────────────────────────────

var teamGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Show team details via daemon (JSON output)",
	Long: `Query the daemon for a single team's full configuration.
Outputs rich JSON suitable for LLM consumption.

Example:
  mindx team get alpha`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.TeamGet(args[0])
		if err != nil {
			return err
		}
		var pretty map[string]any
		if err := json.Unmarshal(result, &pretty); err != nil {
			fmt.Println(string(result))
			return nil
		}
		formatted, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(formatted))
		return nil
	},
}

// ── team add ───────────────────────────────────────────────────

var teamAddCmd = &cobra.Command{
	Use:   "add <name> --leader <agent> [--member <agent>]...",
	Short: "Create a team (leader required, members are agent name references)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("team name is required")
		}
		name := args[0]
		leader, _ := cmd.Flags().GetString("leader")
		description, _ := cmd.Flags().GetString("description")
		nickName, _ := cmd.Flags().GetString("nick_name")
		icon, _ := cmd.Flags().GetString("icon")
		members, _ := cmd.Flags().GetStringArray("member")
		profile, _ := cmd.Flags().GetString("profile")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		params := rpc.TeamUpdateParams{Name: name, Hired: boolPtr(true)}
		if leader != "" {
			params.Leader = &leader
		}
		if description != "" {
			params.Description = &description
		}
		if nickName != "" {
			params.NickName = &nickName
		}
		if icon != "" {
			params.Icon = &icon
		}
		if profile != "" {
			params.Profile = &profile
		}
		if len(members) > 0 {
			params.Members = members
		}
		if _, err := cl.TeamUpdate(params); err != nil {
			return err
		}
		fmt.Printf("团队 %q 已创建（负责人：%s，成员 %d 名）。\n", name, leader, len(members))
		return nil
	},
}

// ── team update ────────────────────────────────────────────────

var teamUpdateCmd = &cobra.Command{
	Use:   "update <name> [--leader <agent>] [--member <agent>]... [--description <text>] [--profile <text>]",
	Short: "Update a team (pointer semantics: only provided fields change)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("team name is required")
		}
		name := args[0]
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		params := rpc.TeamUpdateParams{Name: name}
		if f := cmd.Flags(); f.Changed("leader") {
			v, _ := f.GetString("leader")
			params.Leader = &v
		}
		if f := cmd.Flags(); f.Changed("description") {
			v, _ := f.GetString("description")
			params.Description = &v
		}
		if f := cmd.Flags(); f.Changed("profile") {
			v, _ := f.GetString("profile")
			params.Profile = &v
		}
		if f := cmd.Flags(); f.Changed("member") {
			v, _ := f.GetStringArray("member")
			params.Members = v
		}
		if _, err := cl.TeamUpdate(params); err != nil {
			return err
		}
		fmt.Printf("团队 %q 已更新。\n", name)
		return nil
	},
}

// ── team hire / fire ───────────────────────────────────────────

var teamHireCmd = &cobra.Command{
	Use:   "hire <name>",
	Short: "Hire a team (makes it selectable for team sessions)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTeamHireToggle(args, true)
	},
}

var teamFireCmd = &cobra.Command{
	Use:   "fire <name>",
	Short: "Fire a team (hidden from selection, existing sessions keep working)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTeamHireToggle(args, false)
	},
}

func runTeamHireToggle(args []string, hired bool) error {
	if len(args) != 1 {
		return fmt.Errorf("team name is required")
	}
	cl, err := rpc.Dial(daemonAddr)
	if err != nil {
		return fmt.Errorf("cannot connect to daemon: %w", err)
	}
	defer func() { _ = cl.Close() }()

	if hired {
		if _, err := cl.TeamHire(args[0]); err != nil {
			return err
		}
		fmt.Printf("团队 %q 已雇佣。\n", args[0])
		return nil
	}
	if _, err := cl.TeamFire(args[0]); err != nil {
		return err
	}
	fmt.Printf("团队 %q 已解雇。\n", args[0])
	return nil
}

// ── team rm ────────────────────────────────────────────────────

var teamRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a team (referenced agents are untouched)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("team name is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		if _, err := cl.TeamDelete(args[0]); err != nil {
			return err
		}
		fmt.Printf("团队 %q 已删除（成员 Agent 不受影响；既有团队会话将降级为普通会话）。\n", args[0])
		return nil
	},
}

// ── team export / import ───────────────────────────────────────

var teamExportCmd = &cobra.Command{
	Use:   "export <name> [-o <out.mindpkg>]",
	Short: "Export a team as a distributable package (team + member agents)",
	Long: `Export a team as a .mindpkg for sharing or market publishing.

The package contains the team definition (TEAM.md) plus every member agent's
actual effective content (IDENTITY/SOUL/agent-level skills), so installing it
elsewhere recreates the whole team. Missing member agents are reported as
warnings and skipped.

Example:
  mindx team export alpha-team
  mindx team export alpha-team -o /tmp/alpha-team.mindpkg`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, _ := cmd.Flags().GetString("out")
		if out == "" {
			out = args[0] + ".mindpkg"
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.TeamExport(args[0], out)
		if err != nil {
			return err
		}
		var pretty map[string]any
		if err := json.Unmarshal(result, &pretty); err == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(formatted))
		} else {
			fmt.Println(string(result))
		}
		fmt.Printf("团队整包已导出：%s\n", out)
		return nil
	},
}

var teamImportCmd = &cobra.Command{
	Use:   "import <path.mindpkg>",
	Short: "Install a team package locally (team + member agents land together)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("分发包路径为必填")
		}
		overwrite, _ := cmd.Flags().GetBool("overwrite")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.TeamImport(args[0], overwrite)
		if err != nil {
			return err
		}
		var pretty map[string]any
		if err := json.Unmarshal(result, &pretty); err == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(formatted))
		} else {
			fmt.Println(string(result))
		}
		fmt.Printf("提示：团队与成员默认未雇佣，执行 `mindx agent hire <成员>` 与 `mindx team hire` 后即可发起团队会话。\n")
		return nil
	},
}

func boolPtr(b bool) *bool { return &b }

func init() {
	teamAddCmd.Flags().String("leader", "", "团队负责人 Agent 名（必需）")
	teamAddCmd.Flags().String("nick_name", "", "团队昵称（展示主名）")
	teamAddCmd.Flags().String("description", "", "团队描述")
	teamAddCmd.Flags().String("icon", "", "团队图标")
	teamAddCmd.Flags().String("profile", "", "团队 Profile 正文（人设定义）")
	teamAddCmd.Flags().StringArray("member", nil, "团队成员 Agent 名（可重复传入）")
	_ = teamAddCmd.MarkFlagRequired("leader")

	teamUpdateCmd.Flags().String("leader", "", "更换负责人")
	teamUpdateCmd.Flags().String("description", "", "团队描述")
	teamUpdateCmd.Flags().String("profile", "", "团队 Profile 正文")
	teamUpdateCmd.Flags().StringArray("member", nil, "团队成员 Agent 名（全量覆盖）")

	teamListCmd.Flags().Bool("all", false, "Include all teams (not only hired)")
	teamListCmd.Flags().Bool("json", false, "Query daemon and output JSON")

	teamExportCmd.Flags().String("out", "", "输出 .mindpkg 路径（默认 <name>.mindpkg）")
	teamImportCmd.Flags().Bool("overwrite", false, "Overwrite existing team/agents with the same name")

	teamCmd.AddCommand(teamListCmd)
	teamCmd.AddCommand(teamGetCmd)
	teamCmd.AddCommand(teamAddCmd)
	teamCmd.AddCommand(teamUpdateCmd)
	teamCmd.AddCommand(teamHireCmd)
	teamCmd.AddCommand(teamFireCmd)
	teamCmd.AddCommand(teamRmCmd)
	teamCmd.AddCommand(teamExportCmd)
	teamCmd.AddCommand(teamImportCmd)
	rootCmd.AddCommand(teamCmd)
}
