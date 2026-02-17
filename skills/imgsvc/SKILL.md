---
name: imgsvc
description: 图片搜索下载技能，搜索和下载网络图片
version: 1.0.0
category: general
tags:
  - image
  - download
  - search
  - proxy
  - 图片
  - 搜索图片
  - 下载图片
  - 图片搜索
os:
  - darwin
  - linux
enabled: true
timeout: 60
command: imgsvc
requires:
  bins:
    - imgsvc
homepage: https://github.com/imgsvc/imgsvc
---

# 图穹图片服务技能

图穹提供对国内国外网站内容的无障碍搜索（自动翻墙），还提供处理海量图片下载的自动管理与后台下载能力。

## 快速开始

```bash
# 使用Google搜索
imgsvc search "搜索关键字"

# 无障碍访问（自动翻墙）获取网页源码
imgsvc open "https://example.com/gallery"

# 下载图片（自动翻墙）
imgsvc download "https://example.com/image.jpg" "./downloaded_image.jpg"

# 推送图片地址（由图穹自动管理下载）
imgsvc push "https://example.com/new_image.jpg"
```

## 功能特点

- 国内外网站无障碍访问
- 自动翻墙
- 海量图片后台下载
- 下载任务自动管理
