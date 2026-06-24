---
name: content-ops
description: >
  Write high-quality, platform-native content for Chinese social media
  platforms (Xiaohongshu/WeChat OA/Douyin/Bilibili/Zhihu/Weibo).
metadata:
  name_zh: 内容创作
  name_zh-tw: 內容創作
  description_zh: 为中国社交平台撰写高质量、平台原生的内容。
  description_zh-tw: 為中國社交平台撰寫高質量、平台原生的內容。
---

## When to Use

- User asks to write content for a Chinese platform (Xiaohongshu/Douyin/WeChat OA/Bilibili/Zhihu/Weibo)
- User needs content adapted from one platform to another
- User wants copy, scripts, or posts for Chinese social media

## How It Works

Before writing anything, load the target platform's reference file from `references/{platform}.md`. It contains:

- What the platform's algorithm rewards
- Title formulas proven for that platform
- Content structure template
- Writing style expectations
- Quality checklist

Each platform has different rules. The same topic written for Xiaohongshu, Zhihu, and Douyin will look completely different — different structure, different opening, different hook, different length.

## Elicitation Pattern — Reason, Recommend, Alternatives

Before writing, establish what is needed. Do not interrogate the user with a list of questions. Instead:

1. Extract whatever context the user already provided
2. Think through what makes sense — which platform, what angle, what format
3. Present your recommended approach with your reasoning
4. Offer 1-2 alternatives for dimensions where reasonable people might disagree
5. Let the user choose, adjust, or propose something new

```
Based on what you've told me:
- Platform: Xiaohongshu makes sense because [your content is visual + tutorial]
- Angle: Step-by-step guide, because Xiaohongshu users save actionable content
- Format: Carousel post with 6 slides

Recommended:
  Title: "3 steps to [result] in [timeframe]"
  Structure: Pain → Method 1 → Method 2 → Method 3 → Before/After → CTA

Alternatives:
  A: Single image + long caption (less production work, lower save rate)
  B: Video version adapted for Douyin if you want reach over saves

Which direction?
```

Iterate if the user has new ideas.

## Cross-Platform Adaptation

When adapting content from one platform to another, load both platform reference files. Understand the structural differences. For example:

- A Zhihu answer (2000 words, pyramid structure, evidence-heavy) → Xiaohongshu note (500 words, emoji-segmented, save-optimized)
- A Douyin video (3s hook, fast cuts, oral script) → Weibo post (140 chars, suspense in main post, payoff in comments)

## Quality Standards — Universal

Every piece of content must pass these checks before delivery:

| Check                 | What to Verify                                                             |
| --------------------- | -------------------------------------------------------------------------- |
| Platform-native       | Does this match the platform's content format and user expectations?       |
| Human voice           | Does this sound like a person wrote it? No boilerplate, no corporate tone. |
| Value density         | Does every sentence earn its place? Would a user regret reading this?      |
| Specificity           | Are there concrete details, examples, or data — not just general advice?   |
| Emotional entry point | Does it make the reader feel something (curious, relieved, seen)?          |

Plus the platform-specific checklist from `references/{platform}.md`.

## Hard Rules

- Load the target platform's `references/{platform}.md` before writing anything.
- All content produced must be written in Chinese.
- Adapt content to the platform's native format — do not produce "one size fits all" text.
- Content must pass platform-specific quality checklist before delivery.
