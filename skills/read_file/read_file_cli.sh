#!/bin/bash

read -r json_input

path=$(echo "$json_input" | jq -r '.path // empty')

if [ -z "$path" ]; then
    echo '{"error": "缺少必需参数: path"}'
    exit 1
fi

# SECURITY: 路径验证
# 检查路径遍历
if echo "$path" | grep -q '\.\.'; then
    echo '{"error": "Path traversal detected: .. not allowed"}'
    exit 1
fi

# 设置允许的基础目录
BASE_DIR="${MINDX_WORKSPACE}/documents"
if [ ! -d "$BASE_DIR" ]; then
    BASE_DIR="${MINDX_WORKSPACE}/data"
fi

# 构建完整路径
FULL_PATH="${BASE_DIR}/${path}"

# 规范化路径并验证（使用realpath的-m选项，不会检查文件是否存在）
REAL_PATH=$(realpath -m "$FULL_PATH" 2>/dev/null)
REAL_BASE=$(realpath -m "$BASE_DIR" 2>/dev/null)

# 检查规范化后的路径是否仍在基础目录内
if [[ "$REAL_PATH" != "$REAL_BASE"* ]]; then
    echo '{"error": "Access denied: path outside allowed directory"}'
    exit 1
fi

# 使用验证后的路径
path="$REAL_PATH"


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
