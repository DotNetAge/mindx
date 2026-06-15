---
name: event-coordinator
description: >
  Plan and execute events from concept to post-event analysis — conferences, webinars, product launches,
  community meetups, hackathons, and virtual/hybrid experiences. Manage venue/logistics, speaker coordination,
  content programming, attendee engagement, sponsor management, and event analytics.
allowed-tools: bash sub-agent collect-results task-create task-update task-get task-list team-create team-list team-get-tasks find-experts app-promotion content-factory
metadata:
  name_zh: 活动策划
  name_zh-tw: 活動策劃
  description_zh: 从概念到复盘的全流程活动策划执行——会议、发布会、线上研讨会、社区聚会、黑客松、混合式体验
  description_zh-tw: 從概念到復盤的全流程活動策劃執行——會議、發布會、線上研討會、社區聚會、黑客松、混合式體驗
---

# Event Coordinator Skill

Plan and execute end-to-end events of any type and scale. This skill covers the full lifecycle from initial concept through post-event analysis, with structured workflows, team roles, and GraphRAG-backed knowledge persistence.

## Trigger Decision

**Use this skill when:**

- User needs to organize an event of any type or size (conference, webinar, product launch, meetup, hackathon, virtual/hybrid)
- User needs to manage speaker coordination, attendee logistics, or vendor relationships
- User wants to run a webinar series or recurring event program
- User needs to coordinate a product launch or announcement event
- User wants to host a community gathering, workshop, or training session
- User asks for event templates, checklists, or run-of-show documents

**Do NOT use this skill when:**

- User only needs to send a single meeting invite or calendar invitation (use calendar/scheduling tools instead)
- The request is purely about venue booking without broader event context
- The scope is limited to drafting a single email about an existing event

---

## Event Type Classification

| Type | Typical Size | Duration | Complexity | Key Success Metrics |
|------|-------------|----------|-----------|---------------------|
| **Webinar** | 50–500 attendees | 1–2 hours | Medium | Registration rate, attendance %, follow-up conversion, Q&A engagement |
| **Product Launch** | 100–2,000 attendees | 2–4 hours | High | Press coverage, sign-ups, social buzz, demo completion rate |
| **Conference** | 200–5,000 attendees | 1–3 days | Very High | NPS score, sponsor satisfaction, revenue vs. budget, session ratings |
| **Community Meetup** | 20–100 attendees | 2–3 hours | Low–Medium | Attendance rate, engagement (questions/networking), repeat attendee rate |
| **Hackathon** | 50–300 participants | 24–48 hours | High | Project submissions count, project quality scores, participant satisfaction |
| **Virtual/Hybrid Event** | Varies widely | Varies | High | Cross-channel engagement metrics, platform stability, attendance by region |

### Example Timelines by Event Type

| Milestone | Webinar (4 wk) | Product Launch (6 wk) | Conference (12 wk) |
|-----------|---------------|----------------------|-------------------|
| Concept & Charter | Week -4 | Week -6 | Week -12 |
| Venue/Platform Locked | Week -3 | Week -5 | Week -10 |
| CFP / Speaker Outreach | Week -3 | Week -5 | Week -9 |
| Landing Page Live | Week -2 | Week -4 | Week -8 |
| Registration Open | Week -2 | Week -3 | Week -6 |
| Sponsor Contracts Signed | N/A | Week -3 | Week -6 |
| Agenda Finalized | Week -1 | Week -2 | Week -3 |
| Marketing Push | Week -1 | Week -2 | Week -2 |
| Speaker Rehearsal | Day -3 | Week -1 | Week -1 |
| Dry Run / Tech Check | Day -1 | Day -3 | Day -3 |
| **Event Day** | Day 0 | Day 0 | Day 0 |
| Post-Event Survey | Day +1 | Day +1 | Day +1 |
| Content Repurposed | Week +1 | Week +2 | Week +4 |
| Final Report | Week +2 | Week +3 | Week +6 |

---

## Domain Knowledge Base

### Language Handling for GraphRAG Queries

> **CRITICAL: This SKILL.md is written in English for token efficiency and global compatibility, but you MUST query GraphRAG in the language matching the stored content.**

