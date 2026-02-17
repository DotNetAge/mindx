#!/bin/bash

# Notes CLI Skill
# 管理macOS Notes应用中的笔记

set -e

PARAMS=$(cat)
ACTION=$(echo "$PARAMS" | jq -r '.action // "create"')
TITLE=$(echo "$PARAMS" | jq -r '.title // ""')
CONTENT=$(echo "$PARAMS" | jq -r '.content // ""')

case "$ACTION" in
    "create")
        if [ -z "$TITLE" ] || [ "$TITLE" = "null" ]; then
            echo '{"error": "Missing required parameter: title"}' >&2
            exit 1
        fi
        # 创建新笔记
        osascript -e "tell application \"Notes\" to make new note at folder \"Notes\" with properties {name:\"$TITLE\", body:\"$CONTENT\"}" 2>/dev/null
        echo "{\"result\": \"Note created: $TITLE\"}"
        ;;
    "list")
        # 列出所有笔记
        NOTE_LIST=$(osascript -e 'tell application "Notes" to return name of every note in folder "Notes"' 2>/dev/null | tr ',' '\n' | sed 's/^ *//;s/"//g' | awk '{print "{\"title\":\""$0"\"},"}' | sed '$ s/,$//')
        echo "{\"notes\": [$NOTE_LIST]}"
        ;;
    "open")
        if [ -z "$TITLE" ] || [ "$TITLE" = "null" ]; then
            echo '{"error": "Missing required parameter: title"}' >&2
            exit 1
        fi
        # 打开指定笔记
        osascript -e "tell application \"Notes\" to open note \"$TITLE\"" 2>/dev/null
        echo "{\"result\": \"Note opened: $TITLE\"}"
        ;;
    *)
        echo "{\"error\": \"Unknown action: $ACTION. Use 'create', 'list', or 'open'\"}" >&2
        exit 1
        ;;
esac
