---
name: deep-search
description: "Searches the internet for a query, reads and ranks the top results with LLM filtering, and returns a cited summary. Use when the user asks to look something up online, research a topic, or needs current information from the web."
version: 1.0.0
category: general
tags:
  - search
  - ai
  - llm
  - summarize
  - deep-search
  - 搜索
  - 网上搜索
  - 精准搜索
  - 智能搜索
  - 查资料
  - 上网查
os:
  - darwin
  - linux
enabled: true
timeout: 180
is_internal: true
guidance: |
  当用户要求"搜一下"、"查一下"、"上网找"、"帮我搜索"时，使用此工具。
  只需提供 terms 参数，例如：{"terms":"Go语言如何安装"}
parameters:
  terms:
    type: string
    description: 搜索查询或问题，例如 "什么是机器学习"、"最新 AI 发展"
    required: true
---

# 深度搜索技能

AI 驱动的深度搜索，结合网页搜索与 LLM 分析，提供综合答案。

## Workflow

1. Search the web and collect up to 20 results for the query.
2. LLM ranks results by relevance and selects the top 3.
3. Fetch and read the full page content of each selected result.
4. LLM synthesizes findings into a summary with source citations.

## 使用方法

```json
{
  "name": "deep-search",
  "parameters": {
    "terms": "什么是机器学习"
  }
}
```

## 输出格式

```json
{
  "summary": "AI 生成的搜索结果综合总结...",
  "page_contents": [
    {
      "url": "https://example.com/article1",
      "title": "文章1标题",
      "content": "第一篇文章的完整内容..."
    }
  ],
  "elapsed": "15.234s",
  "elapsed_ms": 15234
}
```
