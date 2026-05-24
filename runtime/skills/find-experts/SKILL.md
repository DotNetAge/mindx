---
name: find-experts
description: >
  This skill should be used when the user's request falls **outside your professional
  expertise or defined scope of responsibility**, including but not limited to scenarios
  where your currently available tools and skills are insufficient to complete the task.
  In such cases, use this skill to discover and collaborate with specialized experts who
  possess the required domain knowledge and capabilities. 
---

# When to Use This Skill

Trigger this skill **immediately** when any of the following is true:

- The user's request falls **outside your professional expertise or defined scope of responsibility**
- Your currently available **tools cannot** complete the task
- Your currently available **skills cannot** complete the task
- The task requires domain knowledge or capabilities you do not possess

**Do NOT use this skill** for tasks within your own scope — handle those directly.

---

## Workflow

Follow these steps in order. You are the orchestrator; you own the outcome from start to finish.

## Step 1: Discover Available Experts

Run `list_agents.py` to retrieve the full expert roster. Each expert entry contains `name`, `role`, `description`, `skills` — use these to judge fit.

```bash
python scripts/list_agents.py
```

**Output** is a JSON array of all agents with their name, role, description, model, and skills.

**Selection criteria:** Match the expert's `role` and `description` against the task requirements. Consider:

- **Domain alignment** — does the expert's specialty match the task domain?
- **Task scale & difficulty** — simple tasks need only one well-matched expert; complex tasks may require multiple experts across related domains
- **Skills equipped** — does the agent already have relevant skills installed?

For multi-domain tasks, select **multiple experts** and spawn sub-tasks to each via SubAgent based on their respective specializations. Always aim for the **most suitable** candidate(s), not just any available agent.

## Step 2: Spawn SubAgent (or Create Then Spawn)

### Case A: Suitable Expert Found

Use the `SubAgent` tool directly. Compose a **clear, self-contained task brief** that includes:

- Original user context and intent
- Specific deliverable expected
- Constraints, format requirements, or acceptance criteria
- Priority or deadline if applicable

A vague brief produces a vague result. Be specific and unambiguous.

### Case B: No Suitable Expert Exists

Use the **agent-creator** skill to design and create the agent.
Then return here and spawn it using the same briefing guidelines as Case A.

## Step 3: Collect Results

Use `CollectResults` to retrieve the expert's output when execution completes. The `SubAgent` call returns a tracking ID — pass it to `CollectResults`.

Inspect the result against the user's original request:

- Does it actually answer the question or solve the problem?
- Is it complete, correct, and well-structured?
- Are there gaps, errors, or missing edge cases?

Do NOT accept polished-looking output that misses the point.

## Step 4: Score the Expert

Run `rank_task.py` to record the expert's performance score. This builds a cumulative statistical profile over time, enabling data-driven expert selection in future delegations.

```bash
python scripts/rank_task.py \
    --agent-name "<expert_name>" \
    --task "<task_description>" \
    --score <1-10> \
    --notes "<evaluation_notes>"
```

**Scoring rubric:**

| Score    | Meaning                                                          |
| -------- | ---------------------------------------------------------------- |
| **9–10** | Exceptional — exceeded expectations, insightful, well-structured |
| **7–8**  | Good — completed correctly, meets all requirements               |
| **5–6**  | Adequate — mostly done but with minor gaps or errors             |
| **3–4**  | Below par — significant gaps, needs rework                       |
| **1–2**  | Poor — misses the point entirely, unusable                       |

Be honest. An accurate 6 today is more valuable than an inflated 10 — scores corrupt the statistical profile and defeat the purpose of building reliable performance data.

The score is persisted in the agent's YAML frontmatter under `performance.scores` as `{task, score, notes, timestamp}` records.

## Step 5: Report to User

Present the verified result honestly:

- **Fully resolved** → Deliver the result with a concise summary of what was done and which expert(s) contributed
- **Partial / issues found** → Explain gaps clearly and propose next steps (retry with clarification, try a different expert, or supplement yourself)

You remain the single point of accountability. The user delegated to *you* — you delegated to the expert, but *you* own the outcome.

---

# Multi-Expert Coordination

For large-scale tasks requiring parallel work across domains:

1. Use `TeamCreate` to form a team of selected experts
2. Use `TaskCreate` to assign specific sub-tasks to each member based on their specialization
3. Use `CollectResults` to gather all outputs
4. Synthesize results into a coherent deliverable
5. Score each expert individually via `rank_task.py`

---

# Anti-Patterns

- **Do NOT use SubAgent** for tasks within your own domain of expertise — handle them directly
- **Do NOT use SubAgent** for trivial tasks you can complete faster yourself
- **Do NOT create** an agent without first running `list_agents.py` to confirm no suitable one exists
- **Do NOT use SubAgent** to bounce clarification back upward — the original delegator handles that directly
- **Do NOT accept** unverified output — always inspect before reporting to the user

---

## References

- **`references/agent-best-practices.md`** — Complete guide for writing agent definitions:
  field conventions, selection criteria, model/skill matching, anti-patterns, and creation
  checklist. Shared with the agent-creator skill.
