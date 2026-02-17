---
name: open_url
description: 网页内容提取技能，打开URL链接并提取网页内容、标题
version: 1.0.0
category: general
tags:
  - browser
  - url
  - scrape
  - content
  - 打开网页
  - 提取内容
  - 网页内容
  - 访问链接
os:
  - darwin
  - linux
enabled: true
timeout: 60
is_internal: true
parameters:
  url:
    type: string
    description: 要打开的 URL 地址，例如 "https://example.com/article"
    required: true
---

# 打开 URL 技能

使用无头 Chrome 浏览器打开 URL 并提取页面内容。

## 功能特点

- 支持 JavaScript 渲染
- 提取页面标题、内容和引用链接
- 支持代理配置
- 内置反检测措施

## 使用方法

```json
{
  "name": "open_url",
  "parameters": {
    "url": "https://example.com/article"
  }
}
```

## 输出格式

```json
{
  "title": "页面标题",
  "url": "https://example.com/article",
  "content": "完整页面文本内容...",
  "refs": [
    "https://example.com/link1",
    "https://example.com/link2"
  ],
  "elapsed_ms": 1500
}
```

## 使用场景

- 需要读取特定网页的内容时
- 需要从页面提取链接时
- 需要抓取需要 JavaScript 渲染的动态内容时
