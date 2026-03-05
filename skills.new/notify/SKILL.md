---
name: notify
description: 系统通知技能，显示macOS系统通知和提醒的标准操作程序
version: 1.0.0
author: mindx
tags:
    - notification
    - alert
    - 通知
    - 提醒
    - 弹窗
    - 消息通知
    - general
required_tools:
    - notify
---

# Goal

系统通知技能，显示macOS系统通知和提醒

# Triggers

- 用户要求使用 notify
- 用户提到"notification"
- 用户提到"alert"
- 用户提到"通知"
- 用户提到"提醒"
- 用户提到"弹窗"
- 用户提到"消息通知"


# SOP

1. 解析用户输入，提取参数
2. 调用 notify 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 notify
**助手**: 好的，我来帮你处理。

