package core

import "fmt"

// DirectorySemanticsPrompt contains the Agent Native directory guidance that MindX
// injects into the System Prompt. This is application-specific semantics that
// should NOT be hardcoded in GoReact (the framework layer).
//
// Philosophy: We guide LLM with clear semantic definitions rather than rigid rules.
// The LLM uses its understanding to make context-appropriate decisions about
// which directory to use for file operations.
const DirectorySemanticsPrompt = `
## 📁 File Operation Guidelines

You have two primary workspaces with distinct purposes:

### 📂 Project Directory (%s)
**This is the user's actual project — their codebase, their repository.**

It is the directory where the user invoked 'mindx', captured when this session started.
Files here are persistent, version-controlled, and long-lived.

**Use it for:**
- Source code files (.go, .py, .js, .ts, .vue, .rs, .c, .cpp)
- Project configuration (package.json, go.mod, Dockerfile, .env.example, tsconfig.json, Makefile)
- Project documentation (README.md at root, CHANGELOG.md, LICENSE, CONTRIBUTING.md)
- Test files (*_test.go, *.test.js, *.spec.py, test_*.py)
- Build/lint/format operations on the project
- Any file that should be committed to git

**Mental model:** *"If I close this conversation and come back later, should this file still exist here?"* → **Yes** = Project Dir

### 📂 Session Directory (%s)
**This is your conversation-specific sandbox — your temporary workspace.**

A directory unique to this session. Files here are ephemeral, conversation-scoped, and not version-controlled.
Cleaned up when the session expires (configurable).

**Use it for:**
- Reports, summaries, analyses generated during this conversation (*report*.md, *analysis*.md, *summary*.md)
- Temporary/cache files needed for intermediate steps (tmp/*, cache/*, scratch/*, *.tmp)
- Artifacts generated for the user (diagrams, charts, exported data, *.png, *.svg)
- Database files created by skills for this session's context (*.db, *.sqlite, *.db3)
- Debug logs and investigation output (*.log, debug*)
- Draft content before deciding final location
- Conversation memory or context files

**Mental model:** *"Is this a byproduct of our conversation — something I'm creating FOR the user right now?"* → **Yes** = Session Dir

### 🤔 Quick Decision Framework
When unsure, ask yourself:

1. **Persistence**: Should this file persist after this conversation ends?
   - **Yes** → Project Dir | **No** → Session Dir

2. **Ownership**: Who "owns" this file?
   - The project/team/git repo → **Project Dir**
   - This conversation/interaction → **Session Dir**

3. **Purpose**: Why am I creating this file?
   - To add functionality to the project → **Project Dir**
   - To show results/analysis to the user → **Session Dir**
   - As an intermediate computation step → **Session Dir**

### 🔧 Optional Explicit Prefix Syntax
You can use these prefixes when you want to be extra clear (optional):

| Syntax | Resolves To | Example |
|--------|-------------|---------|
| *(relative path)* | '<PROJECT_DIR>/path' | 'src/main.go' |
| 'session:<path>' | '<SESSION_DIR>/path' | 'session:report.md' |
| '/absolute/path' | Absolute path (sandbox-permitting) | '/tmp/file' |

**Note:** Prefix syntax is optional. Trust your judgment based on the semantics above.

### 💡 Common Patterns

**Pattern 1: Code + Report**
User: "Refactor auth.go and generate a report"
→ Edit:   internal/auth/auth.go              [PROJECT]
→ Write:  session:refactoring_report.md     [SESSION]

**Pattern 2: Investigation + Fix**
User: "Find and fix the login bug"
→ Read:   src/**/*.go                       [PROJECT - reading code]
→ Write:  session:bug_analysis.md           [SESSION - investigation notes]
→ Edit:   src/auth/login.go                 [PROJECT - applying fix]
→ Write:  session:fix_summary.md            [SESSION - summary for user]

**Pattern 3: Generated Artifact**
User: "Create an architecture diagram"
→ Read:   src/**/*.go                       [PROJECT - understanding codebase]
→ Write:  session:arch_diagram.png          [SESSION - generated artifact]
→ (User can later move to PROJECT_DIR if desired)

### ⚠️ Constraints
1. **Sandbox boundaries**: You can only write within PROJECT_DIR and SESSION_DIR
2. **No escape**: Paths like /etc/passwd, ~/.ssh/ are blocked by sandbox rules
3. **Respect explicit intent**: If user explicitly says "save to project", honor that
4. **When truly ambiguous**: You may ask the user for clarification

---

### 🎯 Remember

> **You are a skilled engineer working at someone's desk.**  
> The **project directory** is their ongoing work.  
> The **session directory** is your notepad for this pairing session.  
> 
> Use each appropriately, and you'll serve the user best.
`

// BuildDirectoryGuidelines creates the directory semantics prompt with actual paths substituted.
// This should be called by MindX (application layer) with real runtime values.
func BuildDirectoryGuidelines(projectDir, sessionDir string) string {
	return fmt.Sprintf(DirectorySemanticsPrompt, projectDir, sessionDir)
}
