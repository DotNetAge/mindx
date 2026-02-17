---
name: summarize
description: 内容摘要技能，从URL、视频和文章提取摘要和转录文本
version: 1.0.0
category: general
tags:
  - summarize
  - transcript
  - youtube
  - extract
  - 摘要
  - 总结
  - 转录
  - 提取摘要
os:
  - darwin
  - linux
enabled: true
timeout: 120
command: summarize
requires:
  bins:
    - summarize
homepage: https://summarize.sh
---

# 内容摘要技能

快速摘要 URL、本地文件和 YouTube 链接的 CLI 工具。

## 使用场景

- "这个链接/视频是关于什么的？"
- "摘要这个 URL/文章"
- "转录这个 YouTube/视频"

## 快速开始

```bash
summarize "https://example.com" --model google/gemini-3-flash-preview
summarize "/path/to/file.pdf" --model google/gemini-3-flash-preview
summarize "https://youtu.be/dQw4w9WgXcQ" --youtube auto
```

## YouTube: 摘要 vs 转录

最佳转录（仅 URL）:

```bash
summarize "https://youtu.be/dQw4w9WgXcQ" --youtube auto --extract-only
```

如果用户要求转录但内容很大，先返回紧凑摘要，然后询问要展开哪个部分/时间范围。

## 模型和密钥

设置所选提供商的 API 密钥:

- OpenAI: `OPENAI_API_KEY`
- Anthropic: `ANTHROPIC_API_KEY`
- xAI: `XAI_API_KEY`
- Google: `GEMINI_API_KEY`

默认模型: `google/gemini-3-flash-preview`

## 常用参数

- `--length short|medium|long|xl|xxl|<chars>`
- `--max-output-tokens <count>`
- `--extract-only`（仅 URL）
- `--json`（机器可读）
- `--firecrawl auto|off|always`（后备提取）
- `--youtube auto`（Apify 后备，需要 `APIFY_API_TOKEN`）

## 配置

可选配置文件: `~/.summarize/config.json`

```json
{ "model": "openai/gpt-5.2" }
```

可选服务:

- `FIRECRAWL_API_KEY` 用于被屏蔽的网站
- `APIFY_API_TOKEN` 用于 YouTube 后备
