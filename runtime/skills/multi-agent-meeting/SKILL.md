---
name: multi-agent-meeting
description: >
  When encountering cross-domain collaboration problems, organize a topic-specific meeting, invite relevant experts to participate, collect professional opinions, and generate a structured meeting record.
allowed-tools: subagent bash agent-talk sleep
metadata:
  name_zh: 组织专题会议
  name_zh-tw: 组织专题會議
  description_zh: 当遇到需要跨领域合作的问题时，组织专题会议，邀请相关专家参与，收集专业意见，生成结构化的会议记录。
---

# Multi-Agent Topic Meeting

## When to Use

- When a complex problem requires cross-domain expertise and multi-party collaboration
- When multi-angle analysis and structured expert opinions are needed on a specific topic
- When a structured meeting record with conclusions, key arguments, risks, and action items is required

## Workflow

### 1. Define the Topic and Goal

Extract from user input:

- **Topic**: The core issue to discuss
- **Goal**: The type of decision expected (feasibility assessment, option selection, risk identification, etc.)
- **Constraints**: Limitations to consider (budget, timeline, tech stack, etc.)

### 2. Select Participants

Run `mindx agents list` to see all available agents. **Only invite agents whose domain matches the agenda items.**

### 3. Orchestrate the Meeting

#### Phase 1: Set the Agenda

Confirm the agenda with the user, then assign agenda items to relevant agents:

```
Topic: X
Agenda:
1. [Issue A] → Need input from Agent A and Agent B
2. [Issue B] → Need input from Agent C
3. [Decision] → Synthesize all inputs and make a recommendation
```

#### Phase 2: Collect Initial Input

Use SubAgent to asynchronously gather structured input from each participant in parallel. Each agent must provide:

- **Position**: Stance or recommendation on the topic
- **Rationale**: Reasoning and domain analysis supporting the position
- **Evidence**: Data, facts, or citations supporting the rationale
- **Confidence**: High / Medium / Low — level of certainty in their position
- **Concerns**: Risks, edge cases, or conditions that could invalidate their position
- **Questions for others**: Questions directed at other participants

**Principles**:
  - Independent agenda items can be sent in parallel; dependent items must be serial
  - Each agent only answers questions within its domain
  - Pass topic, relevant agenda, and constraints as context

#### Phase 3: Discussion and Debate

Using the initial input from Phase 2, conduct multi-round debate with the `scripts/debate.py` state machine.

**Create a debate session:**

```bash
# Generate a Session ID (ULID format — used for both AgentTalk session_id and work-dir)
SESSION_ID=$(python scripts/debate.py gen-ulid)

# Initialize debate state in the session directory
python scripts/debate.py init \
  --work-dir "$SESSION_DIR/debate-$SESSION_ID" \
  --topic "<topic>" \
  --agents <AgentA> <AgentB> ... \
  [--max-rounds 3]
```

The `$SESSION_ID` variable is reused in subsequent steps.

**Advance round by round:**

Round 1:
1. `python scripts/debate.py prepare --work-dir "$SESSION_DIR/debate-$SESSION_ID" --round 1`
2. Use AgentTalk to notify each agent — provide the topic and context file path, ask for their position. **Reuse `$SESSION_ID` as the AgentTalk `session_id` parameter for every agent** to maintain conversation continuity
3. Wait for replies
4. For each reply: `python scripts/debate.py record --work-dir "$SESSION_DIR/debate-$SESSION_ID" --round 1 --agent <name> --response '<json>'`
5. `python scripts/debate.py check --work-dir "$SESSION_DIR/debate-$SESSION_ID"`

Round N (N ≥ 2):
1. Confirm all agents replied in the previous round
2. `python scripts/debate.py prepare --work-dir "$SESSION_DIR/debate-$SESSION_ID" --round N`
3. Use AgentTalk to notify each agent for the next round (reuse `$SESSION_ID`)
4. Wait for replies and record
5. `python scripts/debate.py check --work-dir "$SESSION_DIR/debate-$SESSION_ID"`

`check` return values:
- `converged` → debate ends
- `stalled` → escalate to user
- `diverged` → continue to next round
- `max_rounds_reached` → debate ends

After debate ends:

```bash
python scripts/debate.py summary --work-dir "$SESSION_DIR/debate-$SESSION_ID"
```

**Parallel debates**: Each issue group uses an independent `$SESSION_ID` and work-dir (e.g. `debate-$SESSION_ID-issue1`, `debate-$SESSION_ID-issue2`). Interleave rounds across groups.

#### Phase 4: Closure Judgment

Determine closure status for each issue based on debate outcome:

| Status                  | Definition                                     | Action                                                  |
| ----------------------- | ---------------------------------------------- | ------------------------------------------------------- |
| ✅ **Consensus**         | All relevant agents agree                      | Adopt directly, record supporting arguments             |
| ⚠️ **Partial agreement** | General direction agreed, details contested    | Record core consensus + disputed details with positions |
| 🔴 **Deadlock**          | Fundamental disagreement persists after debate | Submit both positions and reasoning to the user         |
| ❓ **Insufficient info** | Unable to form a position                      | Record missing information, suggest further research    |

**Closure rules**:
  - If 2+ agents independently raise the same concern, treat it as a validated risk — do not ignore
  - If an agent changes its position during debate, record the old → new position and the reason
  - If agents from different domains reach the same conclusion via different reasoning paths, mark it as strong corroboration
  - Escalate to the user when:
    (a) A critical-path issue is deadlocked
    (b) The decision involves trade-offs the chair cannot judge
    (c) New information outside existing participants is needed

#### Phase 5: Output Meeting Record

Generate the meeting record following `references/meeting-record-format.md`. **The output must use the same language as the user.**

Include:
- Meeting basic info
- Participant list
- Summary of each agent's input
- Debate process record (position changes per round)
- Decision conclusions and key arguments
- Risks and action items

### 4. Output

Output a structured meeting record containing all of the above.

## Resources

- Record format: [references/meeting-record-format.md](references/meeting-record-format.md)
- Meeting templates: [assets/meeting-templates/](assets/meeting-templates/)
- Debate state machine: [scripts/debate.py](scripts/debate.py)
- View available agents: `mindx agents list`

## Examples

### Example 1: Technical Architecture Decision

- **Goal**: Evaluate whether to adopt microservices
- **Participants**: Architect, DevOps engineer, Frontend engineer, Backend engineer
- **Output**: Professional assessment from each agent, decision recommendation, risk identification, roadmap

### Example 2: Product Pricing Strategy

- **Goal**: Set pricing for a new product
- **Participants**: Market analyst, Product manager, Financial advisor
- **Output**: Competitive pricing analysis, GTM strategy, cost estimation, pricing recommendation

### Example 3: Project Risk Assessment

- **Goal**: Identify and evaluate project risks
- **Participants**: Project manager, relevant engineers, Financial advisor
- **Output**: Risk checklist, impact assessment, mitigation measures, timeline impact

## Notes

- Each agent only speaks to its own domain — do not ask agents to comment outside their expertise
- Independent issues can be debated in parallel by different agent groups for efficiency
- The meeting record is written by the chair synthesizing real agent outputs, not via role-playing
- Disagreement is healthy — different agents have different perspectives; divergence signals a genuine trade-off
- The debate phase (Phase 3) is the core value of the meeting — a meeting without debate is just opinion collection, not a discussed conclusion
- The chair's role is to set the debate framework and judge when convergence is reached, not to relay messages between rounds
- The final decision rests with the user; the meeting outputs are recommendations
