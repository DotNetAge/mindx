#!/bin/bash

# Terminal CLI Skill
# 执行终端命令

set -e

PARAMS=$(cat)
COMMAND=$(echo "$PARAMS" | jq -r '.command // empty')
TIMEOUT=$(echo "$PARAMS" | jq -r '.timeout // "30"')

if [ -z "$COMMAND" ] || [ "$COMMAND" = "null" ]; then
    echo '{"error": "Missing required parameter: command"}' >&2
    exit 1
fi

# 执行命令并设置超时（macOS兼容）
if [ "$TIMEOUT" != "0" ] && [ "$TIMEOUT" != "null" ]; then
    # 使用临时文件捕获输出
    OUTPUT_FILE=$(mktemp)
    
    # 在后台执行命令并捕获输出
    bash -c "$COMMAND" > "$OUTPUT_FILE" 2>&1 &
    PID=$!
    
    # 等待命令完成或超时
    for i in $(seq 1 "$TIMEOUT"); do
        if ! kill -0 "$PID" 2>/dev/null; then
            break
        fi
        sleep 1
    done
    
    if kill -0 "$PID" 2>/dev/null; then
        kill "$PID" 2>/dev/null
        wait "$PID" 2>/dev/null || true
        rm -f "$OUTPUT_FILE"
        echo '{"error": "Command timed out"}' >&2
        exit 124
    fi
    
    wait "$PID"
    EXIT_CODE=$?
    OUTPUT=$(cat "$OUTPUT_FILE")
    rm -f "$OUTPUT_FILE"
else
    OUTPUT=$(bash -c "$COMMAND" 2>&1)
    EXIT_CODE=$?
fi

if [ $EXIT_CODE -eq 0 ]; then
    # 成功：转义输出中的特殊字符
    ESCAPED_OUTPUT=$(echo "$OUTPUT" | sed 's/"/\\"/g' | tr -d '\n' | tr -d '\r')
    echo "{\"result\": \"$ESCAPED_OUTPUT\", \"exit_code\": $EXIT_CODE}"
else
    # 失败
    echo "{\"error\": \"Command failed with exit code $EXIT_CODE\", \"output\": \"$OUTPUT\"}" >&2
    exit $EXIT_CODE
fi