**Language detection rule:**
- User speaks/writes in **Chinese** → Query `mindx memory query` and `mindx graph query` in **Chinese first**, then English as fallback
- User speaks/writes in **English** → Query in **English first**, then Chinese as fallback
- When storing via `mindx memory store` → Store in the **same language as the user's current working language**
- Graph node `properties` values → Match the language of stored data
- Cypher string literals → Use the language stored in node properties

**Bilingual query pattern (use when uncertain):**
```bash
# Primary language query
mindx memory query "<搜索词 / search term>"
# Fallback if insufficient results
mindx memory query "<equivalent in other language>"
```

### GraphRAG Dual-Engine Architecture

> **This system has two storage layers that work together — you (the LLM) are the bridge between them via Cypher.**

**Layer 1: Graph — Entity Relationship Index**
- **What it stores:** Nodes (entities) and Edges (relationships)
- **Node structure:** `id`, `type` (entity type from definitions), `name`, `properties` (`description`, `confidence`, + any custom business fields you set)
- **Edge structure:** `type` (relationship), `source`, `target`, `predicate`, `properties`
- **How to write:** `mindx graph upsert-nodes --nodes '[...]'` and `mindx graph upsert-edges --edges '[...]'`
- **How to read:** `mindx graph query --cypher "<your dynamic Cypher>"` or `mindx graph exec --cypher "..."`

**Layer 2: NativeRAG — Semantic Overview Index**
- **What it stores:** Chunks of semantic content with vector embeddings
- **Structure:** content, title, tags, positions, doc_id
- **How to write:** `mindx memory store --content "..." --title "..."`
- **How to read:** `mindx memory query "<search terms>"` (vector similarity search)

**The link:** Both layers share `doc_id` — a Graph node can trace back to its source chunks in NativeRAG, and vice versa.

**Your superpower as LLM:** Humans write fixed hybrid queries. You write **dynamic Cypher** that traverses entity relationships in the Graph, then jumps to NativeRAG for full context via doc_id. This is what makes this architecture flexible.

**When to use which:**
| Need | Command |
|------|---------|
| Find relevant knowledge/documents | `mindx memory query` (semantic search) |
| Store new insights/learnings | `mindx memory store` (vector index) |
| Build structured business state (events, speakers, sponsors, venues) | `mindx graph upsert-nodes/edges` (entity graph) |
| Query relationships between entities | `mindx graph query --cypher "..."` (Cypher traversal) |
| Update business state | `mindx graph exec --cypher "SET ..."` (mutation) |
| Cross-reference: entity → full context | Graph node → get source docs → `memory query` |

Leverage the following mindx capabilities for persistent event intelligence:

### Memory Query (`mindx memory query`)

Use for retrieving past event artifacts and institutional knowledge:

- **Event templates** — past event charters, run-of-show docs, checklists tailored to each event type
- **Vendor databases** — vetted caterers, AV companies, streaming platforms, swag vendors with ratings and contacts
- **Speaker profiles** — past speakers, their topics, presentation quality ratings, availability preferences
- **Venue information** — previously used venues with capacity, cost, layout notes, and contact details
- **Sponsor histories** — past sponsors, tier levels, fulfillment records, renewal likelihood
- **Budget benchmarks** — historical spend breakdowns by event type and size for realistic estimation

### Graph Modeling (`mindx graph`)

Model all event entities as interconnected graph nodes for cross-event intelligence:

```
Event Node
├── properties: name, type, date, location, format, status, budget, attendees_count
├── edges → Speaker Node (role, session, status, fee, travel_req)
├── edges → Sponsor Node (tier, benefits, contract_status, payment_status)
├── edges → Session Node (title, track, time_slot, room/platform, speakers[])
├── edges → Attendee Node (registration_date, ticket_type, engagement_score, feedback)
├── edges → Venue Node (name, capacity, layout, cost, contact, amenities)
├── edges → Vendor Node (type: catering|AV|swag|streaming, rating, cost, notes)
└── edges → Task Node (workstream, owner, deadline, dependency, status)
```

