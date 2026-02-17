---
name: mail
description: 邮件发送技能，发送电子邮件到指定邮箱地址
version: 1.0.0
category: communication
tags:
  - email
  - mail
  - 邮件
  - 发送邮件
  - 发邮件
  - 电子邮件
os:
  - darwin
  - linux
enabled: true
timeout: 30
command: ./mail_cli.sh
parameters:
  to:
    type: string
    description: 收件人邮箱地址
    required: true
  subject:
    type: string
    description: 邮件主题
    required: true
  body:
    type: string
    description: 邮件正文
    required: true
  cc:
    type: string
    description: 抄送地址
    required: false
  bcc:
    type: string
    description: 密送地址
    required: false
---

# 邮件技能

## 示例
```json
{
  "name": "mail",
  "parameters": {
    "to": "example@email.com",
    "subject": "会议提醒",
    "body": "你好，提醒你明天下午3点的会议。"
  }
}
```
