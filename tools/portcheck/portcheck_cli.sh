#!/bin/bash

# Port Check CLI Skill
# 查看端口占用情况

set -e

PARAMS=$(cat)
PORT=$(echo "$PARAMS" | jq -r '.port // empty')

if [ -z "$PORT" ]; then
    echo "{\"error\": \"缺少端口号参数 'port'\"}" >&2
    exit 1
fi

# 检查端口是否为数字
if ! [[ "$PORT" =~ ^[0-9]+$ ]]; then
    echo "{\"error\": \"端口号必须是数字\"}" >&2
    exit 1
fi

# 检查端口范围
if [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
    echo "{\"error\": \"端口号必须在 1-65535 范围内\"}" >&2
    exit 1
fi

# 查找占用端口的进程
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    RESULT=$(lsof -i :"$PORT" -P -n 2>/dev/null | awk 'NR==2 {print "{\"pid\": "$2", \"name\": \""$1"\", \"user\": \""$3"\"}"}')
else
    # Linux
    RESULT=$(ss -tlnp 2>/dev/null | awk -v port="$PORT" '$4 ~ ":"port"$" {print "{\"pid\": "$7", \"name\": \""$6"\"}"}')
fi

if [ -n "$RESULT" ]; then
    echo "{\"port\": $PORT, \"in_use\": true, \"process\": $RESULT}"
else
    echo "{\"port\": $PORT, \"in_use\": false, \"process\": null}"
fi
