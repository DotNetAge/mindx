#!/bin/bash

# Open CLI Skill
# 使用open命令打开文件、URL或应用程序

set -e

PARAMS=$(cat)
TARGET=$(echo "$PARAMS" | jq -r '.target // empty')
APP=$(echo "$PARAMS" | jq -r '.app // ""')
TYPE=$(echo "$PARAMS" | jq -r '.type // "auto"')

if [ -z "$TARGET" ] || [ "$TARGET" = "null" ]; then
    echo '{"error": "Missing required parameter: target"}' >&2
    exit 1
fi

case "$TYPE" in
    "auto")
        # 自动检测类型
        if [ -n "$APP" ] && [ "$APP" != "null" ]; then
            open -a "$APP" "$TARGET"
            echo "{\"result\": \"Opened $TARGET with $APP\"}"
        else
            open "$TARGET"
            echo "{\"result\": \"Opened $TARGET\"}"
        fi
        ;;
    "url")
        # 打开URL
        open "$TARGET"
        echo "{\"result\": \"Opened URL: $TARGET\"}"
        ;;
    "file")
        # 打开文件
        open "$TARGET"
        echo "{\"result\": \"Opened file: $TARGET\"}"
        ;;
    "app")
        # 打开应用
        open -a "$TARGET"
        echo "{\"result\": \"Opened app: $TARGET\"}"
        ;;
    *)
        echo "{\"error\": \"Unknown type: $TYPE. Use 'auto', 'url', 'file', or 'app'\"}" >&2
        exit 1
        ;;
esac
