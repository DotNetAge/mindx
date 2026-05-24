---
name: agent-creator
description: >
  Creates and registers a new agent with a specific role, expertise,
  or capability. Use when you need a specialist in a particular domain
  and no existing agent fits the requirement.
---

# When to Use This Skill

- The user says "I need a XXX expert", "I need someone who knows XXX",
  "I need a specialist in XXX", "create an agent for XXX"
- The user needs a professional capability no existing agent has
- You're executing a workflow that requires a specialist, and no existing
  agent is suitable

**Do NOT use** when a suitable agent already exists — check existing agents first.

---

## Workflow

### Step 1: Review Best Practices

Read `references/agent-best-practices.md` for guidance on name, role,
description, model, and skills fields before defining the agent.

### Step 2: List Available Skills

See what skills are installable:

```bash
python3 scripts/list_skills.py
```

Select skills that match the expert's domain. Only assign relevant skills —
over-equipping inflates context overhead.

### Step 3: List Available Models

See what models are available:

```bash
python3 scripts/list_models.py
```

Choose the model best suited to the task type.

### Step 4: Create the Agent

Craft a clear system prompt (body) that defines the agent's identity and
responsibilities. Use the agent's role and description as a starting point.

```bash
python3 scripts/create_agent.py \
    --name "<agent_name>" \
    --role "<agent_role>" \
    --description "<description>" \
    --body "<system_prompt>" \
    --model "<model_name>" \
    --skills "skill1,skill2"
```

The agent is now registered in the system and ready for delegation.
