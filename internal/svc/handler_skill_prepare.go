package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/internal/core/skillstore"
)

// ---------------------------------------------------------------------------
// skill.prepare / skill.prepare.confirm_install —— 技能准备流程（阶段五）
//
// 定稿（TODO.md「对Skills的require进行重定义」）：凭据三层解析链缺失时触发
// 准备流程——有 oauth 走授权（daemon 内部启动 runOAuthFlow，存储键
// <skill_name>:<env_name>，壳经 present_* 全局订阅自动弹授权弹窗）；有
// install 命令块走「展示确认一次后自动执行」；都没有则报环境不满足。
//
// 状态机（单次调用只返回一个阻塞态，壳处理后重新调用推进）：
//   ok            全部就绪
//   oauth_started 授权流程已异步启动（outcome 经 oauth.flow_completed/failed 通知）
//   needs_confirm 安装命令待确认（confirm 后经 skill.prepare.confirm_install 执行）
//   blocked       环境不满足（无法自动修复的原因清单）
//
// market.install / skill.import 安装完成后自动调用一次（handler_bundle.go）：
// 授权直接启动，命令确认随安装响应带回壳侧裁决——满足「安装时触发准备流程」。
// ---------------------------------------------------------------------------

// skillPrepareResult 准备流程状态（RPC 响应与安装响应 prepares[] 条目共用形状）
type skillPrepareResult struct {
	Skill         string          `json:"skill"`
	Status        string          `json:"status"`
	Warnings      []string        `json:"warnings,omitempty"`
	Provider      string          `json:"provider,omitempty"`
	ProviderTitle string          `json:"provider_title,omitempty"`
	Env           string          `json:"env,omitempty"`
	Commands      []string        `json:"commands,omitempty"`
	Platform      string          `json:"platform,omitempty"`
	Message       string          `json:"message,omitempty"`
	Results       []commandResult `json:"results,omitempty"`
}

// commandResult 单条安装命令的执行结果
type commandResult struct {
	Command string `json:"command"`
	Output  string `json:"output,omitempty"`
	OK      bool   `json:"ok"`
}

// 准备状态常量
const (
	prepareOK           = "ok"
	prepareOAuthStarted = "oauth_started"
	prepareNeedsConfirm = "needs_confirm"
	prepareBlocked      = "blocked"
)

// handleSkillPrepare 处理 skill.prepare：跑一遍准备检查并推进（幂等可重入，
// 授权流程进行中不重复启动，返回 oauth_started 让壳继续等通知）。
func (d *Daemon) handleSkillPrepare(_ context.Context, params json.RawMessage) (any, error) {
	name, err := skillPrepareNameParam(params)
	if err != nil {
		return nil, err
	}
	res := d.prepareSkill(name)
	return res, nil
}

// handleSkillPrepareConfirmInstall 处理 skill.prepare.confirm_install：执行
// 技能声明的本平台安装命令（壳已展示清单并确认一次），随后重跑准备检查返回收尾状态。
func (d *Daemon) handleSkillPrepareConfirmInstall(ctx context.Context, params json.RawMessage) (any, error) {
	name, err := skillPrepareNameParam(params)
	if err != nil {
		return nil, err
	}
	return d.confirmSkillInstall(ctx, name), nil
}

// skillPrepareNameParam 提取并校验 name 参数
func skillPrepareNameParam(params json.RawMessage) (string, error) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &p); err != nil || strings.TrimSpace(p.Name) == "" {
		return "", fmt.Errorf("缺少 name 参数")
	}
	return strings.TrimSpace(p.Name), nil
}

// loadSkillDetail 从全局技能库定位技能目录并加载依赖声明。
func (d *Daemon) loadSkillDetail(name string) (*skillstore.SkillDetail, error) {
	sk, err := d.app.Skills().Global().GetSkill(name)
	if err != nil || sk == nil {
		return nil, fmt.Errorf("技能 %q 不存在", name)
	}
	detail, _, err := skillstore.LoadSkillDetailFromDir(sk.RootDir, "filesystem")
	if err != nil {
		return nil, fmt.Errorf("读取技能 %q 的依赖声明失败：%w", name, err)
	}
	if detail == nil {
		return nil, fmt.Errorf("技能 %q 缺少 SKILL.md", name)
	}
	return detail, nil
}

