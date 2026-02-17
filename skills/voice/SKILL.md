---
name: voice
description: 语音播报技能，使用系统语音朗读文本内容
version: 1.0.0
category: general
tags:
  - voice
  - notification
  - 语音
  - 朗读
  - 播报
  - 说话
os:
  - darwin
enabled: true
timeout: 30
command: ./voice_cli.sh
parameters:
  text:
    type: string
    description: 要播报的文本内容
    required: true
  voice:
    type: string
    description: 语音类型
    required: false
---

# 语音技能

## 示例
```json
{
  "name": "voice",
  "parameters": {
    "text": "任务完成",
    "voice": "Samantha"
  }
}
```
