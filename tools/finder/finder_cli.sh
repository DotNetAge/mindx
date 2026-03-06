#!/bin/bash

# Finder CLI Skill
# 管理Finder和文件操作

set -e

PARAMS=$(cat)
ACTION=$(echo "$PARAMS" | jq -r '.action // "list"')
PATH_INPUT=$(echo "$PARAMS" | jq -r '.path // "."')

case "$ACTION" in
    "list")
        # 列出目录内容
        RESULT=$(ls -la "$PATH_INPUT" 2>/dev/null | tail -n +2 | awk '{print "{\"name\":\""$9"\",\"size\":\""$5"\",\"permissions\":\""$1"\",\"owner\":\""$3"\"},"}' | sed '$ s/,$//')
        echo "{\"items\": [$RESULT]}"
        ;;
    "open")
        # 在Finder中打开目录
        open "$PATH_INPUT"
        echo "{\"result\": \"Opened in Finder: $PATH_INPUT\"}"
        ;;
    "info")
        # 获取文件信息
        if [ ! -e "$PATH_INPUT" ]; then
            echo '{"error": "Path does not exist"}' >&2
            exit 1
        fi
        FILE_SIZE=$(stat -f%z "$PATH_INPUT" 2>/dev/null || stat -c%s "$PATH_INPUT" 2>/dev/null || echo "0")
        FILE_TYPE=$(file -b "$PATH_INPUT" 2>/dev/null || echo "unknown")
        echo "{\"path\": \"$PATH_INPUT\", \"size\": $FILE_SIZE, \"type\": \"$FILE_TYPE\"}"
        ;;
    *)
        echo "{\"error\": \"Unknown action: $ACTION. Use 'list', 'open', or 'info'\"}" >&2
        exit 1
        ;;
esac
