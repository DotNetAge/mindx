#!/bin/bash

# System Info CLI Skill
# 获取系统信息

set -e

PARAMS=$(cat)
TYPE=$(echo "$PARAMS" | jq -r '.type // "all"')

case "$TYPE" in
    "all" | "overview")
        # 系统概览
        OS=$(sw_vers -productVersion)
        MODEL=$(system_profiler SPHardwareDataType | awk -F': ' '/Model Name/ {print $2}')
        PROCESSOR=$(system_profiler SPHardwareDataType | awk -F': ' '/Processor Name/ {print $2}')
        MEMORY=$(system_profiler SPHardwareDataType | awk -F': ' '/Memory/ {print $2}')
        UPTIME=$(uptime | awk -F'up ' '{print $2}' | awk -F', ' '{print $1}')
        
        echo "{\"os_version\": \"$OS\", \"model\": \"$MODEL\", \"processor\": \"$PROCESSOR\", \"memory\": \"$MEMORY\", \"uptime\": \"$UPTIME\"}"
        ;;
    "disk")
        # 磁盘使用情况
        df -h / | awk 'NR==2 {print "{\"total\":\""$2"\",\"used\":\""$3"\",\"available\":\""$4"\",\"percentage\":\""$5"\"}"}'
        ;;
    "battery")
        # 电池信息
        BATTERY_PERCENT=$(pmset -g batt | grep -o '[0-9]*%' | tr -d '%')
        BATTERY_STATUS=$(pmset -g batt | awk -F';' '{print $2}' | awk '{print $1}')
        echo "{\"percentage\": $BATTERY_PERCENT, \"status\": \"$BATTERY_STATUS\"}"
        ;;
    "network")
        # 网络信息
        IP=$(ipconfig getifaddr en0 2>/dev/null || echo "N/A")
        EXTERNAL_IP=$(curl -s --max-time 3 https://api.ipify.org 2>/dev/null || echo "N/A")
        echo "{\"local_ip\": \"$IP\", \"external_ip\": \"$EXTERNAL_IP\"}"
        ;;
    "cpu")
        # CPU使用率
        CPU=$(top -l 1 | awk -F', ' '/CPU usage/ {print $1}' | awk '{print $3}' | tr -d '%')
        echo "{\"usage\": \"$CPU%\"}"
        ;;
    "memory")
        # 内存使用情况
        MEMORY_INFO=$(vm_stat | awk 'NR==3{print "active="$2} NR==4{print "inactive="$2} NR==5{print "wired="$2} NR==2{print "free="$2}' | tr -d '.')
        echo "{$MEMORY_INFO}"
        ;;
    *)
        echo "{\"error\": \"Unknown type: $TYPE. Use 'overview', 'disk', 'battery', 'network', 'cpu', or 'memory'\"}" >&2
        exit 1
        ;;
esac
