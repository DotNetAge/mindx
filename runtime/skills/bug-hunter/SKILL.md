---
name: bug-hunter
description: >
  Expert SOP for locating, isolating and fixing complex bugs.
  Use when the user mentions bugs, errors, crashes, debugging, or fixing issues.
allowed-tools: grep glob bash subagent read
---

# Debug: Session & Bug Analysis

Help the user debug an issue they're encountering in the project or session.

## Instructions
1. **Gather Context**: Use 'grep' and 'read' to locate [ERROR], [WARN], stack traces, and failure patterns in recent logs or code.
2. **Analyze**: Understand the root cause. If the issue is complex, consider launching an independent SubAgent ('subagent') to deeply analyze the specific module. The SubAgent can have a specialized system prompt focused on that module.
3. **Reproduce & Trace**: Identify the exact steps or code paths that lead to the error.
4. **Explain & Suggest**: Explain what you found in plain language, and suggest concrete fixes or next steps. Provide actionable solutions rather than just listing errors.
