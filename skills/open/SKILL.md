---
name: open
description: 打开文件技能，打开文件、URL链接或启动应用程序
version: 1.0.0
category: system
tags:
  - open
  - url
  - file
  - app
  - 打开文件
  - 打开链接
  - 打开应用
  - 启动程序
os:
  - darwin
enabled: true
timeout: 30
command: ./open_cli.sh
parameters:
  target:
    type: string
    description: 要打开的目标（文件路径、URL或应用名称）
    required: true
  type:
    type: string
    description: 目标类型："auto"自动检测、"url"链接、"file"文件、"app"应用，默认"auto"
    required: false
  app:
    type: string
    description: 指定使用哪个应用打开
    required: false
---

# 打开技能

使用 macOS 的 `open` 命令打开文件、URL 或应用程序。

## 示例

打开 URL:

```json
{
  "name": "open",
  "parameters": {
    "target": "https://google.com",
    "type": "url"
  }
}
```

打开文件:

```json
{
  "name": "open",
  "parameters": {
    "target": "/Users/xxx/Documents/report.pdf",
    "type": "file"
  }
}
```

打开应用:

```json
{
  "name": "open",
  "parameters": {
    "target": "Safari",
    "type": "app"
  }
}
```

使用指定应用打开:

```json
{
  "name": "open",
  "parameters": {
    "target": "/path/to/document.txt",
    "app": "TextEdit"
  }
}
```
