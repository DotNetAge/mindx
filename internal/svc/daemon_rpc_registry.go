package svc

import (
	"github.com/DotNetAge/gort/pkg/gateway"
)

type RPCHandlerRegistry struct {
	daemon *Daemon
}

func NewRPCHandlerRegistry(d *Daemon) *RPCHandlerRegistry {
	return &RPCHandlerRegistry{daemon: d}
}

func (r *RPCHandlerRegistry) RegisterAll(gw *gateway.Server) {
	gw.RegisterMethod("session.list", r.daemon.handleSessionList)
	gw.RegisterMethod("session.get", r.daemon.handleSessionGet)
	gw.RegisterMethod("session.meta", r.daemon.handleSessionMeta)
	gw.RegisterMethod("session.create", r.daemon.handleSessionCreate)
	gw.RegisterMethod("session.delete", r.daemon.handleSessionDelete)
	gw.RegisterMethod("session.confirm_files", r.daemon.handleSessionConfirmFiles)
	gw.RegisterMethod("session.rollback_files", r.daemon.handleSessionRollbackFiles)

	gw.RegisterMethod("memory.query", r.daemon.handleMemoryQuery)
	gw.RegisterMethod("memory.store", r.daemon.handleMemoryStore)
	gw.RegisterMethod("memory.delete", r.daemon.handleMemoryDelete)

	gw.RegisterMethod("agent.list", r.daemon.handleAgentList)
	gw.RegisterMethod("agent.get", r.daemon.handleAgentGet)
	gw.RegisterMethod("agent.create", r.daemon.handleAgentCreate)
	gw.RegisterMethod("agent.update", r.daemon.handleAgentUpdate)
	gw.RegisterMethod("agent.score", r.daemon.handleAgentScore)

	gw.RegisterMethod("model.list", r.daemon.handleModelList)
	gw.RegisterMethod("model.get", r.daemon.handleModelGet)
	gw.RegisterMethod("model.switch", r.daemon.handleModelSwitch)
	gw.RegisterMethod("model.create", r.daemon.handleModelCreate)
	gw.RegisterMethod("model.update", r.daemon.handleModelUpdate)
	gw.RegisterMethod("model.delete", r.daemon.handleModelDelete)

	gw.RegisterMethod("provider.list", r.daemon.handleProviderList)
	gw.RegisterMethod("provider.create", r.daemon.handleProviderCreate)
	gw.RegisterMethod("provider.update", r.daemon.handleProviderUpdate)
	gw.RegisterMethod("provider.delete", r.daemon.handleProviderDelete)

	gw.RegisterMethod("skill.list", r.daemon.handleSkillList)
	gw.RegisterMethod("skill.get", r.daemon.handleSkillGet)

	gw.RegisterMethod("ask_user.reply", r.daemon.handleAskUserReply)
	gw.RegisterMethod("permission.reply", r.daemon.handlePermissionReply)
	gw.RegisterMethod("message.cancel", r.daemon.handleMessageCancel)

	gw.RegisterMethod("fs.list", r.daemon.handleFSList)
	gw.RegisterMethod("fs.home", r.daemon.handleFSHome)

	gw.RegisterMethod("user.config", r.daemon.handleUserConfig)

	gw.RegisterMethod("token.usage.overview", r.daemon.handleTokenUsageOverview)
	gw.RegisterMethod("token.usage.monthly", r.daemon.handleTokenUsageMonthly)
	gw.RegisterMethod("token.usage.by_model", r.daemon.handleTokenUsageByModel)

	gw.RegisterMethod("schedule.list", r.daemon.handleScheduleList)

	gw.RegisterMethod("log.read", r.daemon.handleLogRead)
	gw.RegisterMethod("log.clear", r.daemon.handleLogClear)
	gw.RegisterMethod("log.count", r.daemon.handleLogCount)

	gw.RegisterMethod("i18n.get", r.daemon.handleI18nGet)
	gw.RegisterMethod("i18n.switch", r.daemon.handleI18nSwitch)
	gw.RegisterMethod("i18n.list", r.daemon.handleI18nList)
}
