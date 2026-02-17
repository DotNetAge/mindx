---
name: sysinfo
description: 系统信息查询技能，获取系统概览、磁盘、电池、网络、CPU、内存等信息
version: 1.0.0
category: system
tags:
  - system
  - info
  - 系统信息
  - 系统状态
  - 电池
  - 内存
  - CPU
  - 磁盘
os:
  - darwin
enabled: true
timeout: 30
command: ./sysinfo_cli.sh
parameters:
  type:
    type: string
    description: 信息类型："all"全部信息、"disk"磁盘、"battery"电池、"network"网络、"cpu"处理器、"memory"内存，默认"all"
    required: false
---

# 系统信息技能

## 示例
```json
{
  "name": "sysinfo",
  "parameters": {
    "type": "battery"
  }
}
```
