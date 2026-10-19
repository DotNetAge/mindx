package svc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/devgate"
)

type RPCHandlerRegistry struct {
	daemon *Daemon
}

func NewRPCHandlerRegistry(d *Daemon) *RPCHandlerRegistry {
	return &RPCHandlerRegistry{daemon: d}
}

func (r *RPCHandlerRegistry) handlers() map[string]gateway.MethodHandler {
	return map[string]gateway.MethodHandler{
		"session.list":                       r.daemon.handleSessionList,
		"session.statuses":                   r.daemon.handleSessionStatuses,
		"session.latest_by_dir":              r.daemon.handleSessionLatestByDir,
		"session.get":                        r.daemon.handleSessionGet,
		"session.meta":                       r.daemon.handleSessionMeta,
		"session.rename":                     r.daemon.handleSessionRename,
		"session.create":                     r.daemon.handleSessionCreate,
		"session.delete":                     r.daemon.handleSessionDelete,
		"session.confirm_files":              r.daemon.handleSessionConfirmFiles,
		"session.rollback_files":             r.daemon.handleSessionRollbackFiles,
		"session.context":                    r.daemon.handleSessionContext,
		"session.truncate":                   r.daemon.handleSessionTruncate,
		"session.delete_round":               r.daemon.handleSessionDeleteRound,
		"session.compact":                    r.daemon.handleSessionCompact,
		"project.list":                       r.daemon.handleProjectList,
		"agent.list":                         r.daemon.handleAgentList,
		"agent.get":                          r.daemon.handleAgentGet,
		"agent.create":                       r.daemon.handleAgentCreate,
		"agent.update":                       r.daemon.handleAgentUpdate,
		"agent.score":                        r.daemon.handleAgentScore,
		"agent.hire":                         r.daemon.handleAgentHire,
		"agent.fire":                         r.daemon.handleAgentFire,
		"agent.reload":                       r.daemon.handleAgentReload,
		"team.list":                          r.daemon.handleTeamList,
		"team.get":                           r.daemon.handleTeamGet,
		"team.update":                        r.daemon.handleTeamUpdate,
		"team.delete":                        r.daemon.handleTeamDelete,
		"team.hire":                          r.daemon.handleTeamHire,
		"team.fire":                          r.daemon.handleTeamFire,
		"team.export":                        r.daemon.handleTeamExport,
		"team.import":                        r.daemon.handleTeamImport,
		"model.list":                         r.daemon.handleModelList,
		"model.get":                          r.daemon.handleModelGet,
		"model.switch":                       r.daemon.handleModelSwitch,
		"model.create":                       r.daemon.handleModelCreate,
		"model.update":                       r.daemon.handleModelUpdate,
		"model.delete":                       r.daemon.handleModelDelete,
		"provider.list":                      r.daemon.handleProviderList,
		"provider.create":                    r.daemon.handleProviderCreate,
		"provider.update":                    r.daemon.handleProviderUpdate,
		"provider.delete":                    r.daemon.handleProviderDelete,
		"provider.fetch_ollama_models":       r.daemon.handleProviderFetchOllamaModels,
		"provider.fetch_ollama_model_detail": r.daemon.handleProviderFetchOllamaModelDetail,
		"provider.fetch_siliconflow_models":  r.daemon.handleProviderFetchSiliconFlowModels,
		"provider.fetch_openrouter_models":   r.daemon.handleProviderFetchOpenRouterModels,
		"provider.fetch_dashscope_models":    r.daemon.handleProviderFetchDashScopeModels,
		"provider.fetch_bigmodel_models":     r.daemon.handleProviderFetchBigModelModels,
		"provider.fetch_tencent_models":      r.daemon.handleProviderFetchTencentModels,
		"provider.fetch_kimi_models":         r.daemon.handleProviderFetchKimiModels,
		"provider.fetch_minimax_models":      r.daemon.handleProviderFetchMiniMaxModels,
		"skill.list":                         r.daemon.handleSkillList,
		"skill.get":                          r.daemon.handleSkillGet,
		"skill.delete":                       r.daemon.handleSkillDelete,
		"skill.reload":                       r.daemon.handleSkillReload,
		"skill.promote":                      r.daemon.handleSkillPromote,
		"agent.export":                       r.daemon.handleAgentExport,
		"agent.import":                       r.daemon.handleAgentImport,
		"skill.export":                       r.daemon.handleSkillExport,
		"skill.import":                       r.daemon.handleSkillImport,
		"skill.prepare":                      r.daemon.handleSkillPrepare,
		"skill.prepare.confirm_install":      r.daemon.handleSkillPrepareConfirmInstall,
		"skill.credential.set":               r.daemon.handleSkillCredentialSet,
		"market.list":                        r.daemon.handleMarketList,
		"market.org.list":                    r.daemon.handleMarketOrgs,
		"market.sources":                     r.daemon.handleMarketSources,
		"market.install":                     r.daemon.handleMarketInstall,
		"market.package.read":                r.daemon.handleMarketPackageRead,
		"market.rate":                        r.daemon.handleMarketRate,
		"market.rating.get":                  r.daemon.handleMarketRatingGet,
		"market.device.login":                r.daemon.handleMarketDeviceLogin,
		"market.device.status":               r.daemon.handleMarketDeviceStatus,
		"market.experience.list":             r.daemon.handleMarketExperienceList,
		"market.experience.get":              r.daemon.handleMarketExperienceGet,
		"market.account.get":                 r.daemon.handleMarketAccountGet,
		"market.account.register":            r.daemon.handleMarketAccountRegister,
		"market.account.login":               r.daemon.handleMarketAccountLogin,
		"market.account.logout":              r.daemon.handleMarketAccountLogout,
		"market.account.profile.set":         r.daemon.handleMarketAccountProfileSet,
		"market.account.devices":             r.daemon.handleMarketAccountDevices,
		"market.updateProfile":               r.daemon.handleMarketUpdateProfile,
		"market.uploadAvatar":                r.daemon.handleMarketUploadAvatar,
		"market.pack":                        r.daemon.handleMarketPack,
		"market.publish":                     r.daemon.handleMarketPublish,
		"market.submit":                      r.daemon.handleMarketSubmit,
		"market.submissions.list":            r.daemon.handleMarketSubmissionsList,
		"market.submissions.decide":          r.daemon.handleMarketSubmissionDecide,
		"message.cancel":                     r.daemon.handleMessageCancel,
		"fs.list":                            r.daemon.handleFSList,
		"fs.find":                            r.daemon.handleFSFind,
		"fs.read":                            r.daemon.handleFSRead,
		"fs.read_base64":                     r.daemon.handleFSReadBase64,
		"fs.write":                           r.daemon.handleFSWrite,
		"fs.write_base64":                    r.daemon.handleFSWriteBase64,
		"fs.home":                            r.daemon.handleFSHome,
		"fs.choose_dir":                      r.daemon.handleFSChooseDir,
		"fs.mkdir":                           r.daemon.handleFSMkdir,
		"fs.rm":                              r.daemon.handleFSRm,
		"fs.mv":                              r.daemon.handleFSMv,
		"fs.reveal":                          r.daemon.handleFSReveal,
		"fs.stat":                            r.daemon.handleFSStat,
		"git.clone":                          r.daemon.handleGitClone,
		"git.commit_message":                 r.daemon.handleCommitMessage,
		"user.config":                        r.daemon.handleUserConfig,
		"channel.status":                     r.daemon.handleChannelStatus,
		"channel.bootstrap":                  r.daemon.handleChannelBootstrap,
		"channel.approve":                    r.daemon.handleChannelApprove,
		"channel.deny":                       r.daemon.handleChannelDeny,
		"channel.verify":                     r.daemon.handleChannelVerify,
		"network.public_info":                r.daemon.handleNetworkPublicInfo,
		"network.token.get":                  r.daemon.handleNetworkTokenGet,
		"server.version":                     r.daemon.handleServerVersion,
		"server.check_update":                r.daemon.handleServerCheckUpdate,
		"server.apply_update":                r.daemon.handleServerApplyUpdate,
		"server.restart_daemon":              r.daemon.handleServerRestartDaemon,
		"token.usage.overview":               r.daemon.handleTokenUsageOverview,
		"metric.report":                      r.daemon.handleMetricReport,
		"round.rating":                       r.daemon.handleRoundRating,
		"agent.rate":                         r.daemon.handleAgentRate,
		"metric.record":                      r.daemon.handleMetricRecord,
		"metric.baseline.save":               r.daemon.handleMetricBaselineSave,
		"metric.baseline.list":               r.daemon.handleMetricBaselineList,
		"metric.compare":                     r.daemon.handleMetricCompare,
		"metric.consent.get":                 r.daemon.handleMetricConsentGet,
		"metric.consent.set":                 r.daemon.handleMetricConsentSet,
		"token.usage.monthly":                r.daemon.handleTokenUsageMonthly,
		"token.usage.by_model":               r.daemon.handleTokenUsageByModel,
		"token.usage.total":                  r.daemon.handleTokenUsageTotal,
		"token.usage.session":                r.daemon.handleTokenUsageSession,
		"token.usage.session.detail":         r.daemon.handleTokenUsageSessionDetail,
		"schedule.list":                      r.daemon.handleScheduleList,
		"schedule.add":                       r.daemon.handleScheduleAdd,
		"schedule.create":                    r.daemon.handleScheduleAdd, // 前端 ScheduleView 使用的别名
		"schedule.del":                       r.daemon.handleScheduleDelete,
		"schedule.job_cancel":                r.daemon.handleScheduleJobCancel,

		// 组织信箱（M9 跨机器派工与汇报；mail.deliver 是入站口，见 openMethods 白名单）
		"mail.send":         r.daemon.handleMailSend,
		"mail.reply":        r.daemon.handleMailReply,
		"mail.inbox":        r.daemon.handleMailInbox,
		"mail.list":         r.daemon.handleMailSent,
		"mail.members":      r.daemon.handleMailMembers,
		"log.read":          r.daemon.handleLogRead,
		"log.clear":         r.daemon.handleLogClear,
		"log.count":         r.daemon.handleLogCount,
		"i18n.get":          r.daemon.handleI18nGet,
		"i18n.switch":       r.daemon.handleI18nSwitch,
		"i18n.list":         r.daemon.handleI18nList,
		"kvstore.get":       r.daemon.handleKVGet,
		"kvstore.set":       r.daemon.handleKVSet,
		"kvstore.delete":    r.daemon.handleKVDelete,
		"kvstore.list":      r.daemon.handleKVList,
		"kvstore.batch_set": r.daemon.handleKVBatchSet,
		"kvstore.clear":     r.daemon.handleKVClear,
		"rule.list":         r.daemon.handleRuleList,
		"rule.get":          r.daemon.handleRuleGet,
		"rule.create":       r.daemon.handleRuleCreate,
		"rule.update":       r.daemon.handleRuleUpdate,
		"rule.delete":       r.daemon.handleRuleDelete,
		"terminal.start":    r.daemon.handleTerminalStart,
		"terminal.input":    r.daemon.handleTerminalInput,
		"terminal.resize":   r.daemon.handleTerminalResize,
		"terminal.kill":     r.daemon.handleTerminalKill,
		"terminal.list":     r.daemon.handleTerminalList,
		"translate.rpc":     r.daemon.handleTranslate,
		"optimize.rpc":      r.daemon.handleOptimize,
		"ui.open":           r.daemon.handleUIOpen,
		"ui.open_link":      r.daemon.handleUIOpenLink,
		"ui.run":            r.daemon.handleUIRun,

		// MCP server management（工具清单不再持久化 manifest，改为运行时 tools/list 动态发现）
		"mcp.server.add":         r.daemon.handleMCPServerAdd,
		"mcp.server.update":      r.daemon.handleMCPServerUpdate,
		"mcp.server.remove":      r.daemon.handleMCPServerRemove,
		"mcp.server.set_enabled": r.daemon.handleMCPServerSetEnabled,
		"mcp.server.list":        r.daemon.handleMCPServerList,
		"mcp.server.test":        r.daemon.handleMCPServerTest,
		"mcp.server.discover":    r.daemon.handleMCPServerDiscover,
		"mcp.tool.call":          r.daemon.handleMCPToolCall,

		// OAuth 提供商（阶段四壳侧授权配合：设置区状态 + client_id 填入 + 授权流程）
		"oauth.provider.list":      r.daemon.handleOAuthProviderList,
		"oauth.provider.configure": r.daemon.handleOAuthConfigure,
		"oauth.authorize.start":    r.daemon.handleOAuthAuthorizeStart,
		"oauth.authorize.cancel":   r.daemon.handleOAuthAuthorizeCancel,

		// 设备接入闸与 Mac 间互连（设备分组、接入批准、局域网发现、配对码）
		"device.status":    r.daemon.handleDeviceStatus,
		"device.allow.set": r.daemon.handleDeviceAllowSet,
		"device.approve":   r.daemon.handleDeviceApprove,
		"device.deny":      r.daemon.handleDeviceDeny,
		"device.unblock":   r.daemon.handleDeviceUnblock,
		"device.revoke":    r.daemon.handleDeviceRevoke,
		"device.handshake": r.daemon.handleDeviceHandshake,
		"device.pair.code": r.daemon.handleDevicePairCode,
		"device.scan":      r.daemon.handleDeviceScan,
	}
}

