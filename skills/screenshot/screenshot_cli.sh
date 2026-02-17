#!/bin/bash

# Screenshot CLI Skill
# 截取屏幕截图

set -e

PARAMS=$(cat)
TYPE=$(echo "$PARAMS" | jq -r '.type // "screen"')
FILENAME=$(echo "$PARAMS" | jq -r '.filename // ""')
DELAY=$(echo "$PARAMS" | jq -r '.delay // "0"')

# 构建保存路径
if [ -z "$FILENAME" ] || [ "$FILENAME" = "null" ]; then
    FILENAME="$HOME/Desktop/screenshot_$(date +%Y%m%d_%H%M%S).png"
fi

# 延迟（秒）
if [ "$DELAY" != "0" ] && [ "$DELAY" != "null" ]; then
    sleep "$DELAY"
fi

case "$TYPE" in
    "screen")
        # 截取整个屏幕
        screencapture "$FILENAME"
        ;;
    "selection")
        # 交互式选择区域截图
        screencapture -i "$FILENAME"
        ;;
    "window")
        # 截取窗口
        screencapture -w "$FILENAME"
        ;;
    *)
        echo "{\"error\": \"Unknown type: $TYPE. Use 'screen', 'selection', or 'window'\"}" >&2
        exit 1
        ;;
esac

echo "{\"result\": \"Screenshot saved to $FILENAME\", \"filename\": \"$FILENAME\"}"
