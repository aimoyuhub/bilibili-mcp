#!/bin/bash

# bilibili-mcp 安装设置脚本

set -e

echo "🚀 bilibili-mcp 安装设置"
echo "======================="

# 检查 Go 版本
echo "📋 检查环境..."
if ! command -v go &> /dev/null; then
    echo "❌ 未找到 Go，请先安装 Go 1.21 或更高版本"
    echo "   下载地址: https://golang.org/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "✅ Go 版本: $GO_VERSION"

# 检查配置文件
echo ""
echo "📝 检查配置文件..."
if [ ! -f "config.yaml" ]; then
    echo "⚠️  未找到 config.yaml，从示例文件创建..."
    cp config.example.yaml config.yaml
    echo "✅ 已创建 config.yaml"
    echo "   请根据需要编辑此文件"
else
    echo "✅ config.yaml 已存在"
fi

# 创建必要目录
echo ""
echo "📁 创建必要目录..."
mkdir -p logs cookies dist
echo "✅ 目录创建完成"

# 安装依赖
echo ""
echo "📦 安装 Go 依赖..."
go mod download
go mod tidy
echo "✅ Go 依赖安装完成"

# 安装 Playwright
echo ""
echo "🎭 安装 Playwright 浏览器..."
echo "   这可能需要几分钟时间..."
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium
if [ $? -eq 0 ]; then
    echo "✅ Playwright 安装完成"
else
    echo "⚠️  Playwright 安装失败，但不影响基本功能"
    echo "   可以稍后手动运行: make install-playwright"
fi

# 构建项目
echo ""
echo "🔨 构建项目..."
make build
if [ $? -eq 0 ]; then
    echo "✅ 项目构建完成"
else
    echo "❌ 项目构建失败"
    exit 1
fi

# 完成
echo ""
echo "🎉 安装完成！"
echo ""
echo "📋 下一步操作:"
echo "1. 登录 B站账号:"
echo "   ./bilibili-login"
echo ""
echo "2. 启动 MCP 服务:"
echo "   ./bilibili-mcp"
echo ""
echo "3. 在 AI 客户端中配置 MCP 服务地址:"
echo "   http://localhost:18666/mcp"
echo ""
echo "📖 更多信息请查看 README.md"
