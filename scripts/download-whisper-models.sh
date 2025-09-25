#!/bin/bash

# Whisper.cpp 模型下载脚本
# 用于下载预制模型文件

set -e

echo "🎤 Whisper.cpp 模型下载脚本"
echo "============================"

# 创建模型目录
echo "📁 创建模型目录..."
mkdir -p models
cd models

echo ""
echo "📦 下载基础模型..."

# 下载 tiny 模型 (~39MB)
if [ ! -f "ggml-tiny.bin" ]; then
    echo "⬇️  下载 ggml-tiny.bin (~39MB)..."
    curl -L -o ggml-tiny.bin "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin?download=true"
    echo "✅ ggml-tiny.bin 下载完成"
else
    echo "✅ ggml-tiny.bin 已存在"
fi

# 下载 base 模型 (~142MB)
if [ ! -f "ggml-base.bin" ]; then
    echo "⬇️  下载 ggml-base.bin (~142MB)..."
    curl -L -o ggml-base.bin "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin?download=true"
    echo "✅ ggml-base.bin 下载完成"
else
    echo "✅ ggml-base.bin 已存在"
fi

# macOS Core ML 模型
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo ""
    echo "🚀 下载 macOS Core ML 加速模型..."
    
    # 下载 tiny Core ML 模型
    if [ ! -f "ggml-tiny.en-encoder.mlmodelc.zip" ] && [ ! -d "ggml-tiny.en-encoder.mlmodelc" ]; then
        echo "⬇️  下载 ggml-tiny Core ML 模型..."
        curl -L -o ggml-tiny.en-encoder.mlmodelc.zip "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.en-encoder.mlmodelc.zip?download=true"
        echo "✅ ggml-tiny Core ML 模型下载完成"
    else
        echo "✅ ggml-tiny Core ML 模型已存在"
    fi
    
    # 下载 base Core ML 模型
    if [ ! -f "ggml-base.en-encoder.mlmodelc.zip" ] && [ ! -d "ggml-base.en-encoder.mlmodelc" ]; then
        echo "⬇️  下载 ggml-base Core ML 模型..."
        curl -L -o ggml-base.en-encoder.mlmodelc.zip "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en-encoder.mlmodelc.zip?download=true"
        echo "✅ ggml-base Core ML 模型下载完成"
    else
        echo "✅ ggml-base Core ML 模型已存在"
    fi
    
    echo ""
    echo "📦 解压 Core ML 模型..."
    
    # 解压 Core ML 模型
    if [ -f "ggml-tiny.en-encoder.mlmodelc.zip" ] && [ ! -d "ggml-tiny.en-encoder.mlmodelc" ]; then
        echo "📂 解压 ggml-tiny Core ML 模型..."
        unzip -q ggml-tiny.en-encoder.mlmodelc.zip
        echo "✅ ggml-tiny Core ML 模型解压完成"
    fi
    
    if [ -f "ggml-base.en-encoder.mlmodelc.zip" ] && [ ! -d "ggml-base.en-encoder.mlmodelc" ]; then
        echo "📂 解压 ggml-base Core ML 模型..."
        unzip -q ggml-base.en-encoder.mlmodelc.zip
        echo "✅ ggml-base Core ML 模型解压完成"
    fi
    
    # 清理 zip 文件
    echo "🧹 清理压缩文件..."
    rm -f ggml-*.mlmodelc.zip
fi

cd ..

echo ""
echo "🎉 模型下载完成！"
echo ""
echo "📋 下载的文件："
ls -la models/ | grep -E '\.(bin|mlmodelc)$' || ls -la models/ | grep ggml

echo ""
echo "⚡ 性能说明："
echo "   • ggml-tiny.bin: 最快速度，基础准确性"
echo "   • ggml-base.bin: 平衡速度和质量"
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "   • Core ML 模型: macOS 专用，性能提升 2-3 倍"
fi

echo ""
echo "🚀 现在可以运行 ./whisper-init 来初始化 Whisper.cpp"
