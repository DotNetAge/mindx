package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── org parent：组织人事与架构（大秘「构建组织」的生产通路） ────────

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Org people & architecture: members, invites, org chart (requires mindx start)",
	Long: `Organization people & architecture — the channel for building and running an org.

The architecture chart is an XML tree: the boss describes it in conversation,
the agent drafts it, shows a rendered diagram for confirmation, then publishes
it here. New members are invited by their addressable account (username /
phone / GitHub name) and join by onboarding against a chart slot.

Commands:

  mindx org create <组织名> <别名>                          建组织（创建者即所有者）
  mindx org members [--org <组织>]                          在册成员清单（含移除用的成员编号）
  mindx org invite <账号> [职位] [--org <组织>]             按可寻址账号发定向邀请
  mindx org remove <成员编号> [--org <组织>]                移除单个在册成员（坐席空出）
  mindx org architecture show [--org <组织>]                查看当前架构树（XML 原文）
  mindx org architecture publish [--org <组织>] [--file 路径]  发布/覆盖架构树（无 --file 读 stdin）

Examples:
  mindx org create 积木科技 jimu
  mindx org invite zhangsan "销售经理"
  mindx org architecture publish --file org.xml
  cat org.xml | mindx org architecture publish
  mindx org members`,
}

func init() {
	orgMembersCmd.Flags().String("org", "", "目标组织（缺省取在册的第一个正常组织）")
	orgInviteCmd.Flags().String("org", "", "目标组织（缺省取在册的第一个正常组织）")
	orgRemoveCmd.Flags().String("org", "", "目标组织（缺省取在册的第一个正常组织）")
	orgArchShowCmd.Flags().String("org", "", "目标组织（缺省取在册的第一个正常组织）")
	orgArchPublishCmd.Flags().String("org", "", "目标组织（缺省取在册的第一个正常组织）")
	orgArchPublishCmd.Flags().String("file", "", "从文件读取架构树 XML（指定后忽略 stdin）")
	orgCmd.AddCommand(orgCreateCmd, orgMembersCmd, orgInviteCmd, orgRemoveCmd, orgArchCmd)
	rootCmd.AddCommand(orgCmd)
}

// dialOrg 通连 daemon 的公共小步（每个子命令都要走）。
func dialOrg() (*rpc.Client, func(), error) {
	cl, err := rpc.Dial(daemonAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot connect to daemon: %w", err)
	}
	return cl, func() { _ = cl.Close() }, nil
}

// ── org create：建组织 ────────────────────────────────────────

var orgCreateCmd = &cobra.Command{
	Use:   "create <组织名> <别名>",
	Short: "Create a new org; the creating account becomes its owner (requires mindx start)",
	Long: `Create a new organization. The alias (slug) is the ASCII identifier
used in URLs and --org flags; the display name is what people see.
The creating account becomes the owner.

Next steps after creating: publish the architecture chart
(mindx org architecture publish), then invite members
(mindx org invite).`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.TrimSpace(args[0])
		slug := strings.TrimSpace(args[1])
		if name == "" || slug == "" {
			return fmt.Errorf("组织名称与别名均不能为空")
		}
		cl, done, err := dialOrg()
		if err != nil {
			return err
		}
		defer done()

		if _, err := cl.OrgCreate(rpc.OrgCreateParams{Name: name, Slug: slug}); err != nil {
			return err
		}
		fmt.Printf("组织「%s」（别名 %s）已创建，你是所有者。\n", name, slug)
		fmt.Println("下一步：mindx org architecture publish 发布架构树，再 mindx org invite 邀请成员。")
		return nil
	},
}

// ── org members：在册成员清单 ─────────────────────────────────

var orgMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "List org members with their member IDs (requires mindx start)",
	RunE: func(cmd *cobra.Command, args []string) error {
		slug, _ := cmd.Flags().GetString("org")
		cl, done, err := dialOrg()
		if err != nil {
			return err
		}
		defer done()

		result, err := cl.OrgMembers(slug)
		if err != nil {
			return err
		}
		var res struct {
			Members []struct {
				MemberID    int64  `json:"member_id"`
				PersonaID   int64  `json:"persona_id"`
				DisplayName string `json:"display_name"`
				JobTitle    string `json:"job_title"`
				OrgRole     string `json:"org_role"`
			} `json:"members"`
		}
		if err := json.Unmarshal(result, &res); err != nil {
			fmt.Println(string(result))
			return nil
		}
		if len(res.Members) == 0 {
			fmt.Println("组织里还没有其他在册成员。")
			return nil
		}
		table := render.NewTable([]string{"成员编号", "姓名", "职位", "角色"}, 80)
		for _, m := range res.Members {
			table.AddRow([]string{strconv.FormatInt(m.MemberID, 10), m.DisplayName, m.JobTitle, m.OrgRole})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n共 %d 人。移除成员：mindx org remove <成员编号>\n", len(res.Members))
		return nil
	},
}

