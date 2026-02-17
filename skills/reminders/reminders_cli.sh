#!/bin/bash

# Reminders CLI Skill
# 管理提醒事项

set -e

PARAMS=$(cat)
ACTION=$(echo "$PARAMS" | jq -r '.action // "list"')
TITLE=$(echo "$PARAMS" | jq -r '.title // ""')
DUE_DATE=$(echo "$PARAMS" | jq -r '.due_date // ""')
PRIORITY=$(echo "$PARAMS" | jq -r '.priority // "0"')

case "$ACTION" in
    "list")
        # 列出所有提醒
        REMINDERS=$(osascript -e 'tell application "Reminders" to return name of every reminder of default list whose completed is false' 2>/dev/null | tr ',' '\n' | sed 's/^ *//;s/"//g' | awk '{print "{\"title\":\""$0"\"},"}' | sed '$ s/,$//')
        echo "{\"reminders\": [$REMINDERS]}"
        ;;
    "add")
        if [ -z "$TITLE" ] || [ "$TITLE" = "null" ]; then
            echo '{"error": "Missing required parameter: title"}' >&2
            exit 1
        fi
        
        # 添加提醒
        osascript -e "tell application \"Reminders\" to tell default list to make new reminder with properties {name:\"$TITLE\", due date:if \"$DUE_DATE\" = \"null\" or \"$DUE_DATE\" = \"\" then missing value else date \"$DUE_DATE\", priority:$PRIORITY}" 2>/dev/null
        echo "{\"result\": \"Reminder added: $TITLE\"}"
        ;;
    "complete")
        if [ -z "$TITLE" ] || [ "$TITLE" = "null" ]; then
            echo '{"error": "Missing required parameter: title"}' >&2
            exit 1
        fi
        
        # 完成提醒
        osascript -e "tell application \"Reminders\" to set completed of reminder \"$TITLE\" of default list to true" 2>/dev/null
        echo "{\"result\": \"Reminder completed: $TITLE\"}"
        ;;
    *)
        echo "{\"error\": \"Unknown action: $ACTION. Use 'list', 'add', or 'complete'\"}" >&2
        exit 1
        ;;
esac
