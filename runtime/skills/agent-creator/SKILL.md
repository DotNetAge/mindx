---
name: agent-creator
description: >
  Creates new agent definitions tailored to specific roles and tasks.
  Use when the user says "I need an expert in X", "create an agent for Y",
  "I need a specialist in Z", or when another skill (like find-experts)
  determines no existing agent fits the task and a new one must be created.
---

# When to Use This Skill

- The user directly asks to create a new agent or expert
- The user describes a role they need filled ("我需要一个设计师Agent")
- Another skill (e.g. find-experts) determines no existing agent fits
- You need a specialist agent for a task that no existing agent covers

**Do NOT use** when a suitable agent already exists — check with agent.list first.

---

## Workflow

### Step 1: Review Best Practices

Before writing any agent definition, read the best practices guide.
It covers field conventions, selection criteria, and anti-patterns:

> Read `references/agent-best-practices.md` for complete guidance on
> name, role, description, model, and skills fields.

This file is located in the find-experts skill directory:
`<workspace>/skills/find-experts/references/agent-best-practices.md`

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

```bash
python3 scripts/create_agent.py \
    --name "<agent_name>" \
    --role "<agent_role>" \
    --description '<description>' \
    --model "<model_name>" \
    --skills "skill1,skill2"
```

The agent is now registered in the system and ready for delegation.
