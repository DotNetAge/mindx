---
name: peekaboo
description: UI自动化技能，屏幕捕获、UI元素定位、自动点击和应用窗口管理
version: 1.0.0
category: system
tags:
  - automation
  - ui
  - screenshot
  - macos
  - UI自动化
  - 屏幕捕获
  - 自动点击
  - 窗口管理
os:
  - darwin
enabled: true
timeout: 60
command: peekaboo
requires:
  bins:
    - peekaboo
homepage: https://peekaboo.boo
---

# Peekaboo UI自动化技能

Peekaboo 是一个完整的 macOS UI 自动化 CLI：捕获/检查屏幕、定位 UI 元素、驱动输入、管理应用/窗口/菜单。

## 快速开始

```bash
peekaboo permissions
peekaboo list apps --json
peekaboo see --annotate --path /tmp/peekaboo-see.png
peekaboo click --on B1
peekaboo type "Hello" --return
```

## 核心功能

### 交互操作

- `click`: 通过 ID/查询/坐标点击，支持智能等待
- `drag`: 在元素/坐标/Dock 之间拖放
- `hotkey`: 组合键如 `cmd,shift,t`
- `move`: 光标定位，可选平滑移动
- `paste`: 设置剪贴板 -> 粘贴 -> 恢复
- `press`: 特殊键序列
- `scroll`: 方向滚动（定向 + 平滑）
- `type`: 文本 + 控制键

### 系统操作

- `app`: 启动/退出/重启动/隐藏/切换应用
- `clipboard`: 读写剪贴板（文本/图片/文件）
- `dialog`: 点击/输入/关闭系统对话框
- `dock`: 启动/右键点击/隐藏 Dock 项目
- `menu`: 点击/列出应用菜单
- `window`: 关闭/最小化/最大化/移动/调整窗口

### 视觉操作

- `see`: 带注释的 UI 地图、快照 ID、可选分析
- `image`: 截图（屏幕/窗口/菜单栏区域）

## 常用示例

### 查看 -> 点击 -> 输入

```bash
peekaboo see --app Safari --window-title "Login" --annotate --path /tmp/see.png
peekaboo click --on B3 --app Safari
peekaboo type "user@example.com" --app Safari
peekaboo press tab --count 1 --app Safari
peekaboo type "supersecret" --app Safari --return
```

### 截图 + 分析

```bash
peekaboo image --mode screen --screen-index 0 --retina --path /tmp/screen.png
peekaboo image --app Safari --window-title "Dashboard" --analyze "总结KPI"
```

### 应用 + 窗口管理

```bash
peekaboo app launch "Safari" --open https://example.com
peekaboo window focus --app Safari --window-title "Example"
peekaboo window set-bounds --app Safari --x 50 --y 50 --width 1200 --height 800
peekaboo app quit --app Safari
```

## 注意事项

- 需要屏幕录制 + 辅助功能权限
- 使用 `peekaboo see --annotate` 在点击前识别目标
