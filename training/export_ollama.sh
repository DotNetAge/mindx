#!/bin/bash
# 将微调后的模型导出为 Ollama 可用的 GGUF 格式

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="${SCRIPT_DIR}/.venv"

MODEL_DIR="${1:-}"
MODEL_NAME="${2:-personalized-mindx}"

if [ -z "$MODEL_DIR" ]; then
    echo "用法: $0 <模型目录> [模型名称]"
    echo "示例: $0 ./output/finetune_20260214_020000/merged my-personal-mindx"
    exit 1
fi

if [ ! -d "$MODEL_DIR" ]; then
    echo "错误: 模型目录不存在: $MODEL_DIR"
    exit 1
fi

echo "=========================================="
echo "导出模型到 Ollama"
echo "=========================================="
echo "模型目录: $MODEL_DIR"
echo "模型名称: $MODEL_NAME"
echo "=========================================="

# 检查 llama.cpp 是否可用
if ! command -v llama-quantize &> /dev/null; then
    echo ""
    echo "警告: 未找到 llama.cpp 工具"
    echo ""
    echo "请安装 llama.cpp:"
    echo "  git clone https://github.com/ggerganov/llama.cpp"
    echo "  cd llama.cpp && make"
    echo "  cp llama-quantize /usr/local/bin/"
    echo ""
    echo "或者使用 Python 转换:"
    echo "  pip install llama-cpp-python"
    echo "  python -c \"from llama_cpp import convert; convert('$MODEL_DIR', '${MODEL_NAME}.gguf', 'q4_k_m')\""
    exit 1
fi

# 创建临时目录
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# 转换为 GGUF
echo ""
echo "转换为 GGUF 格式..."
python3 -c "
from llama_cpp import Llama
import os
model_path = '$MODEL_DIR'
output_path = '$TEMP_DIR/model.gguf'
print(f'Converting {model_path} to {output_path}...')
"

# 如果上面的方法不行，使用 llama.cpp 的 convert 脚本
if [ ! -f "$TEMP_DIR/model.gguf" ]; then
    echo "使用 llama.cpp 转换..."
    if command -v python3 &> /dev/null; then
        CONVERT_SCRIPT=$(find /usr -name "convert.py" -path "*/llama.cpp/*" 2>/dev/null | head -1)
        if [ -n "$CONVERT_SCRIPT" ]; then
            python3 "$CONVERT_SCRIPT" "$MODEL_DIR" --outfile "$TEMP_DIR/model.gguf" --outtype q4_k_m
        fi
    fi
fi

# 量化
echo ""
echo "量化模型..."
GGUF_FILE="${SCRIPT_DIR}/${MODEL_NAME}.gguf"
llama-quantize "$TEMP_DIR/model.gguf" "$GGUF_FILE" q4_k_m

# 创建 Modelfile
echo ""
echo "创建 Modelfile..."
MODELFILE="${SCRIPT_DIR}/Modelfile.${MODEL_NAME}"
cat > "$MODELFILE" << EOF
FROM ./${MODEL_NAME}.gguf

PARAMETER temperature 0.7
PARAMETER top_p 0.9
PARAMETER num_ctx 4096
PARAMETER stop "<|im_start|>"
PARAMETER stop "<|im_end|>"

SYSTEM \"\"\"你是一个经过个性化微调的智能助手，基于用户的对话历史学习了用户的说话风格和偏好。\"\"\"
EOF

echo ""
echo "=========================================="
echo "导出完成!"
echo "=========================================="
echo ""
echo "创建的文件:"
echo "  GGUF: $GGUF_FILE"
echo "  Modelfile: $MODELFILE"
echo ""
echo "创建 Ollama 模型:"
echo "  ollama create $MODEL_NAME -f $MODELFILE"
echo ""
echo "测试模型:"
echo "  ollama run $MODEL_NAME"
echo ""
