package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/goharness/tools"
)

// 本工具把「升级已装的市场包」开放给 Agent。两侧纪律（写在 Prompt 里，模型必须遵守）：
//
//   - 只在用户明确要求升级时调用；不主动、不周期性地替用户升级；
//   - 本地内容被改过或无法判定时，工具只返回 needs_confirm，不覆盖。必须先把情况
//     告诉用户，得到同意后才以 force=true 重试。
//
// 为什么这两条不能靠代码兜底：工具无法判断「用户是不是真的说了要升级」，
// 只能靠模型据对话判断；代码能兜住的是「不确认就不覆盖」——这一侧已经兜住了。

// BundleUpdateTarget 一个已装包的升级对照（工具返回给模型的行）。
type BundleUpdateTarget struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	LocalVersion  string `json:"local_version,omitempty"`
	LatestVersion string `json:"latest_version,omitempty"`
	Upgradable    bool   `json:"upgradable"`
	// Modified 本地内容是否被改过；nil = 判不出来（旧安装没记版本，或本地那版已下架）
	Modified *bool  `json:"modified,omitempty"`
	Note     string `json:"note,omitempty"`
}

// BundleUpgradeOutcome 单包升级结果。
type BundleUpgradeOutcome struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Applied     bool   `json:"applied"`
	Status      string `json:"status"` // upgraded | up_to_date | needs_confirm | rolled_back
	FromVersion string `json:"from_version,omitempty"`
	ToVersion   string `json:"to_version,omitempty"`
	Backup      string `json:"backup,omitempty"`
	Note        string `json:"note,omitempty"`
}

// BundleUpgradeBackend 由 daemon 注入（tools 包不能反向依赖 svc）。
// 实现必须与界面按钮、CLI 走同一条执行体，否则会出现两套判定。
type BundleUpgradeBackend interface {
	CheckBundleUpdates(kind, name string) ([]BundleUpdateTarget, error)
	UpgradeBundle(kind, name string, force bool, version string) (BundleUpgradeOutcome, error)
	// AdoptPackages 把「版本字段出现之前装上的包」纳入版本管理：定义文件能对上某一版
	// 就写那一版；社区没有同名包则按自建起版（详见 bundle.adopt 的说明）。
	AdoptPackages(kind, name string, dryRun bool) (AdoptSummary, error)
}

// AdoptRow 单个包的纳管结果。
type AdoptRow struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Adopted bool   `json:"adopted"`
	Reason  string `json:"reason,omitempty"`
}

// AdoptSummary 纳管汇总。
type AdoptSummary struct {
	Items   []AdoptRow `json:"items"`
	Adopted int        `json:"adopted"`
	Pending int        `json:"pending"`
	Errors  []string   `json:"errors,omitempty"`
}

// BundleUpgrade 升级已安装的 Agent / 团队 / 技能包。
type BundleUpgrade struct {
	backend BundleUpgradeBackend
}

// NewBundleUpgrade 创建升级工具（backend 为空时不注册）。
func NewBundleUpgrade(backend BundleUpgradeBackend) tools.FuncTool {
	return &BundleUpgrade{backend: backend}
}

