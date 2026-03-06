#!/bin/bash

# iMessage CLI Skill
# 发送iMessage

set -e

PARAMS=$(cat)
TO=$(echo "$PARAMS" | jq -r '.to // ""')
MESSAGE=$(echo "$PARAMS" | jq -r '.message // ""')
SERVICE=$(echo "$PARAMS" | jq -r '.service // "iMessage"')

if [ -z "$TO" ] || [ "$TO" = "null" ]; then
    echo '{"error": "Missing required parameter: to"}' >&2
    exit 1
fi

if [ -z "$MESSAGE" ] || [ "$MESSAGE" = "null" ]; then
    echo '{"error": "Missing required parameter: message"}' >&2
    exit 1
fi

# 确定服务类型
case "$SERVICE" in
    "iMessage"|"imessage")
        SERVICE_NAME="iMessage"
        ;;
    "SMS"|"sms")
        SERVICE_NAME="SMS"
        ;;
    *)
        # 如果TO看起来像电话号码，使用SMS，否则使用iMessage
        if echo "$TO" | grep -q '^[0-9]\+$'; then
            SERVICE_NAME="SMS"
        else
            SERVICE_NAME="iMessage"
        fi
        ;;
esac

# 转义消息中的特殊字符
ESCAPED_MESSAGE=$(echo "$MESSAGE" | sed 's/\\/\\\\/g; s/"/\\"/g')

# 使用AppleScript发送iMessage
RESULT=$(osascript -e "
tell application \"Messages\"
    set targetService to \"$SERVICE_NAME\"
    set targetBuddy to \"$TO\"
    set myMessage to \"$ESCAPED_MESSAGE\"
    
    try
        set theService to service targetService
        if theService is missing then
            set theService to 1st service whose service type = targetService
        end if
        
        send myMessage to buddy targetBuddy of theService
        return \"Message sent successfully\"
    on error errMsg
        return \"Error: \" & errMsg
    end try
end tell
" 2>&1)

if echo "$RESULT" | grep -qi "error"; then
    echo "{\"error\": \"$RESULT\"}" >&2
    exit 1
else
    echo "{\"result\": \"$RESULT\", \"to\": \"$TO\", \"service\": \"$SERVICE_NAME\"}"
fi