// openMethods 免握手白名单：device.handshake 是接入入口（自带令牌/批准闸）；
// server.version 供对方连前探测兼容；channel.bootstrap 与 channel.verify 是
// 手机端引导入口（手机链路自带同意闸 + 本地短码双重把关）；mail.deliver 是
// 组织来信的直投口——防滥用依据：方法内部必须凭发信方访问令牌经服务端内省
// 核验「直属上级/下级」关系才落档，伪造者拿不出有效令牌，且收信仅入本地留档。
var openMethods = map[string]bool{
	"device.handshake":  true,
	"server.version":    true,
	"channel.bootstrap": true,
	"channel.verify":    true,
	"mail.deliver":      true,
}

// managerMethods 管理面方法（PAT 作用域分层：远程设备禁调，仅本机直通可用）。
// 划界标准——管的是机器自身的安全态与系统态：设备管理、本机凭据下发、
// 系统更新重启、全局配置与凭据（含 API Key）写入。工作面（对话/文件/市场/
// 技能驱动）对远程设备（作用域 ask）开放。
var managerMethods = map[string]bool{
	// 设备管理：接入名单与配对只有本机主人能改
	"device.allow.set": true,
	"device.approve":   true,
	"device.deny":      true,
	"device.unblock":   true,
	"device.revoke":    true,
	"device.pair.code": true,
	// 本机凭据：不下发给外接设备（防换取不可撤销的本机身份）
	"network.token.get": true,
	// 系统操作
	"server.apply_update":   true,
	"server.restart_daemon": true,
	// 全局配置与日志
	"user.config": true,
	"i18n.switch": true,
	"log.clear":   true,
	// 全局 KV 存储
	"kvstore.set":       true,
	"kvstore.batch_set": true,
	"kvstore.delete":    true,
	"kvstore.clear":     true,
	// 凭据写入
	"skill.credential.set": true,
	// 手机配对审批（本机主人的同意动作）
	"channel.approve": true,
	"channel.deny":    true,
	// OAuth 配置（含密钥写入与授权流程）
	"oauth.provider.configure": true,
	"oauth.authorize.start":    true,
	"oauth.authorize.cancel":   true,
	// Provider 管理（含 API Key）
	"provider.create": true,
	"provider.update": true,
	"provider.delete": true,
}

