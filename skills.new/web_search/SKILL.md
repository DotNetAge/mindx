---
name: web_search
description: 网页搜索技能，使用搜索引擎搜索关键词并返回结果的标准操作程序
version: 1.0.0
author: mindx
tags:
    - search
    - web
    - 网页搜索
    - 搜索引擎
    - general
---

# Goal

网页搜索技能，使用搜索引擎搜索关键词并返回结果

# Triggers

- 用户要求使用 web_search
- 用户提到"search"
- 用户提到"web"
- 用户提到"网页搜索"
- 用户提到"搜索引擎"


# SOP

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

# Examples

**用户**: 请使用 web_search
**助手**: 好的，我来帮你处理。

