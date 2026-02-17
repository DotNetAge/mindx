---
name: wifi
description: WiFi管理技能，查看WiFi状态、连接、断开WiFi网络
version: 1.0.0
category: system
tags:
  - wifi
  - network
  - WiFi
  - 无线网络
  - 连接WiFi
  - 网络连接
os:
  - darwin
enabled: true
timeout: 30
command: ./wifi_cli.sh
parameters:
  action:
    type: string
    description: 操作类型："list"列出网络、"connect"连接、"status"查看状态、"disconnect"断开
    required: true
  ssid:
    type: string
    description: WiFi网络名称（连接时需要）
    required: false
  password:
    type: string
    description: WiFi密码（连接时需要）
    required: false
---

# WiFi 技能

## 示例
```json
{
  "name": "wifi",
  "parameters": {
    "action": "list"
  }
}
```
