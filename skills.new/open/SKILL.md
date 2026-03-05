---
name: open
description: 打开文件技能，打开文件、URL链接或启动应用程序的标准操作程序
version: 1.0.0
author: mindx
tags:
    - open
    - url
    - file
    - app
    - 打开文件
    - 打开链接
    - 打开应用
    - 启动程序
    - system
required_tools:
    - open
---

# Goal

打开文件技能，打开文件、URL链接或启动应用程序

# Triggers

- 用户要求使用 open
- 用户提到"open"
- 用户提到"url"
- 用户提到"file"
- 用户提到"app"
- 用户提到"打开文件"
- 用户提到"打开链接"
- 用户提到"打开应用"
- 用户提到"启动程序"


# SOP

1. 解析用户输入，提取参数
2. 调用 open 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 open
**助手**: 好的，我来帮你处理。

