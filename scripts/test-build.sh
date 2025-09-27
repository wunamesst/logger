#!/bin/bash

# 测试构建脚本
set -e

echo "测试静态资源嵌入和构建系统..."

# 创建临时的 web 目录用于测试
mkdir -p web/assets

# 创建测试文件
cat > web/index.html << 'EOF'
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>本地日志查看工具</title>
    <link rel="stylesheet" href="/assets/test.css">
</head>
<body>
    <h1>本地日志查看工具</h1>
    <p>这是一个测试页面，用于验证静态资源嵌入功能。</p>
    <div id="version-info">
        <h2>版本信息</h2>
        <div id="build-info">加载中...</div>
    </div>
    <script src="/assets/test.js"></script>
</body>
</html>
EOF

cat > web/assets/test.css << 'EOF'
body {
    font-family: Arial, sans-serif;
    max-width: 800px;
    margin: 0 auto;
    padding: 20px;
    background-color: #f5f5f5;
}

h1 {
    color: #333;
    text-align: center;
}

#version-info {
    background: white;
    padding: 20px;
    border-radius: 8px;
    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
    margin-top: 20px;
}

#build-info {
    font-family: monospace;
    background: #f8f8f8;
    padding: 10px;
    border-radius: 4px;
    white-space: pre-wrap;
}
EOF

cat > web/assets/test.js << 'EOF'
// 获取构建信息
fetch('/api/version')
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            const buildInfo = data.data;
            document.getElementById('build-info').textContent = 
                `版本: ${buildInfo.version}\n` +
                `Git提交: ${buildInfo.commit}\n` +
                `构建时间: ${buildInfo.buildTime}\n` +
                `Go版本: ${buildInfo.goVersion}\n` +
                `系统: ${buildInfo.os}/${buildInfo.arch}`;
        }
    })
    .catch(error => {
        document.getElementById('build-info').textContent = '获取版本信息失败: ' + error.message;
    });
EOF

# 创建 favicon
cp web/assets/test.css web/favicon.ico 2>/dev/null || echo "# favicon placeholder" > web/favicon.ico

echo "测试文件创建完成"

# 测试构建
echo "开始测试构建..."

# 设置版本信息
export VERSION="test-1.0.0"

# 构建
go build -ldflags "-X main.version=$VERSION -X main.commit=test-commit -X main.date=$(date -u '+%Y-%m-%d_%H:%M:%S')" -o build/logviewer-test cmd/logviewer/main.go

if [ -f "build/logviewer-test" ]; then
    echo "构建成功！"
    echo "测试版本信息："
    ./build/logviewer-test -version
    echo ""
    echo "可以运行以下命令测试服务器："
    echo "./build/logviewer-test -port 8080 -logs ./logs"
else
    echo "构建失败！"
    exit 1
fi