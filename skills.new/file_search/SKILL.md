---
name: file_search
description: 文件搜索技能，在文件系统中搜索文件和目录，支持按文件名搜索、按内容搜索的标准操作程序
version: 1.0.0
author: mindx
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
    - system
required_tools:
    - file_search
---

# Goal

文件搜索技能，在文件系统中搜索文件和目录，支持按文件名搜索、按内容搜索

# Triggers

- 用户要求使用 file_search
- 用户提到"file"
- 用户提到"search"
- 用户提到"find"
- 用户提到"directory"
- 用户提到"文件搜索"
- 用户提到"查找文件"
- 用户提到"搜索文件"
- 用户提到"文件名"
- 用户提到"文件内容"


# SOP

1. 解析用户输入，提取参数
2. 调用 file_search 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 file_search
**助手**: 好的，我来帮你处理。

