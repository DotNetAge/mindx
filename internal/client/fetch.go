package client

import (
	"encoding/json"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/gort/pkg/gateway"
)

func fetchAgents(client *gateway.Client) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.SendCommand("agents", "")
		if err != nil {
			return errMsg(fmt.Errorf("获取 Agent 列表失败: %w", err))
		}
		var agents []agentInfo

		var opts []map[string]interface{}
		if err := json.Unmarshal([]byte(resp), &opts); err == nil {
			for _, o := range opts {
				name, _ := o["label"].(string)
				if name == "" {
					name, _ = o["value"].(string)
				}
				role, _ := o["role"].(string)
				desc, _ := o["desc"].(string)
				model, _ := o["model"].(string)
				isMaster, _ := o["master"].(string)
				if isMaster == "" {
					if b, ok := o["master"].(bool); ok && b {
						isMaster = "true"
					}
				}
				if name != "" {
					agents = append(agents, agentInfo{
						name:        name,
						role:        role,
						description: desc,
						model:       model,
						master:      isMaster == "true",
					})
				}
			}
		}

		seen := make(map[string]bool)
		unique := make([]agentInfo, 0, len(agents))
		var masterName string
		for _, a := range agents {
			if !seen[a.name] {
				seen[a.name] = true
				unique = append(unique, a)
				if a.master {
					masterName = a.name
				}
			}
		}
		return agentsFetchedMsg{agents: unique, masterName: masterName}
	}
}

func fetchCommands(client *gateway.Client) tea.Cmd {
	return func() tea.Msg {
		cmds, err := client.GetCommands()
		if err != nil {
			return errMsg(fmt.Errorf("获取命令列表失败: %w", err))
		}
		return commandsFetchedMsg{commands: cmds}
	}
}
