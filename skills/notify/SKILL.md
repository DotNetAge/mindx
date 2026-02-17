---
name: notify
description: 系统通知技能，显示macOS系统通知和提醒
version: 1.0.0
category: general
tags:
  - notification
  - alert
  - 通知
  - 提醒
  - 弹窗
  - 消息通知
os:
  - darwin
enabled: true
timeout: 30
command: ./notify_cli.sh
parameters:
  title:
    type: string
    description: 通知标题，默认"Notification"
    required: false
  message:
    type: string
    description: 通知消息内容
    required: true
  sound:
    type: string
    description: 提示音名称
    required: false
---

# 通知技能

## 示例
```json
{
  "name": "notify",
  "parameters": {
    "title": "任务完成",
    "message": "您的任务已成功完成"
  }
}
```
