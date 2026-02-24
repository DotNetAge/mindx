package skills

import (
	"encoding/json"
	"fmt"
	"mindx/internal/entity"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPSkillMetadata struct {
	Server string `json:"server"`
	Tool   string `json:"tool"`
}

func IsMCPSkill(def *entity.SkillDef) bool {
	if def == nil || def.Metadata == nil {
		return false
	}
	mcp, ok := def.Metadata["mcp"]
	if !ok {
		return false
	}
	mcpMap, ok := mcp.(map[string]interface{})
	if !ok {
		return false
	}
	_, hasServer := mcpMap["server"]
	_, hasTool := mcpMap["tool"]
	return hasServer && hasTool
}

func GetMCPSkillMetadata(def *entity.SkillDef) (*MCPSkillMetadata, bool) {
	if def == nil || def.Metadata == nil {
		return nil, false
	}
	mcpRaw, ok := def.Metadata["mcp"]
	if !ok {
		return nil, false
	}
	mcpMap, ok := mcpRaw.(map[string]interface{})
	if !ok {
		return nil, false
	}
	server, _ := mcpMap["server"].(string)
	tool, _ := mcpMap["tool"].(string)
	if server == "" || tool == "" {
		return nil, false
	}
	return &MCPSkillMetadata{
		Server: server,
		Tool:   tool,
	}, true
}

// MCPToolToSkillDef 将 MCP Tool 转换为 MindX SkillDef
func MCPToolToSkillDef(serverName string, tool *mcp.Tool) *entity.SkillDef {
	skillName := fmt.Sprintf("mcp_%s_%s", serverName, tool.Name)

	params := make(map[string]entity.ParameterDef)
	if tool.InputSchema != nil {
		if schemaMap, ok := tool.InputSchema.(map[string]any); ok {
			extractParameters(schemaMap, params)
		} else {
			// InputSchema 可能是 json.RawMessage 或其他类型，尝试 JSON 转换
			data, err := json.Marshal(tool.InputSchema)
			if err == nil {
				var schemaMap map[string]any
				if json.Unmarshal(data, &schemaMap) == nil {
					extractParameters(schemaMap, params)
				}
			}
		}
	}

	return &entity.SkillDef{
		Name:        skillName,
		Description: tool.Description,
		Category:    "mcp",
		Tags:        []string{"mcp", serverName},
		Enabled:     true,
		Timeout:     30,
		Parameters:  params,
		Metadata: map[string]interface{}{
			"mcp": map[string]interface{}{
				"server": serverName,
				"tool":   tool.Name,
			},
		},
	}
}

// extractParameters 从 JSON Schema 提取参数定义
func extractParameters(schema map[string]any, params map[string]entity.ParameterDef) {
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}

	requiredSet := make(map[string]bool)
	if required, ok := schema["required"].([]any); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				requiredSet[s] = true
			}
		}
	}

	for name, propRaw := range props {
		prop, ok := propRaw.(map[string]any)
		if !ok {
			continue
		}
		paramType, _ := prop["type"].(string)
		if paramType == "" {
			paramType = "string"
		}
		desc, _ := prop["description"].(string)
		params[name] = entity.ParameterDef{
			Type:        paramType,
			Description: desc,
			Required:    requiredSet[name],
		}
	}
}
