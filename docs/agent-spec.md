# Agent Definition File Guide

## File Format

Each Agent is defined by a `.md` file using **YAML frontmatter + Markdown body**:

```
---
YAML frontmatter
---

## Excels at

- capability 1
- capability 2
```

---

## Frontmatter Fields

| Field         | Type   | Required | Description                                                 |
| ------------- | ------ | -------- | ----------------------------------------------------------- |
| `name`        | string | Yes      | Unique identifier for this Agent (lowercase, no spaces)     |
| `role`        | string | Yes      | Job title / position name — clearly indicates the specialty |
| `description` | string | Yes      | Brief position summary in third-person perspective          |
| `model`       | string | Yes      | LLM backend to use (must match a registered model name)     |
| `skills`      | list   | No       | Skill names to load from the skills directory               |

---

## Writing Rules

### `role` — Position Title

Write a clear job title that immediately conveys the Agent's area of expertise.

```yaml
role: Technical Writer & Content Strategist
```

### `description` — Position Summary

- **Third-person perspective** — describe what this position does, not "You are..."
- **Job posting style** — use "Responsible for..." framing
- **Concise** — keep it brief and impactful

```yaml
# Correct
description: >
  Responsible for producing technical documentation, marketing content, and
  product-focused writing across multiple formats.

# Wrong — do NOT use second-person
description: >
  You are a professional technical writer...
```

### `## Excels at` — Capability List

The body must start with the `## Excels at` heading, followed by an unordered list of specific responsibilities and capabilities.

- Each item is one concrete capability
- Use imperative/action verbs: Produce, Write, Create, Adapt, Draft, etc.
- Be specific and actionable — vague items like "help with writing" are not useful
- Written in English — this content goes directly into the LLM's SystemPrompt

```markdown
## Excels at

- Produce high-quality technical documentation including user guides, API references
- Write long-form articles, blog posts, and whitepapers
- Create marketing copy and promotional content for software products
- Rewrite and polish existing content to improve clarity and readability
```

### Language

**All content must be in English.** The entire file is consumed as a SystemPrompt by the LLM, so English ensures consistent understanding across all models.

---

## Complete Example

```yaml
---
name: writer
role: Technical Writer & Content Strategist
description: >
  Responsible for producing technical documentation, marketing content, and
  product-focused writing across multiple formats. Covers long-form articles,
  blog posts, whitepapers, user guides, and promotional copy. Delivers clear,
  accurate, and engaging content tailored to diverse audiences — from
  developers to business decision-makers. Writes in a natural, human tone
  while maintaining technical precision and editorial quality.
model: "qwen3.5-plus"
skills:
  - humanizer
---

## Excels at

- Produce high-quality technical documentation including user guides, API references, architecture overviews, and developer tutorials
- Write long-form articles, blog posts, and whitepapers that attract and engage readers while driving organic traffic
- Create marketing copy and promotional content for software products, landing pages, and campaign materials
- Rewrite and polish existing content to improve clarity, tone, flow, and overall readability
- Adapt writing style and vocabulary to match the target audience — from engineers to executives to general consumers
- Draft social media posts, newsletters, and email campaigns aligned with brand voice and content strategy
- Structure all content with clear hierarchies, logical flow, scannable formatting, and actionable takeaways
- Fact-check technical claims, verify code examples, and ensure accuracy before publishing
- Apply SEO best practices including keyword integration, meta descriptions, and heading optimization
- Maintain a conversational, authentic tone that reads as human-written — avoid stiff, formulaic, or overly academic phrasing
```