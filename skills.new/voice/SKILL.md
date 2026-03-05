---
name: voice
description: 语音播报技能，使用系统语音朗读文本内容的标准操作程序
version: 1.0.0
author: mindx
tags:
    - voice
    - notification
    - 语音
    - 朗读
    - 播报
    - 说话
    - general
required_tools:
    - voice
---

# Goal

语音播报技能，使用系统语音朗读文本内容

# Triggers

- 用户要求使用 voice
- 用户提到"voice"
- 用户提到"notification"
- 用户提到"语音"
- 用户提到"朗读"
- 用户提到"播报"
- 用户提到"说话"


# SOP

1. 解析用户输入，提取参数
2. 调用 voice 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 voice
**助手**: 好的，我来帮你处理。

