---
name: write_file
description: 文件写入技能，将内容写入到指定文件的标准操作程序
version: 1.0.0
author: mindx
tags:
    - file
    - write
    - 文件写入
    - 保存文件
    - general
---

# Goal

文件写入技能，将内容写入到指定文件

# Triggers

- 用户要求使用 write_file
- 用户提到"file"
- 用户提到"write"
- 用户提到"文件写入"
- 用户提到"保存文件"


# SOP

# 写入文件技能

将内容写入到文件中，所有文件只能写入当前工作区的 documents 子目录。

## 功能特点

- 自动创建不存在的目录
- 支持自定义文件路径
- 返回写入文件的绝对路径
- 记录写入耗时

## 使用方法

### 写入到 documents 根目录

```json
{
  "name": "write_file",
  "parameters": {
    "filename": "note.txt",
    "content": "这是要写入的内容"
  }
}
```

### 写入到 documents 下的子目录

```json
{
  "name": "write_file",
  "parameters": {
    "filename": "data.json",
    "content": "{\"key\": \"value\"}",
    "path": "notes"
  }
}
```

### 写入到 documents 下的多级子目录

```json
{
  "name": "write_file",
  "parameters": {
    "filename": "report.txt",
    "content": "报告内容",
    "path": "reports/2024"
  }
}
```

## 输出格式

```json
{
  "file_path": "/Users/ray/projects/mindx/documents/note.txt",
  "content_length": 20,
  "elapsed_ms": 5
}
```

## 使用场景

- 需要保存笔记或文档时
- 需要导出数据到文件时
- 需要创建配置文件时
- 需要记录日志或结果时

# Examples

**用户**: 请使用 write_file
**助手**: 好的，我来帮你处理。

