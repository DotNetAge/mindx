# Meeting Record Format Specification

## Table of Contents
- [Meeting Record Structure](#meeting-record-structure)
- [Section Details](#section-details)
- [Format Example](#format-example)
- [Output Specifications](#output-specifications)

## Overview
This document defines the standard output format for the Multi-Agent Meeting skill. Meeting records are in Markdown format, clearly presenting meeting basic info, the full discussion process, decision conclusions, and follow-up actions.

## Meeting Record Structure

```
1. Meeting Basic Info
2. Participating Agents
3. Discussion Process
   - Phase 1: Opening Statements
   - Phase 2: Free Discussion
   - Phase 3: Deep Debate
   - Phase 4: Consensus Convergence
   - Phase 5: Decision Generation
4. Decision Conclusion
5. Key Arguments Summary
6. Risk Warnings
7. Follow-up Action Items
```

## Section Details

### 1. Meeting Basic Info
- **Topic**: Concise description of the core issue discussed
- **Time**: Simulated meeting time (optional)
- **Goal**: Type of decision expected
- **Constraints**: Limitations to consider

### 2. Participating Agents
Show all participating agents' role information in a table or list:
- Role name
- Domain
- Characteristics

### 3. Discussion Process
Record the discussion content chronologically by phase. Each entry includes:
- **Speaker**: Agent role name
- **Content**: Full statement
- **Key Points** (optional): Extracted highlights

**Format rules**:
- Use blockquote `>` to mark each statement
- Clearly label the phase and sequence number
- Preserve the original meaning and logic of the discussion

### 4. Decision Conclusion
Concisely state the final decision:
- Clear decision result (yes/no/which option)
- Primary basis for the decision
- Scope and conditions of applicability

### 5. Key Arguments Summary
Summarize the main arguments supporting the decision, categorized by viewpoint:
- **Supporting Arguments**: 3-5 core arguments
- **Opposing Arguments**: Main arguments that were overruled
- **Compromise**: The final compromise approach

### 6. Risk Warnings
Identify potential risks of the decision:
- **Technical Risk**: Risks in technical implementation
- **Business Risk**: Market and business risks
- **Operational Risk**: Execution and operational risks
- **Compliance Risk**: Legal and regulatory risks
- **Mitigation Measures**: Recommended countermeasures for each risk

### 7. Follow-up Action Items
Provide specific follow-up action recommendations:
- **Immediate Actions**: Steps to execute right away
- **Short-term Plan**: Actions for the next 1-3 months
- **Long-term Plan**: Medium-to-long-term direction
- **Responsibility Assignment**: Suggested division of work (if applicable)

## Format Example

```markdown
# Meeting Record: Microservices Architecture Feasibility Assessment

## 1. Meeting Basic Info
- **Topic**: Whether to migrate the existing monolith to a microservices architecture
- **Goal**: Assess feasibility and form a decision recommendation
- **Constraints**: Budget $500K, timeline 6 months, team size 20

## 2. Participating Agents
| Role              | Domain                                    | Characteristics                        |
| ----------------- | ----------------------------------------- | -------------------------------------- |
| Architect         | System architecture, tech selection       | Rational, rigorous, pursues excellence |
| DevOps engineer   | Deployment, CI/CD                         | Pragmatic, risk-aware                  |
| Frontend engineer | UX, frontend performance                  | User-oriented, detail-focused          |
| Project manager   | Project management, resource coordination | Holistic, time-sensitive               |

## 3. Discussion Process

### Phase 1: Opening Statements

> **Chair**: Today we are discussing whether to adopt a microservices architecture. Please share your professional perspectives.

> **Architect**: From an architectural standpoint, the current monolith has reached 5 million lines of code with a deployment cycle of 2 hours. Microservices can bring better scalability and team autonomy, but require addressing inter-service communication and data consistency issues. I lean toward a cautious approach.

> **DevOps engineer**: From an ops perspective, microservices increase deployment complexity and monitoring costs. Our current automated test coverage is only 60%. Splitting into 50+ services would multiply management overhead. I recommend strengthening infrastructure capabilities first.

> **Frontend engineer**: On the UX side, we want sub-second response times and independent feature iteration. Microservices give the frontend team more flexibility, but API fragmentation adds integration complexity. I suggest piloting with core services first.

> **Project manager**: Overall assessment — 20 people migrating a 5M-line monolith in 6 months carries high risk. I recommend an incremental approach: pilot core modules first. Team skill gaps need to be addressed through training.

### Phase 2: Free Discussion
(Detailed discussion content omitted, key points retained)

### Phase 3: Deep Debate
(Detailed debate content omitted, key points retained)

### Phase 4: Consensus Convergence
(Detailed convergence content omitted, key points retained)

### Phase 5: Decision Generation

> **Chair**: Synthesizing everyone's input, we have reached the following consensus...

## 4. Decision Conclusion
**Adopt incremental microservices migration**, specifically:

1. **Phase 1 (3 months)**: Pilot splitting user service and order service to validate feasibility
2. **Phase 2 (6 months)**: Split 5-8 core services, establish microservices infrastructure
3. **Hold full migration**: Pause remaining service splitting until pilot is validated

**Basis**:
- Technical direction is correct, but current team and infrastructure readiness are insufficient
- Incremental approach controls risk — validate first, then expand
- Fits within budget and timeline constraints, avoids over-investment

## 5. Key Arguments Summary

### Supporting Arguments
1. Monolith has reached 5M LOC, hard to maintain, deployment takes 2 hours
2. Microservices improve team autonomy and iteration speed
3. Industry trends support microservices architecture

### Opposing Arguments
1. Test coverage below 60%, microservices increase management complexity
2. Team skills insufficient, requires training investment
3. Data consistency and inter-service communication are technical challenges

### Compromise
Adopt incremental migration — pilot first to validate, avoid big-bang risk

## 6. Risk Warnings

| Risk Type   | Description                               | Severity | Mitigation                     |
| ----------- | ----------------------------------------- | -------- | ------------------------------ |
| Technical   | Inter-service latency affects performance | Medium   | Service mesh + caching         |
| Operational | Team skill gaps delay progress            | High     | Pre-training, external experts |
| Cost        | Infrastructure exceeds budget             | Medium   | Pay-as-you-go cloud services   |

## 7. Follow-up Action Items

### Immediate (this week)
- [ ] Form microservices migration task force
- [ ] Assess current system, define splitting plan
- [ ] Start team training program

### Short-term (3 months)
- [ ] Complete user service microservices migration
- [ ] Set up CI/CD infrastructure
- [ ] Implement monitoring and logging

### Long-term (6-12 months)
- [ ] Split 5-8 core services
- [ ] Establish microservices governance framework
- [ ] Evaluate full-scale rollout
```

## Output Specifications

### Format Requirements
- Output in Markdown format
- Clear heading hierarchy, no more than 4 levels
- Use tables for structured information
- Use blockquotes for discussion content

### Content Quality
- Discussion content should be natural and fit the agent's role
- Discussion process should be logical with clear point/counterpoint
- Decision conclusions should be clear with sufficient supporting arguments
- Risk identification should be comprehensive with feasible mitigation measures
- Action items should be actionable with clear timelines

### Length Control
- Full meeting record recommended no more than 3000 lines
- Discussion section can be expanded or condensed as needed
- Decision conclusions and key arguments should be concise
- Risks and action items should be specific but not redundant
