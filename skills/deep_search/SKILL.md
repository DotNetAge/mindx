---
name: deep_search
description: 互联网深度搜索技能，搜索关键词、阅读并总结网页内容，提供综合答案
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

## 工作原理

1. 搜索网页获取相关结果（最多 20 条）
2. 使用 LLM 筛选出最相关的 3 条结果
3. 打开并阅读筛选出的页面内容
4. 使用 LLM 总结发现并提供参考链接

## 功能特点

- AI 驱动的结果筛选
- 自动内容总结
- 包含参考链接
- 多语言输出

## 使用方法

```json
{
  "name": "deep_search",
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
    },
    {
      "url": "https://example.com/article2",
      "title": "文章2标题",
      "content": "第二篇文章的完整内容..."
    }
  ],
  "elapsed": "15.234s",
  "elapsed_ms": 15234
}
```

## 使用场景

- 需要复杂问题的综合答案时
- 希望 AI 阅读并总结多篇文章时
- 需要带有验证参考链接的答案时
- 研究需要多个来源的主题时
