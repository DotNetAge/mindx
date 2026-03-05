---
name: weather
description: 天气查询技能，查询全球城市天气信息、气温、天气预报的标准操作程序
version: 1.0.0
author: mindx
tags:
    - weather
    - forecast
    - 天气
    - 气温
    - 天气预报
    - 查询天气
    - 温度
    - general
required_tools:
    - weather
---

# Goal

天气查询技能，查询全球城市天气信息、气温、天气预报

# Triggers

- 用户要求使用 weather
- 用户提到"weather"
- 用户提到"forecast"
- 用户提到"天气"
- 用户提到"气温"
- 用户提到"天气预报"
- 用户提到"查询天气"
- 用户提到"温度"


# SOP

1. 解析用户输入，提取参数
2. 调用 weather 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 weather
**助手**: 好的，我来帮你处理。

