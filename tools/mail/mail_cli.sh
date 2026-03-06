#!/bin/bash

# Mail CLI Skill
# 发送邮件

set -e

PARAMS=$(cat)
TO=$(echo "$PARAMS" | jq -r '.to // ""')
SUBJECT=$(echo "$PARAMS" | jq -r '.subject // ""')
BODY=$(echo "$PARAMS" | jq -r '.body // ""')
CC=$(echo "$PARAMS" | jq -r '.cc // ""')
BCC=$(echo "$PARAMS" | jq -r '.bcc // ""')

if [ -z "$TO" ] || [ "$TO" = "null" ]; then
    echo '{"error": "Missing required parameter: to"}' >&2
    exit 1
fi

if [ -z "$SUBJECT" ] || [ "$SUBJECT" = "null" ]; then
    echo '{"error": "Missing required parameter: subject"}' >&2
    exit 1
fi

# 构建邮件内容
TMPFILE=$(mktemp)
{
    echo "Subject: $SUBJECT"
    [ -n "$CC" ] && [ "$CC" != "null" ] && echo "Cc: $CC"
    [ -n "$BCC" ] && [ "$BCC" != "null" ] && echo "Bcc: $BCC"
    echo "To: $TO"
    echo ""
    echo "$BODY"
} > "$TMPFILE"

# 发送邮件（使用sendmail或mail命令）
if command -v sendmail &> /dev/null; then
    # 使用sendmail
    cat "$TMPFILE" | sendmail -t
    RESULT="Email sent using sendmail"
elif command -v mail &> /dev/null; then
    # 使用mail命令
    [ -n "$CC" ] && [ "$CC" != "null" ] && CC_ARG="-c $CC"
    [ -n "$BCC" ] && [ "$BCC" != "null" ] && BCC_ARG="-b $BCC"
    echo "$BODY" | mail $CC_ARG $BCC_ARG -s "$SUBJECT" "$TO"
    RESULT="Email sent using mail command"
else
    # 使用AppleScript打开Mail应用准备发送
    osascript -e "tell application \"Mail\" to make new outgoing message with properties {visible:true, subject:\"$SUBJECT\", content:\"$BODY\"} at end of outgoing messages" 2>/dev/null
    osascript -e "tell application \"Mail\" to tell the last outgoing message to make new to recipient at end of to recipients with properties {address:\"$TO\"}" 2>/dev/null
    if [ -n "$CC" ] && [ "$CC" != "null" ]; then
        osascript -e "tell application \"Mail\" to tell the last outgoing message to make new cc recipient at end of cc recipients with properties {address:\"$CC\"}" 2>/dev/null
    fi
    osascript -e "tell application \"Mail\" to activate" 2>/dev/null
    RESULT="Mail application opened with email ready to send"
fi

# 清理临时文件
rm -f "$TMPFILE"

echo "{\"result\": \"$RESULT\", \"to\": \"$TO\", \"subject\": \"$SUBJECT\"}"
