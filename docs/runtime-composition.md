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
