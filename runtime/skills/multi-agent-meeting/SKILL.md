---
name: multi-agent-meeting
description: >
  Orchestrate cross-functional meetings by calling real agents via SubAgent to gather expert opinions on specific topics.
  The executive assistant chairs the meeting, selects domain-relevant participants, and produces a structured meeting record.
allowed-tools: subagent bash
metadata:
  name_zh: 多智能体会议
  name_zh-tw: 多智能體會議
  description_zh: 编排多个真实 Agent 协作开会的会议技能。
---

# Multi-Agent Meeting Orchestration

## Objective

- **Purpose**: The executive assistant chairs the meeting, selects participants based on the agenda, calls real agents via SubAgent to collect professional opinions, and produces a structured meeting record.
- **Capabilities**:
  - Select relevant agents based on the meeting topic from the live agent registry
  - Call agents in parallel or sequence to gather domain-specific input
  - Identify consensus and disagreement, guide convergence
  - Generate a structured meeting record with conclusions, key arguments, risks, and action items
- **Trigger**: When the user needs multi-perspective analysis, cross-department collaboration, or joint decision-making.

## Procedure

### 1. Define the Meeting Topic and Goal

Extract from user input:

- **Topic**: The core issue to discuss
- **Goal**: The type of decision expected (feasibility assessment, option selection, risk identification, etc.)
- **Constraints**: Limitations to consider (budget, timeline, tech stack, etc.)

### 2. Select Participants

Run `mindx agents list` to see all available agents. **Only invite agents whose domain matches the agenda items.**

### 3. Orchestrate the Meeting

#### Phase 1: Set the Agenda

The executive assistant confirms the agenda with the user, then assigns agenda items to relevant agents:

```
Topic: X
Agenda:
1. [Issue A] → Need input from Agent A and Agent B
2. [Issue B] → Need input from Agent C
3. [Decision] → Synthesize all inputs and make a recommendation
```

#### Phase 2: Collect Input

For each participant, use SubAgent to call the actual agent and get their domain-specific opinion.

- **Method**: Use the SubAgent tool, specify the target agent name, and pass the agenda context
- **Principles**:
  - Independent agenda items can be called in parallel; dependent items must be called sequentially
  - Each agent only answers questions within its domain
  - Pass meeting topic, relevant agenda, and constraints as context

#### Phase 3: Synthesize and Compare

After collecting all inputs, the executive assistant produces a structured summary:

- Consensus points (where agents agree)
- Disagreements (where agents diverge)
- Ambiguities that need user clarification

#### Phase 4: Drive Convergence

Present consensus and disagreement to the user for decision:

- Consensus items are adopted directly
- Disagreements are presented for user judgment, or schedule a follow-up discussion
- Record the user's decision rationale

#### Phase 5: Output Meeting Record

Produce the meeting record following `references/meeting-record-format.md`. **The output must use the same language as the user.**

### 4. Output

Output a meeting record following [references/meeting-record-format.md](references/meeting-record-format.md), including:

- Meeting basic info
- Participant list
- Summary of each agent's input
- Decision conclusions and key arguments
- Risks and action items

## Resources

- Record format: [references/meeting-record-format.md](references/meeting-record-format.md)
- Meeting templates: [assets/meeting-templates/](assets/meeting-templates/)
- View available agents: `mindx agents list`

## Examples

### Example 1: Technical Architecture Decision

- **Purpose**: Evaluate whether to adopt a microservices architecture
- **Chair**: Executive assistant
- **Participants**: Architect, DevOps engineer, Frontend engineer, Backend engineer
- **Output**: Professional assessment from each agent, decision recommendation, risk identification, roadmap

### Example 2: Product Pricing Strategy

- **Purpose**: Set pricing for a new product
- **Chair**: Executive assistant
- **Participants**: Market analyst, Product manager, Financial advisor
- **Output**: Competitive pricing analysis, GTM strategy, cost estimation, pricing recommendation

### Example 3: Project Risk Assessment

- **Purpose**: Identify and evaluate project risks
- **Chair**: Executive assistant
- **Participants**: Project manager, relevant engineers, Financial advisor
- **Output**: Risk checklist, impact assessment, mitigation measures, timeline impact

## Notes

- Each agent only speaks to its own domain — do not ask agents to comment outside their expertise
- Parallel SubAgent calls significantly speed up the meeting when agenda items have no dependencies
- The meeting record is written by the executive assistant synthesizing real agent outputs, not by role-playing
- Disagreement is healthy — different agents have different perspectives; divergence signals a genuine trade-off
- The final decision rests with the user; the meeting outputs are recommendations
