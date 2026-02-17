---
name: volume
description: 音量控制技能，调节系统音量、静音、获取音量状态
version: 1.0.0
category: system
tags:
  - volume
  - audio
  - 音量
  - 声音
  - 静音
  - 调节音量
os:
  - darwin
enabled: true
timeout: 30
command: ./volume_cli.sh
parameters:
  action:
    type: string
    description: 操作类型："get"获取音量、"set"设置音量、"mute"静音、"unmute"取消静音、"increase"增加音量、"decrease"降低音量
    required: true
  level:
    type: number
    description: 音量级别（0-100，set时必需）
    required: false
---

# 音量技能

## 示例
```json
{
  "name": "volume",
  "parameters": {
    "action": "set",
    "level": 50
  }
}
```
