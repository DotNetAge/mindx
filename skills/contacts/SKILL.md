---
name: contacts
description: 联系人管理技能，搜索、查看、添加联系人信息
version: 1.0.0
category: productivity
tags:
  - contacts
  - address-book
  - 联系人
  - 通讯录
  - 电话本
  - 查找联系人
os:
  - darwin
enabled: true
timeout: 30
command: ./contacts_cli.sh
parameters:
  action:
    type: string
    description: 操作类型："search"搜索、"list"列出、"add"添加
    required: true
  name:
    type: string
    description: 联系人姓名（搜索和添加时使用）
    required: false
  phone:
    type: string
    description: 电话号码
    required: false
  email:
    type: string
    description: 邮箱地址
    required: false
---

# 联系人技能

## 示例
搜索联系人：
```json
{
  "name": "contacts",
  "parameters": {
    "action": "search",
    "name": "张三"
  }
}
```

添加联系人：
```json
{
  "name": "contacts",
  "parameters": {
    "action": "add",
    "name": "李四",
    "phone": "+8613800138000",
    "email": "lisi@example.com"
  }
}
```
