#!/usr/bin/env python3
"""
Qwen3 LoRA 微调脚本
支持 CPU 训练（Intel 芯片）
"""

import argparse
import json
import os
import sys
from datetime import datetime
from pathlib import Path

import torch
from datasets import Dataset
from transformers import (
    AutoModelForCausalLM,
    AutoTokenizer,
    TrainingArguments,
    BitsAndBytesConfig,
)
from peft import LoraConfig, get_peft_model, TaskType
from trl import SFTTrainer


def parse_args():
    parser = argparse.ArgumentParser(description="Qwen3 LoRA 微调")
    parser.add_argument(
        "--data", type=str, required=True, help="训练数据文件路径 (JSONL)"
    )
    parser.add_argument(
        "--output", type=str, default="./output", help="输出目录"
    )
    parser.add_argument(
        "--model", type=str, default="Qwen/Qwen2.5-0.5B-Instruct", help="基础模型"
    )
    parser.add_argument(
        "--epochs", type=int, default=3, help="训练轮数"
    )
    parser.add_argument(
        "--batch-size", type=int, default=1, help="批次大小 (CPU 建议 1)"
    )
    parser.add_argument(
        "--learning-rate", type=float, default=2e-4, help="学习率"
    )
    parser.add_argument(
        "--max-length", type=int, default=512, help="最大序列长度"
    )
    parser.add_argument(
        "--lora-r", type=int, default=8, help="LoRA 秩"
    )
    parser.add_argument(
        "--export-gguf", action="store_true", help="导出 GGUF 格式"
    )
    parser.add_argument(
        "--gguf-output", type=str, default=None, help="GGUF 输出路径"
    )
    return parser.parse_args()


def load_dataset(data_path: str) -> Dataset:
    """加载 JSONL 格式的训练数据"""
    data = []
    with open(data_path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            item = json.loads(line)
            
            prompt = item.get("prompt", "")
            completion = item.get("completion", "")
            
            if prompt and completion:
                prompt = prompt.replace("用户: ", "").replace("User: ", "")
                completion = completion.replace("助手: ", "").replace("Assistant: ", "")
                
                text = f"<|im_start|>user\n{prompt}<|im_end|>\n<|im_start|>assistant\n{completion}<|im_end|>"
                data.append({"text": text})
    
    print(f"加载了 {len(data)} 条训练数据")
    return Dataset.from_list(data)


def setup_model(model_name: str, device: str):
    """加载模型和分词器"""
    print(f"加载模型: {model_name}")
    print(f"设备: {device}")
    
    tokenizer = AutoTokenizer.from_pretrained(
        model_name,
        trust_remote_code=True,
        use_fast=True,
    )
    
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token
    
    if device == "cpu":
        model = AutoModelForCausalLM.from_pretrained(
            model_name,
            trust_remote_code=True,
            torch_dtype=torch.float32,
            device_map="cpu",
        )
    else:
        bnb_config = BitsAndBytesConfig(
            load_in_4bit=True,
            bnb_4bit_quant_type="nf4",
            bnb_4bit_compute_dtype=torch.float16,
            bnb_4bit_use_double_quant=True,
        )
        model = AutoModelForCausalLM.from_pretrained(
            model_name,
            trust_remote_code=True,
            quantization_config=bnb_config,
            device_map="auto",
        )
    
    return model, tokenizer


def setup_lora(model, lora_r: int):
    """配置 LoRA"""
    lora_config = LoraConfig(
        task_type=TaskType.CAUSAL_LM,
        r=lora_r,
        lora_alpha=lora_r * 2,
        lora_dropout=0.05,
        target_modules=["q_proj", "k_proj", "v_proj", "o_proj"],
        bias="none",
    )
    
    model = get_peft_model(model, lora_config)
    model.print_trainable_parameters()
    
    return model


def train(model, tokenizer, dataset, args, output_dir: str):
    """执行训练"""
    training_args = TrainingArguments(
        output_dir=output_dir,
        num_train_epochs=args.epochs,
        per_device_train_batch_size=args.batch_size,
        gradient_accumulation_steps=4,
        learning_rate=args.learning_rate,
        weight_decay=0.01,
        warmup_ratio=0.1,
        logging_steps=10,
        save_steps=100,
        save_total_limit=2,
        fp16=False,
        bf16=False,
        gradient_checkpointing=True,
        optim="adamw_torch",
        report_to="none",
        remove_unused_columns=False,
    )
    
    trainer = SFTTrainer(
        model=model,
        args=training_args,
        train_dataset=dataset,
        tokenizer=tokenizer,
        max_seq_length=args.max_length,
        dataset_text_field="text",
        packing=False,
    )
    
    print("\n开始训练...")
    print(f"轮数: {args.epochs}")
    print(f"批次大小: {args.batch_size}")
    print(f"学习率: {args.learning_rate}")
    print(f"数据量: {len(dataset)}")
    print()
    
    trainer.train()
    
    return trainer


def export_model(model, tokenizer, output_dir: str, gguf_path: str = None):
    """导出模型"""
    merged_dir = os.path.join(output_dir, "merged")
    os.makedirs(merged_dir, exist_ok=True)
    
    print(f"\n合并 LoRA 权重...")
    merged_model = model.merge_and_unload()
    
    print(f"保存合并后的模型到: {merged_dir}")
    merged_model.save_pretrained(merged_dir, safe_serialization=True)
    tokenizer.save_pretrained(merged_dir)
    
    if gguf_path:
        print(f"\n导出 GGUF 格式需要 llama.cpp 工具")
        print(f"请手动执行以下命令:")
        print(f"  python -m llama_cpp.convert {merged_dir} --outfile {gguf_path} --outtype q4_k_m")
    
    return merged_dir


def main():
    args = parse_args()
    
    if not os.path.exists(args.data):
        print(f"错误: 数据文件不存在: {args.data}")
        sys.exit(1)
    
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    output_dir = os.path.join(args.output, f"finetune_{timestamp}")
    os.makedirs(output_dir, exist_ok=True)
    
    print("=" * 50)
    print("Qwen3 LoRA 微调")
    print("=" * 50)
    print(f"数据文件: {args.data}")
    print(f"输出目录: {output_dir}")
    print(f"基础模型: {args.model}")
    print("=" * 50)
    
    device = "cuda" if torch.cuda.is_available() else "mps" if torch.backends.mps.is_available() else "cpu"
    
    dataset = load_dataset(args.data)
    
    if len(dataset) == 0:
        print("错误: 没有有效的训练数据")
        sys.exit(1)
    
    model, tokenizer = setup_model(args.model, device)
    
    model = setup_lora(model, args.lora_r)
    
    train(model, tokenizer, dataset, args, output_dir)
    
    gguf_path = args.gguf_output or os.path.join(output_dir, "model.gguf")
    export_model(model, tokenizer, output_dir, gguf_path if args.export_gguf else None)
    
    config = {
        "base_model": args.model,
        "timestamp": timestamp,
        "epochs": args.epochs,
        "learning_rate": args.learning_rate,
        "lora_r": args.lora_r,
        "train_samples": len(dataset),
        "device": device,
    }
    with open(os.path.join(output_dir, "config.json"), "w") as f:
        json.dump(config, f, indent=2, ensure_ascii=False)
    
    print("\n" + "=" * 50)
    print("训练完成!")
    print(f"模型保存于: {output_dir}")
    print("=" * 50)


if __name__ == "__main__":
    main()
