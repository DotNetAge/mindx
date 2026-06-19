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
	gw.RegisterMethod("session.truncate", r.daemon.handleSessionTruncate)

	gw.RegisterMethod("memory.query", r.daemon.handleMemoryQuery)
	gw.RegisterMethod("memory.store", r.daemon.handleMemoryStore)
	gw.RegisterMethod("memory.delete", r.daemon.handleMemoryDelete)
	gw.RegisterMethod("memory.chunks", r.daemon.handleMemoryChunks)
	gw.RegisterMethod("memory.get_chunks", r.daemon.handleMemoryGetChunks)
	gw.RegisterMethod("memory.count", r.daemon.handleMemoryCount)
	gw.RegisterMethod("memory.stats", r.daemon.handleMemoryStats)
	gw.RegisterMethod("memory.sync_project", r.daemon.handleMemorySyncProject)

	gw.RegisterMethod("agent.list", r.daemon.handleAgentList)
	gw.RegisterMethod("agent.get", r.daemon.handleAgentGet)
	gw.RegisterMethod("agent.create", r.daemon.handleAgentCreate)
	gw.RegisterMethod("agent.update", r.daemon.handleAgentUpdate)
	gw.RegisterMethod("agent.score", r.daemon.handleAgentScore)
	gw.RegisterMethod("agent.reload", r.daemon.handleAgentReload)

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
	gw.RegisterMethod("skill.reload", r.daemon.handleSkillReload)

	gw.RegisterMethod("ask_user.reply", r.daemon.handleAskUserReply)
	gw.RegisterMethod("permission.reply", r.daemon.handlePermissionReply)
	gw.RegisterMethod("message.cancel", r.daemon.handleMessageCancel)

	gw.RegisterMethod("fs.list", r.daemon.handleFSList)
	gw.RegisterMethod("fs.read", r.daemon.handleFSRead)
	gw.RegisterMethod("fs.write", r.daemon.handleFSWrite)
	gw.RegisterMethod("fs.home", r.daemon.handleFSHome)
	gw.RegisterMethod("fs.mkdir", r.daemon.handleFSMkdir)
	gw.RegisterMethod("fs.rm", r.daemon.handleFSRm)
	gw.RegisterMethod("fs.mv", r.daemon.handleFSMv)
	gw.RegisterMethod("fs.reveal", r.daemon.handleFSReveal)

	gw.RegisterMethod("user.config", r.daemon.handleUserConfig)

	gw.RegisterMethod("server.version", r.daemon.handleServerVersion)
	gw.RegisterMethod("server.check_update", r.daemon.handleServerCheckUpdate)
	gw.RegisterMethod("server.apply_update", r.daemon.handleServerApplyUpdate)
	gw.RegisterMethod("server.restart_daemon", r.daemon.handleServerRestartDaemon)

	gw.RegisterMethod("token.usage.overview", r.daemon.handleTokenUsageOverview)
	gw.RegisterMethod("token.usage.monthly", r.daemon.handleTokenUsageMonthly)
	gw.RegisterMethod("token.usage.by_model", r.daemon.handleTokenUsageByModel)
	gw.RegisterMethod("token.usage.total", r.daemon.handleTokenUsageTotal)
	gw.RegisterMethod("token.usage.session", r.daemon.handleTokenUsageSession)

	gw.RegisterMethod("schedule.list", r.daemon.handleScheduleList)
	gw.RegisterMethod("schedule.add", r.daemon.handleScheduleAdd)
	gw.RegisterMethod("schedule.del", r.daemon.handleScheduleDelete)

	gw.RegisterMethod("log.read", r.daemon.handleLogRead)
	gw.RegisterMethod("log.clear", r.daemon.handleLogClear)
	gw.RegisterMethod("log.count", r.daemon.handleLogCount)

	gw.RegisterMethod("i18n.get", r.daemon.handleI18nGet)
	gw.RegisterMethod("i18n.switch", r.daemon.handleI18nSwitch)
	gw.RegisterMethod("i18n.list", r.daemon.handleI18nList)

	gw.RegisterMethod("graph.query", r.daemon.handleGraphQuery)
	gw.RegisterMethod("graph.exec", r.daemon.handleGraphExec)
	gw.RegisterMethod("graph.upsert_nodes", r.daemon.handleGraphUpsertNodes)
	gw.RegisterMethod("graph.upsert_edges", r.daemon.handleGraphUpsertEdges)
	gw.RegisterMethod("graph.get_node", r.daemon.handleGraphGetNode)
	gw.RegisterMethod("graph.get_neighbors", r.daemon.handleGraphGetNeighbors)
	gw.RegisterMethod("graph.list_nodes", r.daemon.handleGraphListNodes)
	gw.RegisterMethod("graph.list_edges", r.daemon.handleGraphListEdges)

	gw.RegisterMethod("kvstore.get", r.daemon.handleKVGet)
	gw.RegisterMethod("kvstore.set", r.daemon.handleKVSet)
	gw.RegisterMethod("kvstore.delete", r.daemon.handleKVDelete)
	gw.RegisterMethod("kvstore.list", r.daemon.handleKVList)
	gw.RegisterMethod("kvstore.batch_set", r.daemon.handleKVBatchSet)
	gw.RegisterMethod("kvstore.clear", r.daemon.handleKVClear)

	gw.RegisterMethod("rule.list", r.daemon.handleRuleList)
	gw.RegisterMethod("rule.get", r.daemon.handleRuleGet)
	gw.RegisterMethod("rule.create", r.daemon.handleRuleCreate)
	gw.RegisterMethod("rule.update", r.daemon.handleRuleUpdate)
	gw.RegisterMethod("rule.delete", r.daemon.handleRuleDelete)

	gw.RegisterMethod("filewatch.start", r.daemon.handleFilewatchStart)
	gw.RegisterMethod("filewatch.stop", r.daemon.handleFilewatchStop)
	gw.RegisterMethod("filewatch.remove", r.daemon.handleFilewatchRemove)
	gw.RegisterMethod("filewatch.status", r.daemon.handleFilewatchStatus)

	gw.RegisterMethod("memory.file_states", r.daemon.handleMemoryFileStates)

	gw.RegisterMethod("entity_tags.get", r.daemon.handleEntityTagsGet)
	gw.RegisterMethod("entity_tags.save", r.daemon.handleEntityTagsSave)

	gw.RegisterMethod("terminal.start", r.daemon.handleTerminalStart)
	gw.RegisterMethod("terminal.input", r.daemon.handleTerminalInput)
	gw.RegisterMethod("terminal.resize", r.daemon.handleTerminalResize)
	gw.RegisterMethod("terminal.kill", r.daemon.handleTerminalKill)
	gw.RegisterMethod("terminal.list", r.daemon.handleTerminalList)

	gw.RegisterMethod("translate.rpc", r.daemon.handleTranslate)
}