func (t *BundleUpgrade) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:         "BundleUpgrade",
		Description:  "检查并升级本机已安装的 Agent、团队、技能包到社区发布的新版本。",
		IsReadOnly:   false,
		IsIdempotent: false,
		TimeoutSec:   600,
		Prompt: `检查并升级本机已安装的 Agent / 团队 / 技能包。

调用时机（严格）：
- 只有用户明确要求「看看有没有新版本」「升级某个成员/团队/技能」时才调用。
- 不要主动巡检，不要在对话里自作主张地升级——升级会覆盖本机文件。

操作：
- **check**：列出已装包与社区最新版本的对照。kind 可省略（=三类全查），也可只查某一类；给 name 则只查那一个包。只读，不改变任何文件。
- **upgrade**：执行升级。kind 必填；name 省略时升级该类全部可升级的包。version 用于回滚到指定版本（省略=升到最新）。

本地改动与 force：
- 升级前会判断「本机的定义文件是否与已装那版一致」（解析 frontmatter 后比对，忽略排版与雇佣标记；
  包内附带的技能副本不参与——技能有自己的版本链）。
- 一致 → 直接升级（覆盖前自动整目录备份）。
- 不一致或判不出来 → 工具**不会覆盖**，返回 status="needs_confirm"。此时必须先把「哪个包被改过/判不出来、升级会覆盖哪些文件」告诉用户，用户明确同意后才以 force=true 重试。
- 永远不要为了「省一步」自己加 force。

旧安装（没有版本号）：
- 这类包查不到版本，也就收不到更新提示。用 action="adopt"（必须给 kind）可以为它们补上版本号：
  拿本机定义文件与社区的各个版本比对，只有能对上某一版时才写（对上就不会认错），
  一个版本都对不上就如实报「认不出来」；社区根本没有同名包时，视为本地自建，起版 1.0.0。
  这只是补一个字段，不改任何内容。

结果怎么汇报：用包名 + 版本变化说清楚（例如「dashboard-builder 从 1.0.0 升到 1.0.1」），
不要念 JSON，也不要暴露备份目录路径之外的内部细节。`,
		Parameters: []tools.Parameter{
			{
				Name:        "action",
				Type:        "string",
				Description: "操作类型：\"check\" 只查不改，\"upgrade\" 执行升级，\"adopt\" 给旧安装补版本号。",
				Required:    true,
				Enum:        []any{"check", "upgrade", "adopt"},
			},
			{
				Name:        "kind",
				Type:        "string",
				Description: "包类型：\"agent\"（成员）、\"team\"（团队）、\"skill\"（技能）。check 可省略表示三类全查；upgrade 必填。",
				Required:    false,
				Enum:        []any{"agent", "team", "skill"},
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "包名。check 时省略 = 查全部；upgrade 时省略 = 升级该类型下全部可升级的包。",
				Required:    false,
			},
			{
				Name:        "version",
				Type:        "string",
				Description: "目标版本号（仅 upgrade）。省略 = 升到社区最新；给出旧版本号即回滚。只对单个包有效。",
				Required:    false,
			},
			{
				Name:        "force",
				Type:        "boolean",
				Description: "本地内容被改过或无法判定时仍覆盖。仅在对应用户明确同意后使用；每次覆盖前自动备份。",
				Required:    false,
				Default:     false,
			},
		},
	}
}

