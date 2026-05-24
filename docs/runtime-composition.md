# Runtime Composition: A Different Architecture for AI Agent Systems

## Every Agent Platform Gets This Wrong

Every AI agent platform asks the same question: "How do you make multiple agents collaborate on a complex task?"

The answers are remarkably consistent:

- **AutoGen**: Code-defined agent conversation graphs
- **CrewAI**: YAML-defined teams and workflows
- **LangGraph**: State machines defined in a graph DSL
- **Semantic Kernel**: C#-coded planners
- **OpenAI Assistants**: Hardcoded function calling chains

Common thread: **orchestration happens at development time, not at runtime.**

The developer must anticipate every possible flow, encode it as a graph, chain, or YAML. The LLM is just an execution node within this predefined structure.

It's like managing a company with an org chart — you draw the departments, reporting lines, and approval flows, then tell employees to follow them. Employees don't need to think because you've already thought of everything. But user needs never follow the org chart you drew.

## A Different Approach: Skills as Atomic Capabilities, LLM as Runtime Orchestrator

What if you flip the architecture?

No predefined flows. Just **capability modules** (we call them Skills), and the LLM decides at runtime how to combine them based on the user's input.

Each Skill is a plain Markdown file that tells the LLM:
- When to use this capability
- What the workflow is
- What scripts are available

No code, no graphs, no YAML. Just text.

The LLM loads it via an ordinary tool call:

```
Skill("project-manager")
```

The Skill's content is appended to the conversation context. Now the LLM knows everything about project management.

Here's the key: **a Skill file can tell the LLM to load another Skill.**

The project-manager Skill contains this line:
> If you need to find or create a specialist agent, load the find-experts skill.

So when the PM needs a designer during planning:

```
Skill("find-experts")
```

Now both Skills' instructions are in the same context. The LLM can freely switch between them.

This isn't code-level composition — it's **text-level composition**. Two Skill documents coexist in the same conversation context, and the LLM decides when to use which.

## Why This Works

Because LLMs are exceptionally good at processing text.

Give it project-manager instructions — it knows how to be a PM. Give it find-experts instructions — it knows how to find experts. Give it both simultaneously — it understands "I'm doing project management, but I can find experts when needed."

This doesn't require framework support. **SkillTool is just a file reader** — it reads any file, any number of times. Composition happens in the LLM's attention mechanism, not in code class hierarchies.

## What This Enables

**1. No predefined orchestration**

Just write Skill documents. Each Skill is independent. Composition is decided by the LLM at runtime.

**2. Natural nesting**

```
User: "Run my Xiaohongshu account"
  → LLM loads project-manager
  → Needs a designer during planning
  → LLM loads find-experts → creates @designer → returns to project-manager
```

The LLM freely switches between Skills. Zero orchestration code.

**3. Semantic boundaries, not code interfaces**

Skills don't need interfaces, shared types, or version alignment. They just need to be written in the same language (natural language). If the LLM can understand both, they can collaborate.

**4. OPC is just one application**

project-manager = PM capability, find-experts = HR capability, AgentTalk = communication tool. Combine them and you get a company.

## Going Deeper: SOP Within SOP

The real power of this pattern is: **a Skill IS a complete SOP (Standard Operating Procedure)**. It's not a tool description, not an API doc — it's a complete instruction manual for how to do an entire job.

When Skills can nest Skills, SOPs can nest SOPs, and real-world business processes can be expressed fully:

```
project-manager (SOP for project management)
  ├── Phase 1: Discovery
  ├── Phase 2: Decompose & Assign
  │   └── If expert needed → load find-experts (SOP for hiring)
  │       ├── List all agents
  │       ├── Create new agent if none fits
  │       └── Assign task
  ├── Phase 3: Track
  │   └── Receive AgentTalk → evaluate → adjust
  └── Phase 4: Report
```

Each SOP is complete, self-contained, independently usable. Nesting isn't code-level import — it's text-level "load into context."

### The Fundamental Difference

Existing platforms follow **tools-as-orchestration**:

```
Startup → load 50 tool definitions into system prompt
        → each tool ~200 tokens
        → fixed ~10K token overhead
        → LLM picks 2-3 out of 50
```

Regardless of what the user asks, all 50 tools occupy token budget. Unused tools waste context.

**Skills-as-orchestration** is fundamentally different:

```
User: "Run my Xiaohongshu account"
  → load project-manager (~2K tokens)
  → Planning: needs a designer
      → load find-experts (~1K tokens)
      → create @designer
      → find-experts naturally slides out of context
  → continue project-manager workflow
```

**Token consumption scales with the actual workflow**, not with the system's total capability surface.

| | Tools-as-orchestration | Skills-as-orchestration |
|---|---|---|
| Loading | All pre-loaded | **Lazy, on-demand** |
| Token cost | Fixed high (~10K+) | **Proportional to workflow** |
| Composition | Function level (params) | **Document level (context co-existence)** |
| Flow expression | DAG / state machine | **SOP nesting SOP** |
| Developer experience | Write code, define interfaces | **Write Markdown, reference other Skills** |
| Runtime flexibility | Predefined, immutable | **LLM decides the path** |

### What This Means

An entire company's SOPs can be written as a collection of Markdown files. Each department owns its Skill. Each Skill can reference other Skills. The LLM, as an "employee", loads what it needs, when it needs it.

Onboarding is no longer reading docs — it's loading the corresponding Skill.

Cross-department collaboration is no longer API calls — it's loading the other department's Skill.

One Skill = one Standard Operating Procedure. A collection of Skills = a company.

## Comparison

| | Existing platforms | This approach |
|---|---|---|
| Orchestration time | Development time (code/YAML/graph) | **Runtime** (LLM decides) |
| Capability unit | Functions/tools/plugins | **Natural language documents** |
| Composition | Code-defined interfaces | **Text coexisting in context** |
| Extension | Code, PR, deploy | **Write a Markdown file** |
| Composition depth | Bounded by API surface | **Bounded by LLM comprehension** |

## Limitations

This depends on context window — every Skill consumes tokens. Loading too many fills the context.

But this is a rapidly improving constraint — from GPT-4's 8K to today's 128K-200K. The trend is clear.

The other question is whether the LLM correctly judges which Skill to load. This depends on Skill description quality. Well-written Skills (like project-manager with clear "when to use" guidance) rarely get mis-selected.

## Conclusion

This architecture isn't someone's invention — it's a natural consequence of capable LLMs. When models are powerful enough, natural language becomes the orchestration language. No DSL, no graphs, no YAML needed.

Skills are capabilities. Conversation context is the bus. The LLM is the runtime orchestrator.

There's no simpler architecture than this.
