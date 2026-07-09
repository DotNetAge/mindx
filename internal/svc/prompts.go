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
			"Capacities lists your role's standard tools. When a task matches a listed skill's domain:\n" +
			"1. Use `mindx skill list -f \"<skill1_name>,<skill2_name>,...\"` to check current description\n" +
			"2. Confirm matching → Load via Skill tool → Execute per instructions\n" +
			"Only skip loading if you have verified no skill in Capacities matches the task.\n" +
			"Forbidden: Starting domain work without first loading the corresponding skill."

		var nameBuilder strings.Builder
		for _, s := range skills {
			nameBuilder.WriteString(fmt.Sprintf("- %s\n", s.Name))
		}

		return header + nameBuilder.String() + footer
	}
}

// NewEnvironmentPrompt returns a builder function that overrides the default
// Environment section in the agent system prompt. It enriches the base info
// (ProjectDir, SessionDir) with SessionID, local time, user preferences dir,
// and Python virtual environment path.
func NewEnvironmentPrompt(userPrefsDir, venvDir string) func(agents.EnvsParams) string {
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

		// if userPrefsDir != "" {
		// 	sb.WriteString(fmt.Sprintf("- **User Prefs**: %s\n", userPrefsDir))
		// 	sb.WriteString("  Application configuration, skills, and agent definitions.\n")
		// }
		if venvDir != "" {
			sb.WriteString(fmt.Sprintf("- **Python Venv Dir**: %s\n", venvDir))
		}

		if params.SessionID != "" {
			sb.WriteString(fmt.Sprintf("- **Session ID**: %s\n", params.SessionID))
		}
		sb.WriteString(fmt.Sprintf("- **Local Time**: %s\n", time.Now().Format("2006-01-02")))

		return sb.String()
	}
}

// NewSearchStrategyPrompt returns a builder function that overrides the default
// Search Strategy section. It tells the LLM to prioritize LocalSearch (semantic
// search) over traditional file tools and web search for codebase questions.
func NewSearchStrategyPrompt() func() string {
	return func() string {
		return "## Search Strategy\n\n" +
			"1. For codebase questions, use LocalSearch (semantic mode) FIRST before Grep/Ls/Read/Glob — " +
			"it searches by meaning, not just by filename or text pattern.\n" +
			"2. Also try LocalSearch before WebSearch when the question might be about the user's own project.\n" +
			"3. Fall back to web search (WebSearch) for external topics or when LocalSearch yields nothing.\n" +
			"4. For browsing project structure, use LocalSearch (tree mode) FIRST before Ls or Glob — " +
			"it returns the same directory tree with semantic summaries. Fall back to Ls/Glob when LocalSearch is unavailable or insufficient."
	}
}