func (r *RPCHandlerRegistry) RegisterAll(gw *gateway.Server) {
	for method, handler := range r.handlers() {
		gw.RegisterMethod(method, r.guard(method, handler))
	}
}

// guard 设备接入强制闸：客户端必须先经 device.handshake 完成接入验证
// （本机令牌直通或已授权设备 token）才能调用其余方法；未验证客户端
// 收到统一的人话错误。闸未装配（devgate 初始化失败降级）时放行。
// 闸之上叠加作用域闸（PAT 分层授权）：远程设备（ask）不得调用管理面
// 方法——设备管理、本机凭据、系统与全局配置变更仅限本机直通连接。
func (r *RPCHandlerRegistry) guard(method string, h gateway.MethodHandler) gateway.MethodHandler {
	if openMethods[method] {
		return h
	}
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		clientID := gateway.ClientIDFromContext(ctx)
		if !r.daemon.deviceGateAuthorized(clientID) {
			return nil, fmt.Errorf("这台设备还没有完成接入验证，请先在设备端完成连接后再操作")
		}
		if managerMethods[method] && r.daemon.devGate != nil && r.daemon.devGate.ScopeOf(clientID) != devgate.ScopeLocal {
			return nil, fmt.Errorf("这项操作涉及本机管理，请直接在主机上操作")
		}
		return h(ctx, params)
	}
}
