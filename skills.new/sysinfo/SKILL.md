---
name: sysinfo
description: 系统信息查询技能，获取系统概览、磁盘、电池、网络、CPU、内存等信息的标准操作程序
version: 1.0.0
author: mindx
tags:
    - system
    - info
    - 系统信息
    - 系统状态
    - 电池
    - 内存
    - CPU
    - 磁盘
    - system
required_tools:
    - sysinfo
---

# Goal

系统信息查询技能，获取系统概览、磁盘、电池、网络、CPU、内存等信息

# Triggers

- 用户要求使用 sysinfo
- 用户提到"system"
- 用户提到"info"
- 用户提到"系统信息"
- 用户提到"系统状态"
- 用户提到"电池"
- 用户提到"内存"
- 用户提到"CPU"
- 用户提到"磁盘"


# SOP

1. 解析用户输入，提取参数
2. 调用 sysinfo 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 sysinfo
**助手**: 好的，我来帮你处理。

