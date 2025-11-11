#!/bin/bash

# Nyaa 爬虫构建脚本
# 用于编译release版本

set -e  # 遇到错误时退出

echo "开始构建 Nyaa 爬虫 release 版本..."

# 创建输出目录
OUTPUT_DIR="release"
mkdir -p "$OUTPUT_DIR"

# 获取当前日期作为版本号的一部分
VERSION=$(date +%Y%m%d)

# 编译主爬虫程序
echo "编译主爬虫程序..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$OUTPUT_DIR/nyaa-crawler-linux-amd64" main.go

# 编译查询工具
echo "编译查询工具..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$OUTPUT_DIR/nyaa-query-linux-amd64" tools/query_tool.go

# 为不同的平台编译（可选）
echo "编译 Windows 版本..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$OUTPUT_DIR/nyaa-crawler-windows-amd64.exe" main.go
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$OUTPUT_DIR/nyaa-query-windows-amd64.exe" tools/query_tool.go

echo "编译 macOS 版本..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "$OUTPUT_DIR/nyaa-crawler-macos-amd64" main.go
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "$OUTPUT_DIR/nyaa-query-macos-amd64" tools/query_tool.go

# 创建压缩包
echo "创建压缩包..."
cd "$OUTPUT_DIR"
tar -czf "nyaa-crawler-linux-amd64-$VERSION.tar.gz" nyaa-crawler-linux-amd64 nyaa-query-linux-amd64
tar -czf "nyaa-crawler-windows-amd64-$VERSION.tar.gz" nyaa-crawler-windows-amd64.exe nyaa-query-windows-amd64.exe
tar -czf "nyaa-crawler-macos-amd64-$VERSION.tar.gz" nyaa-crawler-macos-amd64 nyaa-query-macos-amd64

# 返回上级目录
cd ..

echo "构建完成！"
echo "输出文件位于 $OUTPUT_DIR 目录中："
echo "- Linux: nyaa-crawler-linux-amd64-$VERSION.tar.gz"
echo "- Windows: nyaa-crawler-windows-amd64-$VERSION.tar.gz"
echo "- macOS: nyaa-crawler-macos-amd64-$VERSION.tar.gz"
echo ""
echo "构建的二进制文件可以直接运行，无需安装 Go 环境。"