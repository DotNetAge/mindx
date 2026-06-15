# AI Capability Configuration

Commands for configuring providers, models, agents, skills, and permission rules.
These define **what the MindX system can do** — which LLMs it can call, which agent
profiles exist, what skills are available, and what tools are allowed/blocked.

## Providers (LLM API Endpoints)

Define where MindX connects to get language model responses.

### Offline Commands (no daemon needed)

| Task | Command | Notes |
|------|---------|-------|
| List all providers | `mindx provider list` | Table: name / title / base URL / API key (masked) / local? |
| List as JSON | `mindx provider list --json` | Machine-readable output |
| Add or update provider | `mindx provider add --name <name> --base-url <url>` | Writes to `providers.yml` |
| Set title | `mindx provider add --name openai --title "OpenAI"` | Human-friendly name |
| Store API key reference | `mindx provider add --api-key OPENAI_API_KEY` | Env var name (not the key itself) |
| Mark as local model | `mindx provider add --local` | For Ollama, LM Studio, etc. |
| Remove provider | `mindx provider rm <name>` | Deletes from config |
| Store actual API key | `mindx provider setkey <provider> <key>` | Goes to macOS Keychain or encrypted file |

### Daemon Commands (requires daemon running)

| Task | Command | Notes |
|------|---------|-------|
| Create via daemon | `mindx provider create --name <n> --title <t> --base-url <url> --api-key <key>` | Daemon-managed storage |
| Update via daemon | `mindx provider update --name <n> --base-url <new-url>` | Update existing |
| Delete via daemon | `mindx provider delete --name <name>` | Daemon-managed deletion |

### Examples

```bash
# Cloud provider (DashScope/Alibaba)
mindx provider add --name dashscope \
  --title "DashScope" \
  --base-url https://dashscope.aliyuncs.com/compatible-mode/v1 \
  --api-key DASHSCOPE_API_KEY
mindx provider setkey dashscope sk-xxxxxxxxxxxx

# Local provider (Ollama)
mindx provider add --name ollama \
  --title "Ollama Local" \
  --base-url http://localhost:11434 \
  --local
```

## Models (LLM Definitions)

Define which specific models are available, tied to a provider.

| Task | Command | Notes |
|------|---------|-------|
| List models | `mindx model list` | Table: name / provider / ctx len / max tokens / func-calling / enabled |
| List as JSON | `mindx model list --json` | Machine-readable |
| Add model | `mindx model add --name <name> --provider <prov>` | Minimum required fields |
| Set context length | `mindx model add ... --context-length 32000` | Default varies by model |
| Set max output tokens | `mindx model add ... --max-tokens 4096` | Response token limit |
| Enable function calling | `mindx model add ... --func-calling` | Required for tool-use models |
| Enable web search | `mindx model add ... --web-searching` | Model has search capability |
| Set generation params | `mindx model add ... --temperature 0.7 --top-p 0.9` | Sampling parameters |
| Disable model | `mindx model add ... --enabled false` | Keep config but don't use |
| Set as default | `mindx model set <model-name>` | Used for new sessions |
| Switch active model | `mindx model switch --name <model>` | **Daemon required** — changes current session |
| Switch with provider | `mindx model switch --name <model> --provider <prov>` | Disambiguate same name across providers |
| Remove model | `mindx model rm <name>` | Delete from config |

### Example
```bash
mindx model add --name qwen-max \
  --provider dashscope \
  --context-length 32000 \
  --max-tokens 4096 \
  --func-calling \
  --temperature 0.7
mindx model set qwen-max
```

## Agents (AI Agent Profiles)

Define agent personas with roles, descriptions, skills, and models.

### Offline Commands

| Task | Command | Notes |
|------|---------|-------|
| List agents | `mindx agent list` | Table: name / role / model / skill count |
| List as JSON | `mindx agent list --json` | Full details via daemon |
| Create new agent | `mindx agent add <name> --role "<role>"` | Creates `.md` file + YAML frontmatter |
| Set description | `mindx agent add ... --description "..."` | What this agent does |
| Assign model | `mindx agent add ... --model qwen-max` | Default model for this agent |
| Assign skills | `mindx agent add ... --skills find-experts,introspect` | Comma-separated skill names |
| Delete agent | `mindx agent rm <name>` | Removes agent file |

### Daemon Commands

| Task | Command | Notes |
|------|---------|-------|
| Get full agent config | `mindx agent get <name>` | Complete YAML frontmatter as JSON |
| Partially update agent | `mindx agent update --agent-name <name>` | Change any field without rewriting |
| Update role | `mindx agent update ... --role "Senior CSM"` | |
| Update description | `mindx agent update ... --description "..."` | |
| Update introduction | `mindx agent update ... --introduction "..."` | Agent's self-intro prompt |
| Change model | `mindx agent update ... --model gpt-4o` | |
| Replace skill set | `mindx agent update ... --skills s1,s2,s3` | Full replacement, not append |
| Exclude tools | `mindx agent update --exclude-tools fs.write,bash` | Block specific tools |
| Score agent performance | `mindx agent score --agent-name <n> --task "<desc>" --score 8 --notes "..."` | 1-10 scale, stored in KVStore |
| Get agent score | (via `mindx kv list --prefix score:`) | Query stored scores |

### Example
```bash
mindx agent add csm-lead \
  --role "Customer Success Lead" \
  --description "Manages enterprise accounts, runs QBRs" \
  --model gpt-4o \
  --skills find-experts,customer-success,content-ops
```

## Skills (Installed Skill Inspectors)

View available skills loaded from `runtime/skills/`.

| Task | Command | Notes |
|------|---------|-------|
| List installed skills | `mindx skill list` | Name / description / allowed-tools |
| List as JSON | `mindx skill list --json` | Includes full frontmatter metadata |
| View skill detail | `mindx skill get <name>` | Shows SKILL.md content |

## Permission Rules (Tool Access Control)

Define which tools agents can use. Rules are injected into the system prompt.

| Task | Command | Notes |
|------|---------|-------|
| List all rules | `mindx rule list` | Current allow/deny/ask rules |
| Get rule details | `mindx rule get --id <tool-name>` | Full rule definition |
| Create new rule | `mindx rule create --id <id> --intro "<instruction>"` | New tool permission |
| Set scope | `mindx rule create ... --scope global\|session\|agent` | When rule applies |
| Set priority | `mindx rule create ... --priority high\|medium\|low` | Conflict resolution |
| Disable rule | `mindx rule create ... --enabled false` | Create but don't activate |
| Update rule | `mindx rule update --id <id> --intro "..."` | Modify existing |
| Toggle enabled | `mindx rule update --id <id> --enabled true/false` | |
| Delete rule | `mindx rule delete --id <id>` | Remove permanently |

## Typical Setup Sequence

```bash
# 1. Provider first
mindx provider add --name dashscope --base-url <url> --api-key DASHSCOPE_KEY
mindx provider setkey dashscope sk-real-key-here

# 2. Then models
mindx model add --name qwen-max --provider dashscope --context-length 32000 --func-calling
mindx model set qwen-max

# 3. Then agents
mindx agent add executive-assistant --role "Executive Assistant" \
  --model qwen-max --skills find-experts,introspect

# 4. Verify
mindx status
mindx provider list
mindx model list
mindx agent list
```
