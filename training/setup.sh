#!/bin/bash
# 设置 Python 虚拟环境并安装依赖

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="${SCRIPT_DIR}/.venv"

echo "=========================================="
echo "设置微调环境"
echo "=========================================="

# 检查 Python 3
if ! command -v python3 &> /dev/null; then
    echo "错误: 未找到 python3，请先安装 Python 3.10+"
    exit 1
fi

PYTHON_VERSION=$(python3 --version 2>&1 | awk '{print $2}')
echo "Python 版本: $PYTHON_VERSION"

# 创建虚拟环境
if [ ! -d "$VENV_DIR" ]; then
    echo ""
    echo "创建虚拟环境: $VENV_DIR"
    python3 -m venv "$VENV_DIR"
else
    echo "虚拟环境已存在: $VENV_DIR"
fi

# 激活虚拟环境
echo ""
echo "激活虚拟环境..."
source "${VENV_DIR}/bin/activate"

# 升级 pip
echo ""
echo "升级 pip..."
pip install --upgrade pip

# 安装依赖
echo ""
echo "安装依赖..."
pip install -r "${SCRIPT_DIR}/requirements.txt"

echo ""
echo "=========================================="
echo "环境设置完成!"
echo "=========================================="
echo ""
echo "使用方法:"
echo "  1. 激活环境: source ${VENV_DIR}/bin/activate"
echo "  2. 运行微调: python ${SCRIPT_DIR}/finetune.py --data <数据文件> --output <输出目录>"
echo "  3. 退出环境: deactivate"
echo ""
