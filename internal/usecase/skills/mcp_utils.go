package skills

import (
	"mindx/internal/entity"
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
	mcp, ok := def.Metadata["mcp"]
	if !ok {
		return nil, false
	}
	mcpMap, ok := mcp.(map[string]interface{})
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
