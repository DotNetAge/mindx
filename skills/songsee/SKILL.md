---
name: songsee
description: 音频频谱可视化技能，从音频文件生成频谱图和可视化
version: 1.0.0
category: general
tags:
  - audio
  - spectrogram
  - visualization
  - music
  - 音频
  - 频谱
  - 可视化
  - 音乐
os:
  - darwin
  - linux
enabled: true
timeout: 60
command: songsee
requires:
  bins:
    - songsee
homepage: https://github.com/steipete/songsee
---

# 音频频谱可视化技能

从音频生成频谱图和特征面板可视化。

## 快速开始

```bash
# 频谱图
songsee track.mp3

# 多面板
songsee track.mp3 --viz spectrogram,mel,chroma,hpss,selfsim,loudness,tempogram,mfcc,flux

# 时间切片
songsee track.mp3 --start 12.5 --duration 8 -o slice.jpg

# 标准输入
cat track.mp3 | songsee - --format png -o out.png
```

## 常用参数

- `--viz` 可视化类型列表（可重复或逗号分隔）
- `--style` 调色板（classic, magma, inferno, viridis, gray）
- `--width` / `--height` 输出尺寸
- `--window` / `--hop` FFT 设置
- `--min-freq` / `--max-freq` 频率范围
- `--start` / `--duration` 时间切片
- `--format` jpg|png

## 注意事项

- WAV/MP3 解码原生支持
- 其他格式需要 ffmpeg
- 多个 `--viz` 会渲染为网格
