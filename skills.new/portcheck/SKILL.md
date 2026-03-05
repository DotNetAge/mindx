---
name: portcheck
description: 端口占用查询技能，查看指定端口的占用情况、进程信息的标准操作程序
version: 1.0.0
author: mindx
tags:
    - port
    - 端口
    - 端口占用
    - 端口查询
    - 网络
    - 进程
    - system
required_tools:
    - portcheck
---

# Goal

端口占用查询技能，查看指定端口的占用情况、进程信息

# Triggers

- 1. 端口号必须是 1-65535 之间的数字
- 2. 如果端口未被占用，状态显示"空闲"，进程名/进程ID/用户显示"-"
- 3. 如果端口被占用，显示占用进程的详细信息


# SOP

1. 解析用户输入，提取参数
2. 调用 portcheck 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 portcheck
**助手**: 好的，我来帮你处理。

