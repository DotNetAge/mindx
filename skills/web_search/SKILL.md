---
name: web_search
description: 网页搜索技能，使用搜索引擎搜索关键词并返回结果
version: 1.0.0
category: general
tags:
  - search
  - web
  - 网页搜索
  - 搜索引擎
os:
  - darwin
  - linux
enabled: true
timeout: 60
is_internal: true
parameters:
  terms:
    type: string
    description: 搜索关键词，例如 "Go 语言教程"、"最新科技新闻"
    required: true
---

# 网页搜索技能

使用 DuckDuckGo 搜索引擎进行网页搜索，返回搜索结果列表。

## 功能特点

- 使用 DuckDuckGo 搜索引擎
- 支持 JavaScript 渲染的动态页面
- 返回标题、链接和描述
- 内置反检测措施

## 使用方法

```json
{
  "name": "web_search",
  "parameters": {
    "terms": "Go 语言教程"
  }
}
```

## 输出格式

```json
{
  "count": 10,
  "elapsed_ms": 2500,
  "results": [
    {
      "title": "Go 语言官方教程",
      "link": "https://golang.org/doc/tutorial",
      "description": "Go 语言官方入门教程..."
    }
  ]
}
```

## 使用场景

- 需要搜索网页内容时
- 需要获取最新信息时
- 需要查找参考资料时
