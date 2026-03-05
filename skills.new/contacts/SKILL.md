---
name: contacts
description: 联系人管理技能，搜索、查看、添加联系人信息的标准操作程序
version: 1.0.0
author: mindx
tags:
    - contacts
    - address-book
    - 联系人
    - 通讯录
    - 电话本
    - 电话
    - phone
    - 查找联系人
    - productivity
required_tools:
    - contacts
---

# Goal

联系人管理技能，搜索、查看、添加联系人信息

# Triggers

- 搜索联系人时只需提供 action 和 name 两个参数，不需要 phone 和 email。
- 示例：{"action":"search","name":"张三"}
- 添加联系人时才需要提供 phone 或 email。


# SOP

1. 解析用户输入，提取参数
2. 调用 contacts 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 contacts
**助手**: 好的，我来帮你处理。