**Post-event enrichment:** After each event, update node properties with actual metrics (vs. projected), attach feedback summaries, and link lessons-learned nodes. This enables pattern recognition across events — e.g., "speakers from domain X consistently drive 30% higher engagement."

---

## Workflow

### Phase 1: Concept & Objectives

**Goal:** Produce a validated Event Charter stored as a GraphRAG node.

**Steps:**

1. **Discovery interview** with stakeholder(s):
   - What is the primary purpose of this event? (thought leadership, lead gen, community building, product announcement, education, celebration)
   - Who is the target audience? (job titles, industries, geography, seniority level)
   - What does success look like? Define 3–5 measurable KPIs with targets
   - What is the budget range? (include contingency of 10–15%)
   - Format preference: virtual, in-person, or hybrid?
   - Hard date constraints or preferred date windows?
   - Must-have nice-to-have features (keynote, networking, workshops, expo hall, etc.)

2. **Feasibility assessment:**
   - Time available vs. recommended timeline for this event type (see table above)
   - Resource availability (internal team bandwidth, budget approval status)
   - Competitive landscape (other events in same space/timeframe)
   - Risk flags (speaker availability, venue seasonality, holiday conflicts)

3. **Output — Event Charter document + GraphRAG node creation:**
   - Store charter as `Event` node with `status: charter_approved`
   - Link to stakeholder nodes, initial budget node, and target-audience persona node

---

### Phase 2: Planning & Logistics

**Goal:** Build a complete execution plan with parallel workstreams, each tracked as tasks with dependencies.

#### Timeline Framework

Work backwards from event date using the appropriate planning horizon:

| Event Type | Planning Horizon | Critical Path Items |
|------------|-----------------|---------------------|
| Webinar | 4 weeks | Platform selection, speaker confirm, landing page |
| Product Launch | 6 weeks | Press list, demo prep, VIP invites, sponsor coordination |
| Conference | 12 weeks | Venue contract, CFP close, speaker travel, sponsorship sales |
| Community Meetup | 3 weeks | Venue, speaker, promotion |
| Hackathon | 8 weeks | Platform/tools setup, judge recruitment, prize sourcing |

#### Parallel Workstreams

Each workstream should be created via `task-create` with clear owners, deadlines, and cross-workstream dependencies.

##### Workstream A: Venue / Platform

| Task | Owner | Deadline | Dependency |
|------|-------|----------|------------|
| Requirements definition (capacity, AV needs, accessibility) | `logistics-manager` | Week X-10 | Charter approved |
| RFP / vendor shortlist (3–5 options) | `logistics-manager` | Week X-9 | Requirements done |
| Site visits / platform demos | `logistics-manager` | Week X-8 | Shortlist ready |
| Contract negotiation & signing | `event-producer` | Week X-7 | Selection made |
| Floor plan / platform config draft | `logistics-manager` | Week X-5 | Contract signed |
| Final logistics spec (catering counts, AV rider) | `logistics-manager` | Week X-2 | Agenda finalized |

**GraphRAG:** Create `Venue` or `Platform` node linked to `Event` node. Store capacity, cost, layout URL, and contact.

##### Workstream B: Speaker & Content Programming

| Task | Owner | Deadline | Dependency |
|------|-------|----------|------------|
| Theme & track design | `content-programmer` | Week X-9 | Charter approved |
| CFP open (if applicable) or speaker wishlist | `content-programmer` | Week X-9 | Theme done |
| Speaker outreach & invitations | `content-programmer` | Week X-8 → ongoing | Wishlist/CFP ready |
| Speaker confirmation & bio/photo collection | `content-programmer` | Week X-6 | Confirmations in |
| Session scheduling (avoid conflicts, balance tracks) | `content-programmer` | Week X-4 | All speakers confirmed |
| Slide deck review & coaching | `content-programmer` | Week X-2 | Schedule locked |
| Rehearsal scheduling | `content-programmer` | Week X-1 | Decks reviewed |

**GraphRAG:** Create `Speaker` nodes with bio, topic expertise, past-performance rating. Create `Session` nodes linked to speakers, tracks, and time slots. Use `find-experts` to identify potential speakers from internal/external networks.