// prepareSkill 准备检查主流程。返回值永不阻断调用方（安装响应附带时不产生错误）。
func (d *Daemon) prepareSkill(name string) *skillPrepareResult {
	detail, err := d.loadSkillDetail(name)
	if err != nil {
		return &skillPrepareResult{Skill: name, Status: prepareBlocked, Message: err.Error()}
	}
	return d.evaluatePreparation(name, detail)
}

// evaluatePreparation 对已加载的技能声明跑准备检查（与技能定位解耦，便于单测）。
func (d *Daemon) evaluatePreparation(name string, detail *skillstore.SkillDetail) *skillPrepareResult {
	res := &skillPrepareResult{Skill: name, Status: prepareOK}

	var reasons []string
	oauth := (*skillstore.OAuthRequirement)(nil)
	if detail.Requires != nil {
		oauth = detail.Requires.OAuth

		// ① OAuth 凭据：缺失 → 注册表查配置态 → 已配置即启动授权（异步，
		// 壳经全局订阅自动弹窗）；未配置/未知提供商记为阻塞原因。
		if oauth != nil {
			if _, ok := core.ResolveSkillCredential(d.oauthCredStore, name, oauth.Env); !ok {
				p, pok := d.oauthRegistryOrEmpty().Provider(oauth.Provider)
				switch {
				case !pok || p == nil:
					reasons = append(reasons, fmt.Sprintf("未知的授权提供商 %q", oauth.Provider))
				case strings.TrimSpace(effectiveClientID(p, d.oauthCredStore)) == "":
					reasons = append(reasons, fmt.Sprintf("提供商 %q 尚未配置 Client ID，请先在设置的 OAuth 提供商页填入", p.Title))
				default:
					storeKey := core.SkillCredentialKey(name, oauth.Env)
					if !d.oauthFlows.running(p.Name) {
						d.oauthFlows.start(p.Name, func(ctx context.Context) {
							d.runOAuthFlow(ctx, p.Name, storeKey)
						})
					}
					res.Status = prepareOAuthStarted
					res.Provider = p.Name
					res.ProviderTitle = p.Title
					res.Env = oauth.Env
					return res // 交互流程已启动，其余检查待收尾后重入推进
				}
			}
		}

		// ② 其余环境变量（非 oauth 落点）：缺失无可修复动作 → 记为阻塞原因
		for _, env := range detail.Requires.Env {
			if oauth != nil && env == oauth.Env {
				continue
			}
			if _, ok := core.ResolveSkillCredential(d.oauthCredStore, name, env); !ok {
				reasons = append(reasons, fmt.Sprintf("缺少环境变量 %s", env))
			}
		}
	}

	// ③ 二进制缺失：有本平台 install 命令块 → needs_confirm；否则记为阻塞原因
	if missing := skillstore.MissingBins(detail.Requires); len(missing) > 0 {
		commands, platform := platformInstallCommands(detail.InstallCommands)
		if len(commands) > 0 {
			res.Status = prepareNeedsConfirm
			res.Commands = commands
			res.Platform = platform
			res.Warnings = append(reasons, fmt.Sprintf("缺少依赖的二进制：%s", strings.Join(missing, "、")))
			return res
		}
		reasons = append(reasons, fmt.Sprintf("缺少依赖的二进制：%s（技能未提供 %s 平台的安装命令）", strings.Join(missing, "、"), runtime.GOOS))
	}

	if len(reasons) > 0 {
		res.Status = prepareBlocked
		res.Message = strings.Join(reasons, "；")
	}
	return res
}

// platformInstallCommands 取当前平台的安装命令块；未命中运行平台时回退 linux
// （darwin 命令在 linux 上不可用，反之通常兼容）。
func platformInstallCommands(install map[string][]string) ([]string, string) {
	if cmds, ok := install[runtime.GOOS]; ok && len(cmds) > 0 {
		return cmds, runtime.GOOS
	}
	if runtime.GOOS != "linux" {
		if cmds, ok := install["linux"]; ok && len(cmds) > 0 {
			return cmds, "linux"
		}
	}
	return nil, ""
}

