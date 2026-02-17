#!/bin/bash

read -r json_input

path=$(echo "$json_input" | jq -r '.path // empty')

if [ -z "$path" ]; then
    echo '{"error": "缺少必需参数: path"}'
    exit 1
fi

if [ ! -f "$path" ]; then
    echo "{\"success\": false, \"error\": \"文件不存在: $path\"}"
    exit 1
fi

if [ ! -r "$path" ]; then
    echo "{\"success\": false, \"error\": \"没有文件读取权限: $path\"}"
    exit 1
fi

content=$(cat "$path" 2>&1)
exit_code=$?

if [ $exit_code -eq 0 ]; then
    content_escaped=$(printf '%s' "$content" | jq -R -s '.')
    bytes_read=$(wc -c < "$path")
    echo "{\"success\": true, \"path\": \"$path\", \"content\": $content_escaped, \"bytes_read\": $bytes_read}"
else
    error_escaped=$(printf '%s' "$content" | jq -R -s '.')
    echo "{\"success\": false, \"error\": $error_escaped}"
    exit 1
fi
