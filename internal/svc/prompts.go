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
			"When your existing tools cannot fully address the user's request, check whether one of the following specialized skills covers the domain. If a skill matches, use the Skill tool to load its instructions, which will guide you through domain-specific workflows and expose additional tools.\n\n" +
			"### Side-Effect Rules\n" +
			"- The Skill() tool's return value represents the complete knowledge of that skill. For any given skill name, you may call Skill() at most ONCE per session. After that, all references to that skill's content MUST rely on what is already in memory — do NOT use any tool (Bash, Read, Grep, Glob, WebFetch, etc.) to re-read its files.\n\n" +
			"### Pre-Execution Self-Check\n" +
			"Before calling Bash, Read, or Grep to access file or directory content, you MUST run this check first:\n" +
			"1. Role Gate (P0): is this task within my remit? If NO → delegate per Behavioral Rules, do NOT proceed.\n" +
			"2. If within remit: does the Capacities list above contain a Skill that covers this task?\n" +
			"3. If yes, have I already loaded it via Skill()?\n" +
			"4. Output your reasoning and decision:\n" +
			"   - Reasoning: [remit check result + which Skill was considered]\n" +
			"   - Decision: delegate (if outside remit) | Skill() (if not yet loaded) | proceed with tools (if loaded or no matching Skill)\n"

		footer := "\n### Loading Strategy\n" +
			"- Load skills LAZILY: only when you are about to perform a task that requires it\n" +
			"- To inspect a skill's description before loading, run: `mindx skill list -f \"<skill1>,<skill2>,...\"`\n" +
			"- Each skill persists once loaded — do NOT load the same skill twice\n"

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
			sb.WriteString("  The Python virtual environment used for python script execution.\n")
		}

		if params.SessionID != "" {
			sb.WriteString(fmt.Sprintf("- **Session ID**: %s\n", params.SessionID))
		}
		sb.WriteString(fmt.Sprintf("- **Local Time**: %s\n", time.Now().Format("2006-01-02")))

		return sb.String()
	}
}

// NewSearchStrategyPrompt returns a builder function that overrides the default
// Search Strategy section. It tells the LLM to prioritize knowledge base tools
// (QuickSearch, QuickExplore, FindRelation) over traditional file tools and web search.
func NewSearchStrategyPrompt() func() string {
	return func() string {
		return "## Search Strategy\n\n" +
			"1. For codebase questions, use QuickSearch FIRST before Grep — " +
			"it searches by meaning, not just by filename or text pattern.\n" +
			"2. Also try QuickSearch before WebSearch when the question might be about the user's own project.\n" +
			"3. Fall back to web search (WebSearch) for external topics or when QuickSearch yields nothing.\n" +
			"4. For browsing project structure, use QuickExplore FIRST before LS or Glob — " +
			"it returns the same directory tree with semantic summaries. Fall back to LS/Glob when QuickExplore is unavailable or insufficient.\n" +
			"5. For dependency or relationship questions, use FindRelation — " +
			"it traverses the knowledge graph to show how entities are connected."
	}
}
