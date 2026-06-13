---
name: bug-hunter
description: >
  Expert SOP for locating, isolating and fixing complex bugs.
  Use when the user mentions bugs, errors, crashes, debugging, or fixing issues.
allowed-tools: grep glob bash subagent read
metadata:
  name_zh: Bug 猎手
  name_zh-tw: Bug 獵手
  description_zh: 定位、隔离和修复复杂 Bug 的专家标准流程
  description_zh-tw: 定位、隔離和修復複雜 Bug 的專家標準流程
---

# Debug: Session & Bug Analysis

Help the user debug an issue they're encountering in the project or session.

## Instructions
1. **Gather Context**: Use 'grep' and 'read' to locate [ERROR], [WARN], stack traces, and failure patterns in recent logs or code.
2. **Analyze**: Understand the root cause. If the issue is complex, consider launching an independent SubAgent ('subagent') to deeply analyze the specific module. The SubAgent can have a specialized system prompt focused on that module.
3. **Reproduce & Trace**: Identify the exact steps or code paths that lead to the error.
4. **Explain & Suggest**: Explain what you found in plain language, and suggest concrete fixes or next steps. Provide actionable solutions rather than just listing errors.
