#!/usr/bin/env python3
"""
LoRA微调脚本 - 使用llama.cpp进行轻量化微调
适用于Qwen3:0.6b等小型模型，CPU训练
"""

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path


def check_llama_cpp():
    """检查llama.cpp是否已安装"""
    try:
        result = subprocess.run(
            ["llama-cli", "--help"],
            capture_output=True,
            timeout=5
        )
        return result.returncode == 0
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return False


def train_lora(
    base_model: str,
    train_data: str,
    lora_out: str,
    epochs: int = 3,
    batch_size: int = 2,
    learning_rate: float = 0.0002,
    threads: int = 4,
    ctx_size: int = 4096,
):
    """
    使用llama.cpp进行LoRA微调

    Args:
        base_model: 基础模型路径
        train_data: 训练数据JSONL文件路径
        lora_out: LoRA权重输出路径
        epochs: 训练轮数
        batch_size: 批次大小
        learning_rate: 学习率
        threads: 使用的线程数
        ctx_size: 上下文大小
    """
    # 检查文件是否存在
    if not os.path.exists(base_model):
        print(f"Error: Base model not found: {base_model}")
        return False

    if not os.path.exists(train_data):
        print(f"Error: Training data not found: {train_data}")
        return False

    # 创建输出目录
    os.makedirs(os.path.dirname(lora_out), exist_ok=True)

    # 构建命令
    cmd = [
        "llama-cli",
        "finetune",
        "--model", base_model,
        "--lora-out", lora_out,
        "--train-data", train_data,
        "--epochs", str(epochs),
        "--batch-size", str(batch_size),
        "--learning-rate", str(learning_rate),
        "--threads", str(threads),
        "--ctx-size", str(ctx_size),
        "--use-ckpt",  # 使用检查点，支持断点续训
    ]

    print("=" * 60)
    print("Starting LoRA Fine-tuning with llama.cpp")
    print("=" * 60)
    print(f"Base model: {base_model}")
    print(f"Training data: {train_data}")
    print(f"LoRA output: {lora_out}")
    print(f"Epochs: {epochs}")
    print(f"Batch size: {batch_size}")
    print(f"Learning rate: {learning_rate}")
    print(f"Threads: {threads}")
    print(f"Context size: {ctx_size}")
    print("=" * 60)

    # 执行训练
    try:
        result = subprocess.run(cmd, check=True)
        print("=" * 60)
        print("LoRA fine-tuning completed successfully!")
        print("=" * 60)
        return True
    except subprocess.CalledProcessError as e:
        print(f"Error during training: {e}")
        return False
    except KeyboardInterrupt:
        print("\nTraining interrupted by user")
        return False


def convert_to_gguf(lora_weights: str, base_model: str, output_path: str):
    """
    将LoRA权重合并到基础模型并转换为GGUF格式

    Args:
        lora_weights: LoRA权重文件路径
        base_model: 基础模型路径
        output_path: 输出GGUF文件路径
    """
    # TODO: 实现LoRA权重合并逻辑
    # 这需要使用llama.cpp的工具或自己实现
    print(f"Merging LoRA weights to {output_path}")
    print("(This feature is not yet implemented)")
    return True


def validate_jsonl(train_data: str) -> bool:
    """验证训练数据格式"""
    try:
        with open(train_data, 'r', encoding='utf-8') as f:
            for i, line in enumerate(f, 1):
                if not line.strip():
                    continue
                try:
                    data = json.loads(line)
                    if 'prompt' not in data or 'completion' not in data:
                        print(f"Line {i}: Missing 'prompt' or 'completion' field")
                        return False
                except json.JSONDecodeError as e:
                    print(f"Line {i}: Invalid JSON - {e}")
                    return False
        return True
    except FileNotFoundError:
        print(f"File not found: {train_data}")
        return False


def main():
    parser = argparse.ArgumentParser(
        description="LoRA Fine-tuning Script for Small Models",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Basic training
  python train_lora.py --model qwen3-0.6b.gguf --data train.jsonl --output lora.gguf

  # Custom parameters
  python train_lora.py \\
    --model qwen3-0.6b.gguf \\
    --data train.jsonl \\
    --output lora.gguf \\
    --epochs 5 \\
    --batch-size 4 \\
    --threads 8

  # Validate training data only
  python train_lora.py --data train.jsonl --validate-only
        """
    )

    parser.add_argument(
        "--model",
        required=True,
        help="Base model path (GGUF format)"
    )
    parser.add_argument(
        "--data",
        required=True,
        help="Training data file (JSONL format)"
    )
    parser.add_argument(
        "--output",
        required=True,
        help="Output path for LoRA weights"
    )
    parser.add_argument(
        "--epochs",
        type=int,
        default=3,
        help="Number of training epochs (default: 3)"
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=2,
        help="Batch size for training (default: 2)"
    )
    parser.add_argument(
        "--learning-rate",
        type=float,
        default=0.0002,
        help="Learning rate (default: 0.0002)"
    )
    parser.add_argument(
        "--threads",
        type=int,
        default=4,
        help="Number of threads to use (default: 4)"
    )
    parser.add_argument(
        "--ctx-size",
        type=int,
        default=4096,
        help="Context size (default: 4096)"
    )
    parser.add_argument(
        "--validate-only",
        action="store_true",
        help="Only validate training data format"
    )

    args = parser.parse_args()

    # 检查llama.cpp
    if not check_llama_cpp():
        print("Error: llama.cpp not found!")
        print("Please install llama.cpp first:")
        print("  git clone https://github.com/ggerganov/llama.cpp")
        print("  cd llama.cpp")
        print("  make")
        print("\nThen add llama-cli to your PATH or copy it to /usr/local/bin")
        sys.exit(1)

    # 验证训练数据
    print(f"Validating training data: {args.data}")
    if not validate_jsonl(args.data):
        print("Training data validation failed!")
        sys.exit(1)
    print("✓ Training data is valid")

    if args.validate_only:
        print("Validation complete!")
        sys.exit(0)

    # 执行训练
    success = train_lora(
        base_model=args.model,
        train_data=args.data,
        lora_out=args.output,
        epochs=args.epochs,
        batch_size=args.batch_size,
        learning_rate=args.learning_rate,
        threads=args.threads,
        ctx_size=args.ctx_size,
    )

    if success:
        print("\nTraining completed successfully!")
        print(f"LoRA weights saved to: {args.output}")
        sys.exit(0)
    else:
        print("\nTraining failed!")
        sys.exit(1)


if __name__ == "__main__":
    main()
