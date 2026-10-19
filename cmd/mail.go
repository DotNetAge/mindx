package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── mail parent：组织信箱（跨机器的派工与汇报载体） ───────────

var mailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Org mailbox: send work requests to teammates and report back (requires mindx start)",
	Long: `Organization mailbox — the carrier for cross-machine work.

A mail is just a letter: the recipient's agent reads it first and decides
what to do (do it now, pass it to its human partner, or just take note).
When you finish what a letter asked, reply to it so the sender knows.

Commands:

  mindx mail send <收件人> <主题> [正文...]   寄一封新信（正文可用 --body-file 或管道输入）
  mindx mail reply <信件编号> [正文...]       按编号回信给寄件人
  mindx mail inbox                           本机收件箱
  mindx mail sent [--org <组织>]             发件对账（对方是否已取、回信数）

Examples:
  mindx mail send 积木君 "季度对账求助" 帮忙查一下 9 月的对账单，明早要
  cat report.md | mindx mail reply a1b2c3d4
  mindx mail inbox`,
}

func init() {
	mailSendCmd.Flags().String("body-file", "", "从文件读取正文（指定后忽略命令行正文参数）")
	mailSentCmd.Flags().String("org", "", "目标组织（缺省取在册的第一个正常组织）")
	mailCmd.AddCommand(mailSendCmd, mailReplyCmd, mailInboxCmd, mailSentCmd)
	rootCmd.AddCommand(mailCmd)
}

// readMailBody 正文三通道：--body-file 优先，其次位置参数，最后管道 stdin。
func readMailBody(args []string, bodyFile string) (string, error) {
	if bodyFile != "" {
		data, err := os.ReadFile(bodyFile)
		if err != nil {
			return "", fmt.Errorf("读取正文文件失败: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	if len(args) > 0 {
		return strings.TrimSpace(strings.Join(args, " ")), nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("读取正文输入失败: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ── mail send：寄新信 ─────────────────────────────────────────

var mailSendCmd = &cobra.Command{
	Use:   "send <收件人> <主题> [正文...]",
	Short: "Send a mail to an org member (requires mindx start)",
	Long: `Compose and send a mail to an org member (matched by display name).

The mail first lands in the server mailbox (global mail ID, audit trail),
then the daemon relays it straight to the recipient's machine when
reachable — otherwise it waits there for the recipient to pick up.

Body sources: command-line words, --body-file <path>, or piped stdin.`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		bodyFile, _ := cmd.Flags().GetString("body-file")
		to, subject := args[0], args[1]
		body, err := readMailBody(args[2:], bodyFile)
		if err != nil {
			return err
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MailSend(rpc.MailSendParams{To: to, Subject: subject, Body: body})
		if err != nil {
			return err
		}
		var res struct {
			UID     string `json:"uid"`
			OrgSlug string `json:"org_slug"`
			To      string `json:"to"`
		}
		if err := json.Unmarshal(result, &res); err != nil {
			fmt.Println(string(result))
			return nil
		}
		fmt.Printf("已寄出给 %s（信件编号 %s）。\n", res.To, res.UID)
		fmt.Println("信已留在对方信箱，对方智能主机上线后会自动收取。")
		fmt.Printf("寄件回执可在 `mindx mail sent` 里对账；对方回信会进入 `mindx mail inbox`。\n")
		return nil
	},
}

// ── mail reply：按编号回信 ────────────────────────────────────

var mailReplyCmd = &cobra.Command{
	Use:   "reply <信件编号> [正文...]",
	Short: "Reply to a mail by its ID (requires mindx start)",
	Long: `Reply to the sender of a mail you received (matched by its mail ID,
which is shown in the inbox listing and in the wake-up briefing).

Body sources: command-line words, --body-file <path>, or piped stdin.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bodyFile, _ := cmd.Flags().GetString("body-file")
		uid := args[0]
		body, err := readMailBody(args[1:], bodyFile)
		if err != nil {
			return err
		}

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MailReply(rpc.MailReplyParams{UID: uid, Body: body})
		if err != nil {
			return err
		}
		var res struct {
			UID       string `json:"uid"`
			InReplyTo string `json:"in_reply_to"`
		}
		if err := json.Unmarshal(result, &res); err != nil {
			fmt.Println(string(result))
			return nil
		}
		fmt.Printf("已回信（信件编号 %s，回复原信 %s）。\n", res.UID, res.InReplyTo)
		fmt.Println("信已留在对方信箱，对方智能主机上线后会自动收取。")
		return nil
	},
}

// ── mail inbox：收件箱 ────────────────────────────────────────

var mailInboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "List mails delivered to this machine (requires mindx start)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MailInbox(rpc.MailListParams{})
		if err != nil {
			return err
		}
		var res struct {
			Mails []localMailRow `json:"mails"`
		}
		if err := json.Unmarshal(result, &res); err != nil {
			fmt.Println(string(result))
			return nil
		}
		if len(res.Mails) == 0 {
			fmt.Println("收件箱暂时没有信。")
			return nil
		}
		table := render.NewTable([]string{"信件编号", "寄件人", "主题", "收信时间", "回复指向"}, 80)
		for _, m := range res.Mails {
			reply := m.InReplyTo
			if reply == "" {
				reply = "—"
			}
			table.AddRow([]string{m.UID, m.FromName, m.Subject, m.ReceivedAt, reply})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n共 %d 封信。回信：mindx mail reply <信件编号> \"内容\"\n", len(res.Mails))
		return nil
	},
}

// localMailRow 收件箱条目的 CLI 视图（对齐 daemon LocalMail 的 snake_case 键）。
type localMailRow struct {
	UID        string `json:"uid"`
	FromName   string `json:"from_name"`
	Subject    string `json:"subject"`
	InReplyTo  string `json:"in_reply_to"`
	ReceivedAt string `json:"received_at"`
}

// ── mail sent：发件对账 ───────────────────────────────────────

var mailSentCmd = &cobra.Command{
	Use:   "sent",
	Short: "List mails you sent with delivery status (requires mindx start)",
	Long: `Account view of mails you sent: whether the recipient has picked
them up, and how many replies each one received.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		orgSlug, _ := cmd.Flags().GetString("org")

		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.MailSent(rpc.MailListParams{OrgSlug: orgSlug})
		if err != nil {
			return err
		}
		var res struct {
			OrgSlug string        `json:"org_slug"`
			Mails   []sentMailRow `json:"mails"`
		}
		if err := json.Unmarshal(result, &res); err != nil {
			fmt.Println(string(result))
			return nil
		}
		if len(res.Mails) == 0 {
			fmt.Println("还没有寄出过信。")
			return nil
		}
		table := render.NewTable([]string{"信件编号", "主题", "状态", "回信数", "寄出时间"}, 80)
		for _, m := range res.Mails {
			status := "已寄出（对方未取）"
			if m.Status == "delivered" {
				status = "对方已取"
			}
			table.AddRow([]string{m.UID, m.Subject, status,
				fmt.Sprintf("%d", m.ReplyCount), m.CreatedAt})
		}
		fmt.Println(table.Render())
		fmt.Printf("\n共 %d 封信（组织 %s）。\n", len(res.Mails), res.OrgSlug)
		return nil
	},
}

// sentMailRow 发件清单条目的 CLI 视图（对齐服务端 Mail 的 snake_case 键）。
type sentMailRow struct {
	UID        string `json:"uid"`
	Subject    string `json:"subject"`
	Status     string `json:"status"`
	ReplyCount int    `json:"reply_count"`
	CreatedAt  string `json:"created_at"`
}
