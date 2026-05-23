# One Person Company: Running an Entire Business with AI Agents

## The Insight

Every AI agent platform I've seen treats agents as **executors**: you give them a task, they do it, done. Even the most sophisticated multi-agent systems are just fancy ways to split a task into sub-tasks, hand them out, and collect results.

That's not how a company works.

A company has:
- **Employees** with different roles and ongoing responsibilities
- **Schedules** — work that repeats daily, weekly, monthly
- **Meetings** — people talk to each other, check status, give feedback
- **Managers** who plan, track, adjust, and report
- **Context** — conversations persist across days and weeks

What if you could build this with AI agents, not by writing more code, but by **connecting existing primitives** in a novel way?

## The Primitives

We already had all the pieces. We just didn't see the pattern.

### 1. The Agent (Reactor Loop)

Every agent runs a Think-Act-Observe loop. It receives a message, thinks about it, decides what to do, executes tools, observes results, and replies. This is the standard LLM agent pattern — nothing new here.

### 2. The Session

Sessions are where conversation history lives. When an agent responds to a message, the exchange is saved. Next time the same session ID is used, the agent picks up where it left off.

Again, standard. Every chat system has this.

### 3. The Scheduler

The Scheduler's job is trivial: at a given cron time, send a message to an agent in a session. That's it. The Scheduler doesn't know it's "managing a project" or "running a business". It just sends messages on schedule.

### 4. AgentTalk — The Missing Piece

(This was the only thing we had to build.)

Agents can talk to each other. Not through a workflow engine, not through a DAG — just a tool call:

```
AgentTalk(agent_name="@writer", session_id="proj-42", message="How's the report going?")
```

The target agent wakes up, sees the message in context, replies. The caller gets the reply back. Same session next time means continuity.

## The Combination — How a Company Emerges

None of these primitives knows about the larger system. Each one does something trivial. But combined, they produce company-like behavior.

### Step 1: The Manager Plans

A user tells the PM agent: "I want to run a Xiaohongshu (Little Red Book) account."

The PM talks to the user, extracts measurable goals, decomposes them:

```
Project: little-red-book
Goal: Grow followers from 0 to 10,000 in 3 months
Tasks:
  - @writer: Write 3 posts per week (Mon/Wed/Fri at 10 AM)
  - @designer: Create 3 images per week (Mon/Wed/Fri at 9 AM)
  - @analyst: Weekly performance report (Saturday at 6 PM)
```

Each task is registered with the Scheduler: who, when, what.

### Step 2: The Scheduler Wakes People Up

Monday 9 AM. The Scheduler sends a message to @designer:

> "Create 3 images for this week's posts. Topic: AI tools for productivity."

@designer doesn't know it was "scheduled". It just sees a message, does its job, produces images.

But the task prompt includes one extra instruction:

> "After completing, use AgentTalk to report to project-manager in session 'little-red-book'."

### Step 3: Agents Report Back

After finishing, @designer calls:

```
AgentTalk("project-manager", "little-red-book", "3 images created for this week. Topics covered: AI writing, AI coding, AI design.")
```

The PM receives this, acknowledges it, and moves on.

Monday 10 AM. @writer finishes its post, reports back.

### Step 4: The Manager Tracks and Adjusts

Over the week, the PM receives reports from all agents. It compiles them into a weekly summary and proactively sends it to the user — before the user asks.

If @writer reports "struggling with topic ideas", the PM can reply:

```
AgentTalk("@writer", "little-red-book", "Try writing about your experience with the new AI design tool. Personal stories perform better.")
```

No polling. No dashboards. Just conversation.

### The Result

From the user's perspective: they had a vague idea ("run a social media account"), and now there's a team of agents working on it autonomously, reporting back, and the user just receives daily briefings.

From the system's perspective: nothing special happened. A scheduler sent messages. Agents replied. Sessions stored text. A tool routed messages between agents.

The magic is not in any single component. The magic is in the **combination**.

## Why This Hasn't Been Done

Most AI platforms approach multi-agent from a **computation** mindset: "how do I distribute a large task across many LLM calls?" The result is workflow DAGs, task queues, and orchestration pipelines.

This approach is from a **management** mindset: "how do I run an organization where agents have ongoing responsibilities?"

The difference is subtle but profound:

| | Computation mindset | Management mindset |
|---|---|---|
| Unit of work | Task | **Ongoing responsibility** |
| Agent lifecycle | Created per task, destroyed after | **Persistent**, responds to messages |
| Communication | Data passing via workflow | **Conversation** via shared sessions |
| Coordination | Orchestrator-controlled DAG | **Autonomous** with guidance |
| State | Immutable workflow state | **Session history**, accumulated context |

Sessions are the key insight. In a computation system, state is a problem to be managed. In a management system, **conversation history IS the state** — and every LLM platform already has it.

## The Code

The entire AgentTalk implementation is ~100 lines of Go:

```go
type AgentTalkFunc func(ctx context.Context, to, sessionID, message string) (string, error)

type AgentTalkTool struct {
    talk AgentTalkFunc
}

func (t *AgentTalkTool) Execute(ctx context.Context, params map[string]any) (any, error) {
    to := params["agent_name"].(string)
    sessionID := params["session_id"].(string)
    message := params["message"].(string)
    
    reply, err := t.talk(ctx, to, sessionID, message)
    return map[string]any{
        "reply":      reply,
        "agent_name": to,
        "session_id": sessionID,
    }, nil
}
```

The default implementation clones the caller agent's config and routes through Agent.Ask() — the same path used for every user message. No new infrastructure needed.

## What It Enables

- **One Person Company (OPC)**: A single human + a PM agent + N specialized agents = an entire organization
- **Autonomous operations**: The PM plans once, agents execute and report back indefinitely
- **Natural coordination**: Agents talk to each other like colleagues, not like microservices
- **Zero dashboards**: The PM tells you what's happening. You never have to ask.

The code is open source at [github.com/DotNetAge/mindx](https://github.com/DotNetAge/mindx). The AgentTalk tool is in the goreact framework at [github.com/DotNetAge/goreact](https://github.com/DotNetAge/goreact).
