---
name: clipboard
description: 剪贴板管理技能，读取和写入剪贴板内容，复制粘贴文本的标准操作程序
version: 1.0.0
author: mindx
tags:
    - clipboard
    - copy
    - 剪贴板
    - 复制
    - 粘贴
    - 复制粘贴
    - system
required_tools:
    - clipboard
---

# Goal

剪贴板管理技能，读取和写入剪贴板内容，复制粘贴文本

# Triggers

- 用户要求使用 clipboard
- 用户提到"clipboard"
- 用户提到"copy"
- 用户提到"剪贴板"
- 用户提到"复制"
- 用户提到"粘贴"
- 用户提到"复制粘贴"


# SOP

1. 解析用户输入，提取参数
2. 调用 clipboard 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 clipboard
**助手**: 好的，我来帮你处理。

