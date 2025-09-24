#!/bin/bash

# Claude Code CLI MCP 配置脚本

echo "🔧 配置 Claude Code CLI MCP"
echo "========================="

# 检查 Claude CLI 是否安装
if ! command -v claude &> /dev/null; then
    echo "❌ 未找到 Claude CLI"
    echo "   请先安装: https://github.com/anthropics/claude-cli"
    exit 1
fi

echo "✅ Claude CLI 已安装"

# 添加 MCP 服务器
echo "📡 添加 bilibili-mcp 服务器..."
claude mcp add --transport http bilibili-mcp http://localhost:18666/mcp

if [ $? -eq 0 ]; then
    echo "✅ MCP 服务器添加成功"
else
    echo "❌ MCP 服务器添加失败"
    exit 1
fi

# 列出已配置的 MCP 服务器
echo ""
echo "📋 当前已配置的 MCP 服务器:"
claude mcp list

echo ""
echo "🎉 配置完成！"
echo ""
echo "📋 使用说明:"
echo "1. 确保 bilibili-mcp 服务正在运行:"
echo "   ./bilibili-mcp"
echo ""
echo "2. 在 Claude 中使用 MCP 工具:"
echo "   claude chat"
echo "   然后在对话中使用 B站相关功能"
