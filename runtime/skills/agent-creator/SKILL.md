---
name: agent-creator
description: >
  Creates and registers a new agent with a specific role, expertise,
  or capability. Use when you need a specialist in a particular domain
  and no existing agent fits the requirement.
metadata:
  name_zh: 创建智能体
  name_zh-tw: 建立智慧體
  description_zh: 创建和注册具有特定角色、专业知识或能力的新智能体
  description_zh-tw: 建立和註冊具有特定角色、專業知識或能力的新智慧體
---

## When to Use

- User says "I need a XXX expert", "I need someone who knows XXX", "create an agent for XXX"
- A workflow requires a specialist and no existing agent is suitable

**Do NOT use** when a suitable agent already exists.

## Workflow

### 1. Check existing agents

```bash
python3 -c "
import sys; sys.path.insert(0,'scripts')
from rpc_client import rpc_call
import json; print(json.dumps(rpc_call('agent.list'), indent=2))
"
```

### 2. Review field rules

Read `references/agent-best-practices.md` — name format, required fields, anti-patterns.

### 3. List available skills & models

```bash
python3 scripts/list_skills.py
python3 scripts/list_models.py
```

Select only relevant skills — each adds context overhead.

### 4. Create the agent

```bash
python3 scripts/create_agent.py \
    --name "agent-name" \
    --role "Senior Role Title" \
    --description "What the agent does..." \
    --body "Full system prompt..." \
    --model "model-name" \
    --skills "skill1,skill2"
```

The agent is now registered and ready for delegation.
