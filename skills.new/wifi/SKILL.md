---
name: wifi
description: WiFi管理技能，查看WiFi状态、连接、断开WiFi网络的标准操作程序
version: 1.0.0
author: mindx
tags:
    - wifi
    - network
    - WiFi
    - 无线网络
    - 连接WiFi
    - 网络连接
    - system
required_tools:
    - wifi
---

# Goal

WiFi管理技能，查看WiFi状态、连接、断开WiFi网络

# Triggers

- 用户要求使用 wifi
- 用户提到"wifi"
- 用户提到"network"
- 用户提到"WiFi"
- 用户提到"无线网络"
- 用户提到"连接WiFi"
- 用户提到"网络连接"


# SOP

1. 解析用户输入，提取参数
2. 调用 wifi 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 wifi
**助手**: 好的，我来帮你处理。

