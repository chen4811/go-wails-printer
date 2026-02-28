#!/bin/bash

echo "================================"
echo "  打印服务器 - 快速启动脚本"
echo "================================"
echo ""

# 检查 Go
if ! command -v go &> /dev/null; then
    echo "❌ 未检测到 Go，请先安装 Go 1.21+"
    echo "下载地址: https://golang.org/dl/"
    exit 1
fi

echo "✅ Go 版本: $(go version)"
echo ""

# 检查 Node.js
if ! command -v node &> /dev/null; then
    echo "❌ 未检测到 Node.js，请先安装 Node.js 18+"
    echo "下载地址: https://nodejs.org/"
    exit 1
fi

echo "✅ Node.js 版本: $(node -v)"
echo ""

# 检查 Wails
if ! command -v wails &> /dev/null; then
    echo "📦 安装 Wails CLI..."
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    echo ""
fi

# 安装前端依赖
if [ ! -d "frontend/node_modules" ]; then
    echo "📦 安装前端依赖..."
    cd frontend && npm install && cd ..
    echo ""
fi

# 下载 Go 依赖
echo "📦 检查 Go 依赖..."
go mod download
echo ""

# 启动开发模式
echo "🚀 启动打印服务器..."
wails dev
