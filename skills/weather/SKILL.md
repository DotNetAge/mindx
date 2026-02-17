---
name: weather
description: 天气查询技能，查询全球城市天气信息、气温、天气预报
version: 1.0.0
category: general
tags:
  - weather
  - forecast
  - 天气
  - 气温
  - 天气预报
  - 查询天气
  - 温度
os:
  - darwin
  - linux
enabled: true
timeout: 60
command: ./weather_cli.sh
parameters:
  city:
    type: string
    description: 城市名称，如"北京"、"New York"
    required: true
  days:
    type: number
    description: 查询天数，默认1天
    required: false
---

# 天气技能

## 示例
```json
{
  "name": "weather",
  "parameters": {
    "city": "北京",
    "days": 3
  }
}
```
