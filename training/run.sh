#!/bin/bash
# 运行微调脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="${SCRIPT_DIR}/.venv"

# 默认参数
DATA_FILE="${1:-./data/training/train.jsonl}"
OUTPUT_DIR="${2:-./output}"
BASE_MODEL="${3:-Qwen/Qwen2.5-0.5B-Instruct}"
EPOCHS="${4:-3}"

# 检查虚拟环境
if [ ! -d "$VENV_DIR" ]; then
    echo "错误: 虚拟环境不存在，请先运行 ./setup.sh"
    exit 1
fi

# 激活虚拟环境
source "${VENV_DIR}/bin/activate"

# 检查数据文件
if [ ! -f "$DATA_FILE" ]; then
    echo "错误: 数据文件不存在: $DATA_FILE"
    echo ""
    echo "用法: $0 <数据文件> [输出目录] [基础模型] [轮数]"
    echo "示例: $0 ./data/training/train.jsonl ./output Qwen/Qwen2.5-0.5B-Instruct 3"
    exit 1
fi

echo "=========================================="
echo "开始微调"
echo "=========================================="
echo "数据文件: $DATA_FILE"
echo "输出目录: $OUTPUT_DIR"
echo "基础模型: $BASE_MODEL"
echo "训练轮数: $EPOCHS"
echo "=========================================="

# 运行微调
python "${SCRIPT_DIR}/finetune.py" \
    --data "$DATA_FILE" \
    --output "$OUTPUT_DIR" \
    --model "$BASE_MODEL" \
    --epochs "$EPOCHS" \
    --batch-size 1 \
    --learning-rate 2e-4 \
    --max-length 512 \
    --lora-r 8

echo ""
echo "微调完成!"
