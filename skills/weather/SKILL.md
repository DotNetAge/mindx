---
name: weather
description: "Queries current weather and multi-day forecasts for any city worldwide. Use when the user asks about weather, temperature, rain, or forecasts for a location."
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

Retrieves weather data for a given city. Returns current conditions (temperature, sky) and optional multi-day forecast.

## Workflow

1. Identify the city from the user's request (Chinese or English name).
2. Set `days` for a multi-day forecast (default 1 = today only).
3. Call the skill and parse the JSON response.
4. Present temperature, conditions, and forecast to the user.

## Examples

Current weather for Beijing:

```json
{
  "name": "weather",
  "parameters": {
    "city": "北京"
  }
}
```

3-day forecast for New York:

```json
{
  "name": "weather",
  "parameters": {
    "city": "New York",
    "days": 3
  }
}
```