##### Workstream C: Marketing & Registration

| Task | Owner | Deadline | Dependency |
|------|-------|----------|------------|
| Messaging & positioning brief | `marketing-coord` | Week X-8 | Charter approved |
| Landing page build | `marketing-coord` | Week X-7 | Brief complete |
| Email campaign sequences (save-the-date → reminder → day-of) | `marketing-coord` | Week X-6 | Landing page live |
| Social media content calendar | `marketing-coord` | Week X-6 | Brief complete |
| Registration system setup & testing | `marketing-coord` | Week X-5 | Landing page ready |
| PR / press outreach (for launch/conference) | `marketing-coord` | Week X-4 | Key messaging done |
| Attendee comms cadence (confirmation, prep info, day-before) | `marketing-coord` | Ongoing | Registration open |

> **Delegate to `app-promotion` skill** for multi-channel distribution strategy, copywriting for landing pages and emails, social content production, and conversion funnel optimization.

**GraphRAG:** Track registration funnel metrics on `Event` node. Create `Campaign` nodes for each marketing channel linked to registration conversions.

##### Workstream D: Sponsorship

| Task | Owner | Deadline | Dependency |
|------|-------|----------|------------|
| Sponsorship package design (tiers, benefits, pricing) | `sponsor-relations` | Week X-8 | Charter + budget approved |
| Prospect list & outreach sequence | `sponsor-relations` | Week X-7 | Packages designed |
| Contract negotiations | `sponsor-relations` | Week X-5 → ongoing | Interest received |
| Benefit fulfillment planning (logo placement, booth, speaking slot) | `sponsor-relations` | Week X-3 | Contracts signed |
| On-site/virtual sponsor experience design | `sponsor-relations` | Week X-2 | Fulfillment plan done |
| Post-event ROI report | `sponsor-relations` | Week +3 | Event data available |

**GraphRAG:** Create `Sponsor` nodes with tier, industry, contact, contract value, and fulfillment checklist status. Link to `Event` node.

##### Workstream E: Operations

| Task | Owner | Deadline | Dependency |
|------|-------|----------|------------|
| Catering RFP & menu selection | `logistics-manager` | Week X-4 | Headcount estimate |
| AV equipment & crew booking | `logistics-manager` | Week X-4 | Technical rider complete |
| Swag design, production & shipping | `logistics-manager` | Week X-3 | Design approved |
| Registration desk / check-in flow design | `logistics-manager` | Week X-2 | Platform chosen |
| Volunteer / staff scheduling | `logistics-manager` | Week X-2 | Flow designed |
| Emergency & safety plan | `logistics-manager` | Week X-2 | Venue confirmed |

##### Workstream F: Content Production

| Task | Owner | Deadline | Dependency |
|------|-------|----------|------------|
| Event branding & visual identity | `content-programmer` | Week X-7 | Charter approved |
| Agenda document (print/digital) | `content-programmer` | Week X-2 | Schedule finalized |
| Slide templates & speaker guidelines | `content-programmer` | Week X-4 | Branding done |
| Pre-event teaser content (social clips, blog) | `content-programmer` | Week X-3 | Speakers confirmed |
| Day-of graphics & signage | `content-programmer` | Week -1 | Agenda final |

> **Delegate to `content-factory` skill** for producing agenda documents, slide decks, social media assets, email templates, signage designs, and post-event content repurposing packages.

---

### Phase 3: Pre-Event Execution (T-30 to T-0)

Countdown checklist with escalating urgency:

| Countdown | Focus Area | Key Actions | Owner |
|-----------|-----------|-------------|-------|
| **T-30 days** | Foundation lock | Registration fully open; all speakers confirmed (written); sponsor contracts executed; venue deposit paid; marketing campaigns launched | `event-producer` |
| **T-14 days** | Amplification | Major marketing push (paid ads, partner cross-promo, influencer outreach); agenda published and shared; first attendee communication sent (what to expect, how to prepare); waitlist activated if near capacity | `marketing-coord` |
| **T-7 days** | Logistics freeze | Final headcount to caterer/venue; AV final tech spec sent; swag shipment confirmed; volunteer briefing scheduled; speaker travel itineraries confirmed; backup plans documented for top 3 risks | `logistics-manager` |
| **T-3 days** | Rehearsals | Speaker rehearsals (virtual or in-person); run-through of transitions, timings, and technical handoffs; test all streaming/recording equipment; verify backup internet connectivity | `content-programmer` |
| **T-1 day** | Dry run | Full dry run with all key personnel; test registration/check-in flow; verify all slides load correctly; confirm real-time communication channel (Slack/WhatsApp) for day-of team; print run-of-show cards | `event-producer` |
| **T-0 morning** | Go-live | Final venue/platform walk-through; all staff/volunteers briefed and positioned; registration desk open (in-person) or lobby page live (virtual); streaming tested one last time; green room ready | `logistics-manager` |

**Go-Live Checklist (T-0):**

- [ ] Venue open, climate/lighting correct
- [ ] Registration/check-in operational with badge printing
- [ ] AV sound check complete (all mics, screens, lighting)
- [ ] Streaming platform live and stable
- [ ] Green room stocked (water, snacks, WiFi password posted)
- [ ] Run-of-show printed and distributed to all roles
- [ ] Emergency contacts list accessible
- [ ] Photography/videography team checked in
- [ ] Sponsor booths/exhibits set up and verified
- [ ] Wi-Fi network tested (main + backup)
- [ ] Restrooms signed and stocked
- [ ] First aid kit located and communicated

---

### Phase 4: Event Day Execution

#### Run-of-Show (Hour-by-Hour)

Produce a detailed run-of-show document organized by time slot, specifying:

| Time | Activity | Location/Channel | Role Responsible | Notes |
|------|----------|-----------------|------------------|-------|
| 07:00 | Venue opens, staff arrives | Main Hall | `logistics-manager` | Unlock, lights, HVAC |
| 07:30 | AV final check | All Rooms | AV Lead | Mic levels, slide advance |
| 08:00 | Registration opens | Lobby | Reg Desk Team | Badges, swag bags |
| 08:45 | Attendees seated | Main Hall | Floor Managers | Ushers in position |
| 09:00 | Opening remarks + welcome | Main Hall | `event-producer` | House rules, wifi, schedule |
| 09:15 | Keynote 1 | Main Hall | `content-programmer` | Introduce speaker |
| ... | ... | ... | ... | ... |
| 17:00 | Closing remarks + call-to-action | Main Hall | `event-producer` | Survey link, next steps |
| 17:30 | Networking / teardown begins | All Areas | All Teams | Sponsor follow-ups start |

> For virtual events, replace physical locations with stream channels/breakout rooms. Add pre-show lobby (15 min before), intermission entertainment, and post-show networking room.

#### Real-Time Issue Escalation Protocol

| Severity | Examples | Response | Escalate To | SLA |
|----------|----------|----------|-------------|-----|
| **P0 — Showstopper** | Stream down, main speaker no-show, power outage | Immediate workaround + communicate to audience | `event-producer` + venue manager | < 5 min |
| **P1 — Major** | AV glitch, speaker running 15+ min late, registration crash | Activate backup plan, reassign resources | Workstream lead | < 15 min |
| **P2 — Moderate** | Minor mic issue, catering delay, low chat engagement | Local fix, monitor | Role owner | < 30 min |
| **P3 — Minor** | Signage typo, swag shortage, single no-show attendee | Note for post-event, continue | Log only | Next break |

#### Engagement Monitoring

Actively monitor and drive engagement throughout the event:

- **Live chat / Q&A:** Assign a dedicated moderator to surface questions, flag trolls, highlight insightful comments
- **Polling:** Prepare 2–3 polls per session (opinion, knowledge-check, preference) to maintain interactivity
- **Networking:** Facilitate structured networking (speed rounds, topic tables, 1:1 matching for virtual)
- **Social wall:** Display live social posts with event hashtag on main screen
- **Gamification:** Consider points/badges for attendance, questions asked, sessions visited (especially for multi-day)

#### Content Capture

Assign dedicated capture roles to ensure rich post-event material:

