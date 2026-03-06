---
name: write_file
description: 文件写入技能，将内容写入到指定文件
version: 1.1.0
category: general
tags:
  - file
  - write
  - 文件写入
  - 保存文件
os:
  - darwin
  - linux
enabled: true
timeout: 30
is_internal: true
parameters:
  filename:
    type: string
    description: 文件名或完整路径，例如 "note.txt"、"/tmp/data.json"
    required: true
  content:
    type: string
    description: 要写入文件的内容
    required: true
  path:
    type: string
    description: 目标目录路径（可选），支持绝对路径或相对于 workspace 的路径
    required: false
---

# 写入文件技能

将内容写入到文件中。是否可写任意路径由「通用设置 -> 读写范围限制」控制。

## 功能特点

- 自动创建不存在的目录
- 支持绝对路径和相对路径
- 相对路径基于 MINDX_WORKSPACE 解析
- 返回写入文件的绝对路径
- 记录写入耗时

## 使用方法

### 写入到 workspace 根目录

```json
{
  "name": "write_file",
  "parameters": {
    "filename": "note.txt",
    "content": "这是要写入的内容"
  }
}
```

### 写入到指定子目录

```json
{
  "name": "write_file",
  "parameters": {
    "filename": "data.json",
    "content": "{\"key\": \"value\"}",
    "path": "documents/notes"
  }
}
```

### 使用绝对路径写入

```json
{
  "name": "write_file",
  "parameters": {
    "filename": "/tmp/output.txt",
    "content": "输出内容"
  }
}
```

### 指定绝对目录路径

```json
{
  "name": "write_file",
  "parameters": {
    "filename": "report.txt",
    "content": "报告内容",
    "path": "/tmp/reports"
  }
}
```

## 输出格式

```json
{
  "file_path": "/Users/ray/.mindx/note.txt",
  "content_length": 20,
  "elapsed_ms": 5
}
```

## 使用场景

- 需要保存笔记或文档时
- 需要导出数据到文件时
- 需要创建配置文件时
- 需要记录日志或结果时

## 路径权限规则

- 关闭限制：默认可写任意路径
- 开启限制：默认仅可写 workspace 路径；可在通用设置中配置额外允许的文件/目录
