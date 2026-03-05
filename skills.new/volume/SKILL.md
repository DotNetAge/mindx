---
name: volume
description: 音量控制技能，调节系统音量、静音、获取音量状态的标准操作程序
version: 1.0.0
author: mindx
tags:
    - volume
    - audio
    - 音量
    - 声音
    - 静音
    - 调节音量
    - system
required_tools:
    - volume
---

# Goal

音量控制技能，调节系统音量、静音、获取音量状态

# Triggers

- 用户要求使用 volume
- 用户提到"volume"
- 用户提到"audio"
- 用户提到"音量"
- 用户提到"声音"
- 用户提到"静音"
- 用户提到"调节音量"


# SOP

1. 解析用户输入，提取参数
2. 调用 volume 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 volume
**助手**: 好的，我来帮你处理。

