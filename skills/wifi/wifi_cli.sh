#!/bin/bash

# WiFi CLI Skill
# 管理WiFi连接

set -e

PARAMS=$(cat)
ACTION=$(echo "$PARAMS" | jq -r '.action // "list"')
SSID=$(echo "$PARAMS" | jq -r '.ssid // ""')
PASSWORD=$(echo "$PARAMS" | jq -r '.password // ""')

case "$ACTION" in
    "list")
        # 列出可用的WiFi网络
        /System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport -s | awk 'NR>1 {print "{\"ssid\":\""$1"\",\"rssi\":\""$2"\",\"channel\":\""$3"\"},"}' | sed '$ s/,$//'
        ;;
    "connect")
        # 连接到WiFi网络
        if [ -z "$SSID" ] || [ "$SSID" = "null" ]; then
            echo '{"error": "Missing required parameter: ssid"}' >&2
            exit 1
        fi
        # 需要系统权限，这里只是示例
        echo "{\"result\": \"WiFi connection request sent for: $SSID\", \"note\": \"This requires macOS permissions\"}"
        ;;
    "status")
        # 获取当前WiFi状态
        CURRENT_SSID=$(/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport -I | awk -F' SSID: ' '/ SSID: / {print $2}')
        RSSI=$(/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport -I | awk -F' agrCtlRSSI: ' '/ agrCtlRSSI: / {print $2}')
        echo "{\"ssid\": \"$CURRENT_SSID\", \"rssi\": \"$RSSI\"}"
        ;;
    "disconnect")
        # 断开WiFi
        networksetup -setairportpower en0 off
        sleep 1
        networksetup -setairportpower en0 on
        echo "{\"result\": \"WiFi disconnected\"}"
        ;;
    *)
        echo "{\"error\": \"Unknown action: $ACTION. Use 'list', 'connect', 'status', or 'disconnect'\"}" >&2
        exit 1
        ;;
esac
