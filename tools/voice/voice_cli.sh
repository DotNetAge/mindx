#!/bin/bash

# Voice CLI Skill
# 从stdin接收JSON参数，执行语音播报

set -e

# 读取JSON参数
PARAMS=$(cat)

# 解析参数（使用jq）
TEXT=$(echo "$PARAMS" | jq -r '.text // empty')
VOICE=$(echo "$PARAMS" | jq -r '.voice // empty')

# 验证必需参数
if [ -z "$TEXT" ] || [ "$TEXT" = "null" ]; then
    echo '{"error": "Missing required parameter: text"}' >&2
    exit 1
fi

# 构建say命令
CMD=("say")

# 添加语音参数（如果指定）
if [ -n "$VOICE" ] && [ "$VOICE" != "null" ]; then
    CMD+=("-v" "$VOICE")
fi

# 添加要播报的文本
CMD+=("$TEXT")

# 执行命令
if "${CMD[@]}"; then
    echo "{\"result\": \"Voice notification sent: $TEXT\"}"
else
    EXIT_CODE=$?
    echo "{\"error\": \"Failed to execute say command\", \"code\": $EXIT_CODE}" >&2
    exit $EXIT_CODE
fi
