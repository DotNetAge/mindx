---
name: camsnap
description: 摄像头截图技能，从RTSP/ONVIF摄像头获取截图和视频
version: 1.0.0
category: general
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
os:
  - darwin
  - linux
enabled: true
timeout: 60
command: camsnap
requires:
  bins:
    - camsnap
    - ffmpeg
homepage: https://camsnap.ai
---

# 摄像头捕获技能

使用 `camsnap` 从 RTSP/ONVIF 摄像头获取截图、视频片段或移动事件。

## 配置

- 配置文件: `~/.config/camsnap/config.yaml`
- 添加摄像头: `camsnap add --name kitchen --host 192.168.0.10 --user user --pass pass`

## 常用命令

- 发现设备: `camsnap discover --info`
- 截图: `camsnap snap kitchen --out shot.jpg`
- 录制视频: `camsnap clip kitchen --dur 5s --out clip.mp4`
- 移动侦测: `camsnap watch kitchen --threshold 0.2 --action '...'`
- 诊断: `camsnap doctor --probe`

## 注意事项

- 需要 `ffmpeg` 在 PATH 中
- 长时间录制前建议先进行短测试
