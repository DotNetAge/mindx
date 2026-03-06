#!/bin/bash

# Notification CLI Skill
# 显示macOS系统通知

set -e

PARAMS=$(cat)
TITLE=$(echo "$PARAMS" | jq -r '.title // "Notification"')
MESSAGE=$(echo "$PARAMS" | jq -r '.message // ""')
SOUND=$(echo "$PARAMS" | jq -r '.sound // ""')

if [ -z "$MESSAGE" ] || [ "$MESSAGE" = "null" ]; then
    echo '{"error": "Missing required parameter: message"}' >&2
    exit 1
fi

# 发送通知
osascript -e "display notification \"$MESSAGE\" with title \"$TITLE\" ${SOUND:+sound name \"$SOUND\"}"

echo "{\"result\": \"Notification displayed\"}"
