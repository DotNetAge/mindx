---
name: screenshot
description: 截图技能，截取屏幕截图，支持全屏、区域选择和窗口截图
version: 1.0.0
category: general
tags:
  - screenshot
  - capture
  - 截图
  - 截屏
  - 屏幕截图
  - 抓图
os:
  - darwin
enabled: true
timeout: 30
command: ./screenshot_cli.sh
parameters:
  type:
    type: string
    description: 截图类型："screen"全屏、"selection"选择区域、"window"窗口，默认"screen"
    required: false
  filename:
    type: string
    description: 保存路径，默认为~/Desktop/screenshot_YYYYMMDD_HHMMSS.png
    required: false
  delay:
    type: number
    description: 延迟秒数，默认0
    required: false
---

# 截图技能

## 示例
```json
{
  "name": "screenshot",
  "parameters": {
    "type": "screen",
    "delay": 2
  }
}
```
