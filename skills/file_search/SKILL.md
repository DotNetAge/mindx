---
name: file_search
description: 文件搜索技能，在文件系统中搜索文件和目录，支持按文件名搜索、按内容搜索
version: 1.0.0
category: system
tags:
  - file
  - search
  - find
  - directory
  - 文件搜索
  - 查找文件
  - 搜索文件
  - 文件名
  - 文件内容
os:
  - darwin
  - linux
enabled: true
timeout: 30
command: ./file_search_cli.py
parameters:
  action:
    type: string
    description: 搜索类型："files"按文件名搜索、"content"按文件内容搜索、"both"同时搜索文件名和内容
    required: true
  pattern:
    type: string
    description: 搜索模式/关键字
    required: true
  path:
    type: string
    description: 搜索起始路径，默认当前目录（.）
    required: false
---

# 文件搜索技能

在文件系统中搜索文件和目录，支持按文件名搜索、按内容搜索，或两者同时搜索。

## 功能说明

- **files**: 按文件名搜索，查找文件名包含指定关键字的文件和目录
- **content**: 按内容搜索，查找文件内容包含指定关键字的文件
- **both**: 同时按文件名和内容搜索，合并去重结果

## 示例

按文件名搜索:

```json
{
  "name": "file_search",
  "parameters": {
    "action": "files",
    "pattern": "config",
    "path": "/Users"
  }
}
```

按内容搜索:

```json
{
  "name": "file_search",
  "parameters": {
    "action": "content",
    "pattern": "import os",
    "path": "/Users/ray/projects"
  }
}
```

同时搜索文件名和内容:

```json
{
  "name": "file_search",
  "parameters": {
    "action": "both",
    "pattern": "mindx",
    "path": "."
  }
}
```

## 输出格式

```json
{
  "results": [
    "/path/to/file1",
    "/path/to/file2"
  ],
  "count": 2,
  "pattern": "搜索关键字"
}
```
