#!/bin/bash

echo "=== 添加日志内容测试脚本 ==="

if [ ! -f "logs/test.log" ]; then
    echo "❌ 找不到 logs/test.log 文件"
    echo "请先运行 ./manual_test.sh 启动服务器"
    exit 1
fi

echo "当前文件内容："
echo "----------------------------------------"
cat logs/test.log
echo "----------------------------------------"
echo ""

echo "添加新内容到文件..."
echo "$(date): 手动添加的测试内容 - $(date +%s)" >> logs/test.log

echo "✅ 内容已添加"
echo ""
echo "更新后的文件内容："
echo "----------------------------------------"
cat logs/test.log
echo "----------------------------------------"
echo ""
echo "请检查服务器控制台是否有文件监听相关的日志输出"
echo "同时检查浏览器页面 http://localhost:8080 是否实时更新"