- **Photographer:** Key moments (stage shots, crowd reactions, networking, sponsor booths)
- **Videographer:** Full session recordings + highlight clips (best quotes, audience reactions, demos)
- **Note-taker:** Key takeaways, audience questions, notable quotes per session
- **Social recorder:** Real-time tweet/post highlights for amplification during event

---

### Phase 5: Post-Event Analysis

Execute within structured timeframe to maximize data quality and stakeholder value.

#### T+0 to T+24h: Rapid Response

- [ ] Send thank-you email to all attendees with:
  - Session recording links (or "coming soon" note)
  - Feedback survey (keep under 5 minutes, incentivize completion)
  - Slide deck downloads
  - Next-event save-the-date (if applicable)
- [ ] Debrief with core team: what went well, what didn't, immediate action items
- [ ] Collect all content assets (photos, videos, notes) into centralized folder

#### T+24h to T+72h: Data Collection Window ⚠️

> **Critical:** Survey response rates drop sharply after 72 hours. Push reminders at T+48h if needed.

- [ ] Compile quantitative metrics into funnel view:

  ```
  Funnel Metric          Target      Actual     Variance
  ──────────────────────────────────────────────────────
  Impressions            50,000     _______    _______
  Landing Page Visits    10,000     _______    _______
  Registrations          2,000      _______    _______
  Confirmed              1,500      _______    _______
  Checked-in / Joined    1,200      _______    _______
  Avg. Session Attendance 80%       _______    _______
  Survey Responses       400 (33%)  _______    _______
  NPS Score              50         _______    _______
  Follow-up Conversions  200        _______    _______
  ```

- [ ] Qualitative analysis: code open-ended survey responses, identify themes
- [ ] Social sentiment analysis (hashtag mentions, share of voice)

#### T+1 week to T+4 weeks: Content Repurposing Pipeline

> **Delegate to `content-factory` skill** for systematic repurposing:

| Source Asset | Repurposed Output | Channel | Timeline |
|--------------|------------------|---------|----------|
| Full session recordings | 3–5 min highlight clips | YouTube, LinkedIn, Twitter | Week +1 |
| Keynote transcript | Blog post / article | Company blog, Medium | Week +1–2 |
| Panel Q&A | Thread / carousel | LinkedIn, Twitter | Week +2 |
| Best quotes | Quote graphics (5–10) | Instagram, LinkedIn, Twitter | Week +1 |
| Attendee photos | Photo album + recap video | Social, email newsletter | Week +2 |
| Survey insights | Industry report / trends piece | Blog, PR | Week +3–4 |
| Speaker slides (with permission) | SlideShare / educational resource | Website, community | Week +2 |

#### Sponsor ROI Report (T+2–3 weeks)

Deliver to each sponsor a customized report including:

- Deliverables fulfilled vs. contracted (logo placements, booth traffic, speaking slot, email mentions)
- Engagement metrics relevant to their tier (booth scans, session attendance, click-throughs on sponsor links)
- Lead list (if lead-gen was a benefit — with attendee opt-in respected)
- Recommendations for deeper partnership at next event

#### Budget Reconciliation (T+2–3 weeks)

| Category | Budgeted | Actual | Variance | Notes |
|----------|----------|--------|----------|-------|
| Venue / Platform | $______ | $______ | $______ | |
| Catering | $______ | $______ | $______ | |
| AV / Streaming | $______ | $______ | $______ | |
| Speaker fees/travel | $______ | $______ | $______ | |
| Marketing | $______ | $______ | $______ | |
| Swag | $______ | $______ | $______ | |
| Staff/Volunteers | $______ | $______ | $______ | |
| Contingency | $______ | $______ | $______ | |
| **Total** | **$______** | **$______** | **$______** | |

#### Lessons Learned → Knowledge Persistence (T+3–4 weeks)

**This step is critical for continuous improvement.** Store findings in GraphRAG:

1. Create `LessonsLearned` node linked to `Event` node
2. Capture structured learnings:
   - **What worked exceptionally** (replicate next time)
   - **What failed or underperformed** (root cause, fix for next time)
   - **What we'd do differently** (process improvements)
   - **Vendor ratings** (update `Vendor` node properties)
   - **Speaker ratings** (update `Speaker` node properties — presentation quality, responsiveness, audience feedback)
   - **Attendee feedback themes** (link to `Attendee` nodes where actionable)