// installCommandTimeout 单条安装命令的执行上限（包管理器安装可能较慢）
const installCommandTimeout = 10 * time.Minute

// confirmSkillInstall 执行本平台安装命令（用户已在壳侧确认一次），随后重跑
// 准备检查返回收尾状态。命令经登录 shell 执行以获得完整用户 PATH
// （GUI 拉起的 daemon PATH 精简，brew 等工具需用户 profile 注入）。
func (d *Daemon) confirmSkillInstall(ctx context.Context, name string) *skillPrepareResult {
	res := &skillPrepareResult{Skill: name, Status: prepareOK}

	detail, err := d.loadSkillDetail(name)
	if err != nil {
		res.Status = prepareBlocked
		res.Message = err.Error()
		return res
	}
	commands, platform := platformInstallCommands(detail.InstallCommands)
	if len(commands) == 0 {
		res.Status = prepareBlocked
		res.Message = fmt.Sprintf("技能 %q 没有提供 %s 平台的安装命令", name, runtime.GOOS)
		return res
	}
	res.Platform = platform
	res.Commands = commands

	for _, cmd := range commands {
		res.Results = append(res.Results, runInstallCommand(ctx, cmd))
	}

	// 执行完重跑准备检查：命令全部生效 → ok；仍有缺失 → 下一个阻塞态
	final := d.evaluatePreparation(name, detail)
	res.Status = final.Status
	res.Provider = final.Provider
	res.ProviderTitle = final.ProviderTitle
	res.Env = final.Env
	res.Message = final.Message
	if len(final.Commands) > 0 {
		res.Commands = final.Commands
	}
	if len(final.Warnings) > 0 {
		res.Warnings = final.Warnings
	}
	return res
}

// installWithPrepare 安装结果 + 准备流程态（bundle.InstallResult 字段 JSON 摊平）
type installWithPrepare struct {
	bundle.InstallResult
	Prepares []skillPrepareResult `json:"prepares,omitempty"`
}

// prepareInstalledSkills 安装完成后对落入全局库的技能自动跑一次准备流程
// （定稿「安装时触发准备流程」：授权直接启动，命令确认随响应带回壳侧裁决）。
// ok 且无警告的技能省略；永不产生错误——安装已成功，准备态属附加信息。
func (d *Daemon) prepareInstalledSkills(names []string) []skillPrepareResult {
	if len(names) == 0 {
		return nil
	}
	out := make([]skillPrepareResult, 0, len(names))
	for _, name := range names {
		res := d.prepareSkill(name)
		if res.Status == prepareOK && len(res.Warnings) == 0 {
			continue
		}
		out = append(out, *res)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// prepareAndAutoInstall 安装后免确认准备（market.install 的 auto_install 契约）：
// 逐技能评估，needs_confirm（缺二进制 + 有安装命令）直接执行命令后收尾评估；
// 其余状态（oauth_started / blocked）与常规准备一致，由调用方按状态呈现。
// ok 且无警告的技能省略；永不产生错误——安装已成功，准备态属附加信息。
func (d *Daemon) prepareAndAutoInstall(ctx context.Context, names []string) []skillPrepareResult {
	if len(names) == 0 {
		return nil
	}
	out := make([]skillPrepareResult, 0, len(names))
	for _, name := range names {
		res := d.prepareSkill(name)
		if res.Status == prepareNeedsConfirm {
			res = d.confirmSkillInstall(ctx, name)
		}
		if res.Status == prepareOK && len(res.Warnings) == 0 {
			continue
		}
		out = append(out, *res)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// runInstallCommand 经登录 shell 执行一条安装命令并收集输出（成功判定取退出码）。
func runInstallCommand(ctx context.Context, command string) commandResult {
	out := commandResult{Command: command}

	runCtx, cancel := context.WithTimeout(ctx, installCommandTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		// -l 登录 shell 注入用户 PATH（Homebrew 等），-i 交互 rc 覆盖 nvm 类工具
		cmd = exec.CommandContext(runCtx, "zsh", "-lic", command)
	} else {
		cmd = exec.CommandContext(runCtx, "sh", "-c", command)
	}
	output, err := cmd.CombinedOutput()
	out.Output = strings.TrimSpace(string(output))
	out.OK = err == nil
	return out
}