func (t *BundleUpgrade) Execute(_ context.Context, params map[string]any) (any, error) {
	if t.backend == nil {
		return nil, fmt.Errorf("%s", tools.GuideMissingContext("BundleUpgrade", "市场服务（daemon 未注入升级后端）"))
	}
	action, err := tools.ValidateRequiredString("BundleUpgrade", params, "action")
	if err != nil {
		return nil, err
	}
	kind, name, version := paramString(params, "kind"), paramString(params, "name"), paramString(params, "version")
	force := paramBool(params, "force")

	if kind != "" && !validBundleKind(kind) {
		return nil, fmt.Errorf("%s", tools.GuideInvalidValue("BundleUpgrade", "kind", kind,
			"先自查：kind 只支持 agent（成员）/ team（团队）/ skill（技能），对照工具参数定义确认后重新调用"))
	}

	switch action {
	case "check":
		items, err := t.backend.CheckBundleUpdates(kind, name)
		if err != nil {
			return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
				"检查包版本时失败",
				tools.WithErrDetail("无法从社区获取已装包的最新版本信息", err),
				"先自查：社区是否可访问（网络/登录状态）？确认后重试；若仍失败应如实告知用户当前查不了，不要猜测版本",
			), err)
		}
		upgradable := 0
		for _, it := range items {
			if it.Upgradable {
				upgradable++
			}
		}
		resp := map[string]any{
			"total":      len(items),
			"upgradable": upgradable,
			"items":      items,
		}
		if upgradable == 0 {
			resp["summary"] = "本机已装包都是最新版本。"
		}
		return resp, nil

	case "upgrade":
		if kind == "" {
			return nil, fmt.Errorf("%s", tools.GuideInvalidValue("BundleUpgrade", "kind", kind,
				"先自查：upgrade 必须指定 kind（agent / team / skill）；若用户说的是「全部升级」，按类型各调一次"))
		}
		if version != "" && name == "" {
			return nil, fmt.Errorf("%s", tools.GuideInvalidValue("BundleUpgrade", "version", version,
				"先自查：version 只对单个包有意义，请同时给出 name；要批量升级请不要传 version"))
		}

		// 先查一次再动手：单包升级也要拿到「改没改过」的判定（否则覆盖确认的原因说不清），
		// 顺便挡掉「其实已是最新」的无谓覆盖。回滚（给了 version）不受「可升级」过滤。
		items, err := t.backend.CheckBundleUpdates(kind, name)
		if err != nil {
			return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
				"升级前检查失败",
				tools.WithErrDetail("无法获取可升级清单，未做任何改动", err),
				"先自查：社区是否可访问？确认后重试；失败时告知用户未做任何改动",
			), err)
		}
		rollingBack := version != ""
		targets := make([]BundleUpdateTarget, 0, len(items))
		for _, it := range items {
			if rollingBack || it.Upgradable {
				targets = append(targets, it)
			}
		}
		if len(targets) == 0 {
			return map[string]any{
				"upgraded":      0,
				"skipped":       0,
				"needs_confirm": 0,
				"results":       []BundleUpgradeOutcome{},
				"summary":       "没有可升级的包，未做任何改动。",
			}, nil
		}

		results := make([]BundleUpgradeOutcome, 0, len(targets))
		var upgraded, skipped, needsConfirm int
		for _, tg := range targets {
			out, err := t.backend.UpgradeBundle(tg.Kind, tg.Name, force, version)
			if err != nil {
				// 单包失败不吞掉：如实回报，其余包继续（用户要的是「能升的都升掉」）
				results = append(results, BundleUpgradeOutcome{
					Kind: tg.Kind, Name: tg.Name, Status: "failed", Note: err.Error(),
				})
				skipped++
				continue
			}
			if out.FromVersion == "" {
				out.FromVersion = tg.LocalVersion
			}
			if out.ToVersion == "" {
				out.ToVersion = tg.LatestVersion
			}
			switch out.Status {
			case "upgraded", "rolled_back":
				upgraded++
			case "needs_confirm":
				needsConfirm++
				out.Note = confirmReason(tg)
			case "up_to_date":
				skipped++
			default:
				skipped++
			}
			results = append(results, out)
		}

		resp := map[string]any{
			"upgraded":      upgraded,
			"skipped":       skipped,
			"needs_confirm": needsConfirm,
			"results":       results,
		}
		if needsConfirm > 0 {
			resp["hint"] = "有包因为本地内容被改过（或无法确认）没有升级。先把这些包告诉用户并说明升级会覆盖本机文件；" +
				"用户明确同意后再以 force=true 重试，不要自行决定覆盖。"
		}
		return resp, nil

	case "adopt":
		// 给「版本字段出现之前装上的包」补版本号：定义文件能对上某一版才写，
		// 认不出来就如实说。不改任何内容，所以可以被模型安全地调用。
		// 必须指定 kind：纳管要逐个版本下载比对，一次跑三类可能几百次下载，
		// 既超出单次调用的合理时长，也让用户看不到中间进展。
		if kind == "" {
			return nil, fmt.Errorf("%s", tools.GuideInvalidValue("BundleUpgrade", "kind", kind,
				"先自查：纳入版本管理必须指定 kind（agent / team / skill），一次处理一类；用户若说「全部」，按类型分三次调用"))
		}
		summary, err := t.backend.AdoptPackages(kind, name, false)
		if err != nil {
			return nil, fmt.Errorf("%s（原始错误：%w）", tools.BuildGuide(
				"纳入版本管理时失败",
				tools.WithErrDetail("无法完成比对或写回，可能只处理了一部分", err),
				"先自查：社区是否可访问？失败时如实告知用户，不要声称已全部纳管",
			), err)
		}
		return summary, nil

	default:
		return nil, fmt.Errorf("%s", tools.GuideInvalidValue("BundleUpgrade", "action", action,
			"先自查：action 只支持 check / upgrade / adopt 三者之一，对照工具参数定义确认拼写后重新调用"))
	}
}

// confirmReason 组装「为什么需要确认」的一句话（模型据此向用户解释）。
func confirmReason(tg BundleUpdateTarget) string {
	switch {
	case tg.Modified != nil && *tg.Modified:
		return "本地内容自安装后被修改过，升级会覆盖这些修改"
	case tg.Note != "":
		return tg.Note
	default:
		return "无法确认本地内容是否被改过，升级会覆盖本机文件"
	}
}

// validBundleKind 包类型合法性。
func validBundleKind(kind string) bool {
	return kind == "agent" || kind == "team" || kind == "skill"
}

// paramString 读取字符串参数（缺省空串）。
func paramString(params map[string]any, key string) string {
	raw, ok := getParam(params, key)
	if !ok {
		return ""
	}
	s, _ := raw.(string)
	return strings.TrimSpace(s)
}

// paramBool 读取布尔参数（缺省 false）。
func paramBool(params map[string]any, key string) bool {
	raw, ok := getParam(params, key)
	if !ok {
		return false
	}
	v, _ := raw.(bool)
	return v
}
