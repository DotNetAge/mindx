#!/bin/bash

# Volume CLI Skill
# 控制系统音量

set -e

PARAMS=$(cat)
ACTION=$(echo "$PARAMS" | jq -r '.action // "get"')
LEVEL=$(echo "$PARAMS" | jq -r '.level // ""')

case "$ACTION" in
    "get")
        # 获取当前音量（0-100）
        VOLUME=$(osascript -e 'output volume of (get volume settings)' 2>/dev/null)
        echo "{\"volume\": $VOLUME}"
        ;;
    "set")
        if [ -z "$LEVEL" ] || [ "$LEVEL" = "null" ]; then
            echo '{"error": "Missing required parameter: level"}' >&2
            exit 1
        fi
        
        # 设置音量
        osascript -e "set volume output volume $LEVEL" 2>/dev/null
        echo "{\"result\": \"Volume set to $LEVEL\"}"
        ;;
    "mute")
        # 静音
        osascript -e "set volume with output muted" 2>/dev/null
        echo "{\"result\": \"Volume muted\"}"
        ;;
    "unmute")
        # 取消静音
        osascript -e "set volume without output muted" 2>/dev/null
        echo "{\"result\": \"Volume unmuted\"}"
        ;;
    "increase")
        # 增加音量
        osascript -e "set volume (output volume of (get volume settings) + 10)" 2>/dev/null
        echo "{\"result\": \"Volume increased\"}"
        ;;
    "decrease")
        # 降低音量
        osascript -e "set volume (output volume of (get volume settings) - 10)" 2>/dev/null
        echo "{\"result\": \"Volume decreased\"}"
        ;;
    *)
        echo "{\"error\": \"Unknown action: $ACTION. Use 'get', 'set', 'mute', 'unmute', 'increase', or 'decrease'\"}" >&2
        exit 1
        ;;
esac
