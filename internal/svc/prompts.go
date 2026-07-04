package svc

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DotNetAge/goharness/agents"
	"github.com/DotNetAge/goharness/skill"
)

// NewSkillsPrompt returns a builder function that overrides the default
// buildSkillsCatalog prompt. It keeps the same header and Loading Strategy
// footer, but lists only skill names instead of full descriptions.
//
// The Loading Strategy footer includes a tip on using "mindx skill list -f"
// to view detailed descriptions for specific skills.
func NewSkillsPrompt() func([]*skill.Skill) string {
	return func(skills []*skill.Skill) string {
		if len(skills) == 0 {
			return ""
		}

		header := "## Capacities (Available Skills)\n" +
			"When your existing tools cannot fully address the user's request, check whether one of the following specialized skills covers the domain. If a skill matches, use the Skill tool to load its instructions, which will guide you through domain-specific workflows and expose additional tools.\n"

		footer := "\n### Loading Strategy\n" +
			"- Load skills LAZILY: only when you're about to perform a task that requires it\n" +
			"- Each skill persists once loaded into conversation context \u2014 do NOT reload already-loaded skills\n" +
			"- To view detailed descriptions of a skill, use `mindx skill list -f \"<skill_name>,<skill_name>,...\"`\n"

		var nameBuilder strings.Builder
		for _, s := range skills {
			nameBuilder.WriteString(fmt.Sprintf("- %s\n", s.Name))
		}

		return header + nameBuilder.String() + footer
	}
}

// NewEnvironmentPrompt returns a builder function that overrides the default
// Environment section in the agent system prompt. It enriches the base info
// (ProjectDir, SessionDir) with SessionID and a local timestamp.
func NewEnvironmentPrompt() func(agents.EnvsParams) string {
	return func(params agents.EnvsParams) string {
		var sb strings.Builder
		sb.WriteString("## Environment\n")

		// ProjectDir: the user's working directory (persistent, the source of truth).
		projectDir := params.ProjectDir
		if projectDir == "" {
			projectDir, _ = os.Getwd()
		}
		sb.WriteString(fmt.Sprintf("- **Project Dir**: %s\n", projectDir))
		sb.WriteString("  The user's working directory — files here persist permanently across sessions.\n")
		sb.WriteString("  Modify existing user files and create long-lived outputs here.\n")

		// SessionDir: an ephemeral workspace scoped to the current conversation.
		sessionDir := params.SessionDir
		if sessionDir == "" {
			sessionDir = "(not set — scratch files will not persist)"
		}
		sb.WriteString(fmt.Sprintf("- **Session Dir**: %s\n", sessionDir))
		sb.WriteString("  A temporary workspace for the current conversation.\n")
		sb.WriteString("  Contents are deleted when the conversation ends — do NOT put important work here.\n")

		if params.SessionID != "" {
			sb.WriteString(fmt.Sprintf("- **Session ID**: %s\n", params.SessionID))
		}
		sb.WriteString(fmt.Sprintf("- **Local Time**: %s\n", time.Now().Format("2006-01-02 15:04:05 MST")))

		return sb.String()
	}
}
