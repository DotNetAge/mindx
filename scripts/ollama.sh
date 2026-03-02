#!/bin/bash

set -e

echo "Checking Ollama..."

if command -v ollama &> /dev/null; then
    echo "Ollama is already installed: $(which ollama)"
    ollama --version
    exit 0
fi

echo "Ollama not found. Installing..."

curl -fsSL https://ollama.com/install.sh | sh

echo "Ollama installed successfully!"
ollama --version

echo "Pulling models..."
ollama pull qwen3:0.6b
# ollama pull qwen3:1.7b
# model blow need GPU
# ollama pull qwen3:4b
# ollama pull qwen3:8b
# ollama pull qwen3:14b
ollama pull quentinz/bge-small-zh-v1.5