---
name: imessage
description: iMessage短信发送技能，发送iMessage或短信到指定联系人
version: 1.0.0
category: communication
tags:
  - imessage
  - sms
  - messaging
  - 短信
  - 发送短信
  - 发消息
  - iMessage
os:
  - darwin
enabled: true
timeout: 30
command: ./imessage_cli.sh
parameters:
  to:
    type: string
    description: 接收者（邮箱地址用于iMessage，电话号码用于SMS）
    required: true
  message:
    type: string
    description: 消息内容
    required: true
  service:
    type: string
    description: 服务类型："iMessage"或"SMS"，默认自动检测
    required: false
---

# iMessage 技能

## 示例
发送 iMessage：
```json
{
  "name": "imessage",
  "parameters": {
    "to": "friend@icloud.com",
    "message": "今天有空一起吃午饭吗？"
  }
}
```

发送短信：
```json
{
  "name": "imessage",
  "parameters": {
    "to": "+8613800138000",
    "message": "下午3点开会",
    "service": "SMS"
  }
}
```