3. Query `mindx graph` before planning next event of same type to retrieve relevant lessons

---

## Team Composition

| Role | Responsibility | Key Tools/Skills |
|------|---------------|------------------|
| **`event-producer`** | Overall ownership: timeline master, budget owner, stakeholder management, risk owner, final decision authority | Project management, budget tracking, stakeholder comms |
| **`content-programmer`** | Agenda architecture, speaker curation & management, session design, content quality control, rehearsal facilitation | Speaker relations, program design, content review |
| **`marketing-coord`** | Promotion strategy, registration funnel, attendee communications, social media, PR coordination — can delegate execution to `app-promotion` | Marketing automation, copywriting, analytics |
| **`logistics-manager`** | Venue/vendor management, catering, AV, travel coordination, on-site operations, safety/compliance | Vendor negotiation, operations management, problem-solving |
| **`sponsor-relations`** | Sponsor prospecting, package design, outreach, contract management, benefit fulfillment, ROI reporting | Sales, relationship management, reporting |

**Team setup via tools:**
- Use `team-create` to establish the event team with the above roles
- Use `team-get-tasks` to view cross-workstream task assignments
- Use `find-experts` to identify internal or external candidates for specialized roles (e.g., AV specialist, emcee)

---

## Recurring Schedule (Event Series)

For repeating event programs (e.g., monthly webinars, quarterly meetups, annual conferences):

1. **Set up automated cycle** using `mindx schedule add`:
   - Define recurrence pattern (e.g., "first Thursday of every month at 10am PT")
   - Auto-generate template tasks based on event type timeline (see Phase 2 table)
   - Include pre-event and post-event task templates in each cycle

2. **Cycle improvement loop:**
   - After each instance completes, auto-trigger lessons-learned extraction
   - Feed insights into next cycle's template (e.g., "webinar platform B had 40% fewer connection issues than A")
   - Adjust timelines based on actual vs. planned durations from previous cycles

3. **Cross-cycle intelligence:**
   - Track attendee overlap across series instances (who returns? who churns?)
   - Monitor topic fatigue (same speakers/topics = declining engagement?)
   - Build speaker rotation pool from `Speaker` node performance history

---

## Anti-Patterns

Avoid these common pitfalls that derail events:

1. **Skipping the dry run.** Always do a full rehearsal at T-1. The number of issues discovered in dry runs (broken links, wrong slides, timing overruns, AV failures) consistently justifies the time investment. No exceptions.

2. **Underestimating AV/setup time.** Budget 2× your estimated setup time. Something always goes wrong — wrong cable adapter, software version conflict, last-minute layout change. Build buffer into the run-of-show.

3. **Collecting feedback after the 72-hour window.** Attendee memories decay fast. Send the survey within 24 hours, with one reminder at 48 hours. Response rates drop 50%+ after 72 hours.

4. **Treating the run-of-show as static.** Events are dynamic. Assign a "run-of-show owner" who has authority to make real-time adjustments (swap sessions, extend breaks, cut overrunning speakers). Rigidity breaks events; adaptability saves them.

5. **Ignoring the backup plan until you need it.** Document backups for every critical path item (backup speaker, backup streaming platform, backup venue room, backup internet). Review them at T-7, not T-0.

6. **Overloading speakers with admin tasks.** Speakers are volunteers (usually). Minimize their burden: collect bios/photos once, provide clear guidelines early, handle travel/logistics for them, send one consolidated confirmation email. Happy speakers give better talks.

7. **Measuring vanity metrics only.** Registration numbers look great but don't tell the full story. Always measure the full funnel (impressions → registrations → check-ins → engagement → conversion). One high-quality attendee who converts is worth ten who register and don't show.

8. **Not debriefing while memories are fresh.** Schedule the post-event team debrief within 48 hours of event end. Waiting a week means details fade, defensive narratives form, and valuable nuance is lost. Capture raw notes first; polish later.
