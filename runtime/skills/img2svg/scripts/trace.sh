#!/usr/bin/env bash
# potrace 描摹：PBM → SVG path（贝塞尔轮廓）
# 用法: bash trace.sh <输入.pbm> <输出.svg>
set -euo pipefail

POTRACE="/usr/local/bin/potrace"   # homebrew bin 不在默认 PATH，用绝对路径

if [ $# -lt 2 ]; then
  echo "用法: bash trace.sh <输入.pbm> <输出.svg>" >&2
  exit 1
fi
IN="$1"
OUT="$2"
[ -x "$POTRACE" ] || { echo "错误: 未找到 potrace（brew install potrace）" >&2; exit 1; }
[ -f "$IN" ] || { echo "错误: 输入不存在: $IN" >&2; exit 1; }

# -s 输出 SVG；-t 2 去小噪点；-O 0.2 曲线拟合容差
"$POTRACE" -s -t 2 -O 0.2 -o "$OUT" "$IN"
echo "描摹完成: ${OUT}（$(wc -c < "$OUT" | tr -d ' ') 字节，路径数 $(grep -c '<path d=' "$OUT")）"
