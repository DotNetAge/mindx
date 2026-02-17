#!/bin/bash

# Clipboard CLI Skill
# 管理剪贴板内容

set -e

PARAMS=$(cat)
ACTION=$(echo "$PARAMS" | jq -r '.action // "get"')
TEXT=$(echo "$PARAMS" | jq -r '.text // empty')

case "$ACTION" in
    "get")
        # 获取剪贴板内容
        CONTENT=$(pbpaste)
        echo "{\"content\": \"$CONTENT\"}"
        ;;
    "set")
        # 设置剪贴板内容
        if [ -z "$TEXT" ] || [ "$TEXT" = "null" ]; then
            echo '{"error": "Missing required parameter: text for set action"}' >&2
            exit 1
        fi
        echo "$TEXT" | pbcopy
        echo "{\"result\": \"Clipboard updated\"}"
        ;;
    *)
        echo "{\"error\": \"Unknown action: $ACTION. Use 'get' or 'set'\"}" >&2
        exit 1
        ;;
esac
