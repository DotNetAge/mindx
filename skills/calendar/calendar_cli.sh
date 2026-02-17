#!/bin/bash

# Calendar CLI Skill
# 管理日历事件

set -e

PARAMS=$(cat)
ACTION=$(echo "$PARAMS" | jq -r '.action // "list"')
TITLE=$(echo "$PARAMS" | jq -r '.title // ""')
START_DATE=$(echo "$PARAMS" | jq -r '.start_date // ""')
END_DATE=$(echo "$PARAMS" | jq -r '.end_date // ""')
DAYS=$(echo "$PARAMS" | jq -r '.days // "7"')

case "$ACTION" in
    "list")
        # 列出未来指定天数的事件
        if [ -z "$START_DATE" ] || [ "$START_DATE" = "null" ]; then
            START_DATE=$(date +"%Y/%m/%d")
        fi
        
        if [ -z "$END_DATE" ] || [ "$END_DATE" = "null" ]; then
            END_DATE=$(date -v+"$DAYS"d +"%Y/%m/%d" 2>/dev/null || date -d "+$DAYS days" +"%Y/%m/%d" 2>/dev/null)
        fi
        
        EVENTS=$(osascript -e "tell application \"Calendar\" to return summary of every event whose start date is greater than (date \"$START_DATE\") and start date is less than (date \"$END_DATE\")" 2>/dev/null | tr ',' '\n' | sed 's/^ *//;s/"//g' | awk '{print "{\"summary\":\""$0"\"},"}' | sed '$ s/,$//')
        echo "{\"events\": [$EVENTS], \"from\": \"$START_DATE\", \"to\": \"$END_DATE\"}"
        ;;
    "create")
        if [ -z "$TITLE" ] || [ "$TITLE" = "null" ] || [ -z "$START_DATE" ] || [ "$START_DATE" = "null" ]; then
            echo '{"error": "Missing required parameter: title or start_date"}' >&2
            exit 1
        fi
        
        # 简化版：创建事件（需要更多信息才能完全工作）
        echo "{\"result\": \"Event creation requested: $TITLE on $START_DATE\", \"note\": \"This is a simplified implementation\"}"
        ;;
    *)
        echo "{\"error\": \"Unknown action: $ACTION. Use 'list' or 'create'\"}" >&2
        exit 1
        ;;
esac