// ── org invite：定向邀请 ──────────────────────────────────────

var orgInviteCmd = &cobra.Command{
	Use:   "invite <账号> [职位]",
	Short: "Invite a person by their addressable account (requires mindx start)",
	Long: `Send an org invitation to an addressable account (username / phone /
GitHub name). The invitee joins by onboarding with the same account,
which anchors them to their slot in the published architecture chart.

Reminder: put the invitee's account into the chart and republish it
first, otherwise onboarding cannot locate their slot.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug, _ := cmd.Flags().GetString("org")
		account := args[0]
		jobTitle := ""
		if len(args) > 1 {
			jobTitle = args[1]
		}
		cl, done, err := dialOrg()
		if err != nil {
			return err
		}
		defer done()

		if _, err := cl.OrgInvite(rpc.OrgInviteParams{OrgSlug: slug, Account: account, JobTitle: jobTitle}); err != nil {
			return err
		}
		fmt.Printf("已向 %s 发出邀请（职位：%s）。\n", account, jobTitle)
		fmt.Println("对方在本机用同一账号入职核销后即占座入册。")
		return nil
	},
}

// ── org remove：移除成员 ──────────────────────────────────────

var orgRemoveCmd = &cobra.Command{
	Use:   "remove <成员编号>",
	Short: "Remove an org member by member ID (requires mindx start)",
	Long: `Remove an org member (soft delete: history kept, their seat is
released for a new member). Owners cannot be removed — transfer
ownership or delete the org instead.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug, _ := cmd.Flags().GetString("org")
		memberID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
		if err != nil || memberID <= 0 {
			return fmt.Errorf("成员编号须为正整数（mindx org members 查看编号）")
		}
		cl, done, err := dialOrg()
		if err != nil {
			return err
		}
		defer done()

		if _, err := cl.OrgMemberRemove(rpc.OrgMemberRemoveParams{OrgSlug: slug, MemberID: memberID}); err != nil {
			return err
		}
		fmt.Printf("已移除成员（编号 %d），其坐席已空出可绑新人。\n", memberID)
		return nil
	},
}

// ── org architecture：架构树 ──────────────────────────────────

var orgArchCmd = &cobra.Command{
	Use:   "architecture",
	Short: "Show or publish the org architecture chart (XML)",
	Long: `The org architecture chart is the single source of truth for who
sits where. Read it with show, publish (replace) the whole tree with
publish. See docs in the org-builder skill for the XML shape.`,
}

var orgArchShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the current architecture chart XML (requires mindx start)",
	RunE: func(cmd *cobra.Command, args []string) error {
		slug, _ := cmd.Flags().GetString("org")
		cl, done, err := dialOrg()
		if err != nil {
			return err
		}
		defer done()

		result, err := cl.OrgArchitectureGet(rpc.OrgArchGetParams{OrgSlug: slug})
		if err != nil {
			return err
		}
		var res struct {
			OrgSlug string `json:"org_slug"`
			XML     string `json:"xml"`
		}
		if err := json.Unmarshal(result, &res); err != nil {
			fmt.Println(string(result))
			return nil
		}
		fmt.Println(res.XML)
		return nil
	},
}

var orgArchPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish (replace) the architecture chart XML (requires mindx start)",
	Long: `Replace the whole architecture tree with the given XML
(--file <path> or piped stdin). The server validates the tree shape:
every member needs a name; an empty account attribute means an open slot.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		slug, _ := cmd.Flags().GetString("org")
		file, _ := cmd.Flags().GetString("file")
		xml, err := readMailBody(nil, file)
		if err != nil {
			return err
		}
		cl, done, err := dialOrg()
		if err != nil {
			return err
		}
		defer done()

		if _, err := cl.OrgArchitecturePublish(rpc.OrgArchPublishParams{OrgSlug: slug, XML: xml}); err != nil {
			return err
		}
		fmt.Println("组织架构树已发布（整树替换）。")
		fmt.Println("新成员入职前，记得先把其账号填入树并重新发布，再发邀请。")
		return nil
	},
}
