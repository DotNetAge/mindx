---
name: camsnap
description: 摄像头截图技能，从RTSP/ONVIF摄像头获取截图和视频的标准操作程序
version: 1.0.0
author: mindx
tags:
    - camera
    - rtsp
    - onvif
    - snapshot
    - video
    - 摄像头
    - 截图
    - 视频
    - 监控
    - general
required_tools:
    - camsnap
---

# Goal

摄像头截图技能，从RTSP/ONVIF摄像头获取截图和视频

# Triggers

- 用户要求使用 camsnap
- 用户提到"camera"
- 用户提到"rtsp"
- 用户提到"onvif"
- 用户提到"snapshot"
- 用户提到"video"
- 用户提到"摄像头"
- 用户提到"截图"
- 用户提到"视频"
- 用户提到"监控"


# SOP

1. 解析用户输入，提取参数
2. 调用 camsnap 工具
3. 处理返回结果
4. 生成友好的响应


# Examples

**用户**: 请使用 camsnap
**助手**: 好的，我来帮你处理。

