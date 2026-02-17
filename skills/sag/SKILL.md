---
name: sag
description: AI语音合成技能，使用ElevenLabs高质量AI语音朗读文本
version: 1.0.0
category: general
tags:
  - tts
  - speech
  - voice
  - elevenlabs
  - 语音合成
  - AI语音
  - 文字转语音
  - 朗读
os:
  - darwin
  - linux
enabled: true
timeout: 60
command: sag
requires:
  bins:
    - sag
  env:
    - ELEVENLABS_API_KEY
homepage: https://sag.sh
---

# 语音合成技能

使用 `sag` 进行 ElevenLabs 文本转语音并本地播放。

## API 密钥（必需）

- `ELEVENLABS_API_KEY`（首选）
- `SAG_API_KEY` 也被 CLI 支持

## 快速开始

```bash
sag "你好"
sag speak -v "Roger" "你好"
sag voices
sag prompting  # 模型特定提示
```

## 模型说明

- 默认: `eleven_v3`（表现力强）
- 稳定: `eleven_multilingual_v2`
- 快速: `eleven_flash_v2_5`

## 发音和交付规则

- 首先修复: 重新拼写（例如 "key-note"），添加连字符，调整大小写
- 数字/单位/URL: `--normalize auto`
- 语言偏置: `--lang en|de|fr|...` 来指导规范化
- v3: 不支持 SSML `<break>`；使用 `[pause]`、`[short pause]`、`[long pause]`
- v2/v2.5: 支持 SSML `<break time="1.5s" />`

## v3 音频标签（放在行首）

- `[whispers]`、`[shouts]`、`[sings]`
- `[laughs]`、`[starts laughing]`、`[sighs]`、`[exhales]`
- `[sarcastic]`、`[curious]`、`[excited]`、`[crying]`、`[mischievously]`

示例:

```bash
sag "[whispers] 保持安静。[short pause] 好吗？"
```

## 语音默认值

- `ELEVENLABS_VOICE_ID` 或 `SAG_VOICE_ID`

## 聊天语音回复

当要求"语音"回复时：

```bash
# 生成音频文件
sag -v Clawd -o /tmp/voice-reply.mp3 "你的消息在这里"

# 然后在回复中包含:
# MEDIA:/tmp/voice-reply.mp3
```

Clawd 的默认语音: `lj2rcrvANS3gaWWnczSX`（或者直接用 `-v Clawd`）
