# 故障排除指南

本指南帮助您诊断和解决使用本地日志查看工具时可能遇到的问题。

## 目录

- [常见问题](#常见问题)
- [启动问题](#启动问题)
- [连接问题](#连接问题)
- [性能问题](#性能问题)
- [功能问题](#功能问题)
- [调试工具](#调试工具)
- [获取帮助](#获取帮助)

## 常见问题

### Q1: 服务无法启动，提示端口被占用

**错误信息**:
```
Error: listen tcp :8080: bind: address already in use
```

**解决方案**:

1. 检查端口占用情况：
```bash
# Linux/macOS
netstat -tlnp | grep 8080
lsof -i :8080

# Windows
netstat -ano | findstr :8080
```

2. 终止占用端口的进程：
```bash
# Linux/macOS
kill -9 <PID>

# Windows
taskkill /PID <PID> /F
```

3. 或者使用不同端口：
```bash
./logviewer -port 9090
```

### Q2: 无法访问 Web 界面

**症状**: 浏览器显示"无法访问此网站"或连接超时

**解决方案**:

1. 检查服务是否正在运行：
```bash
# 检查进程
ps aux | grep logviewer

# 检查端口监听
netstat -tlnp | grep 8080
```

2. 检查防火墙设置：
```bash
# Ubuntu/Debian
sudo ufw status
sudo ufw allow 8080

# CentOS/RHEL
sudo firewall-cmd --list-ports
sudo firewall-cmd --add-port=8080/tcp --permanent
sudo firewall-cmd --reload

# macOS
sudo pfctl -sr | grep 8080
```

3. 检查服务绑定地址：
```bash
# 确保绑定到 0.0.0.0 而不是 127.0.0.1
./logviewer -host 0.0.0.0 -port 8080
```

### Q3: 日志文件列表为空

**症状**: Web 界面显示"没有找到日志文件"

**解决方案**:

1. 检查日志路径是否存在：
```bash
ls -la /var/log
ls -la ./logs
```

2. 检查文件权限：
```bash
# 确保 logviewer 用户有读取权限
sudo chmod 644 /var/log/*.log
sudo chown logviewer:logviewer /var/log/*.log
```

3. 检查配置文件中的路径设置：
```yaml
server:
  logPaths:
    - "/var/log"
    - "./logs"
```

4. 使用绝对路径：
```bash
./logviewer -logs "/absolute/path/to/logs"
```

### Q4: 搜索功能不工作

**症状**: 搜索没有返回结果或返回错误

**解决方案**:

1. 检查搜索语法：
```bash
# 简单关键词搜索
error

# 正则表达式搜索（需要启用正则选项）
\d{4}-\d{2}-\d{2}
```

2. 检查时间格式：
```bash
# 正确的时间格式
2024-01-01T10:00:00Z
2024-01-01 10:00:00
```

3. 验证日志级别过滤：
```bash
# 确保选择了正确的日志级别
ERROR,WARN,INFO,DEBUG
```

### Q5: 实时更新不工作

**症状**: 新的日志条目不会自动显示

**解决方案**:

1. 检查 WebSocket 连接：
```javascript
// 在浏览器控制台中检查
console.log(ws.readyState); // 应该是 1 (OPEN)
```

2. 检查文件监控权限：
```bash
# 确保可以监控文件变化
inotifywait -m /var/log/app.log
```

3. 检查浏览器控制台错误：
```
F12 -> Console -> 查看 WebSocket 相关错误
```

4. 重新连接 WebSocket：
```javascript
// 刷新页面或手动重连
location.reload();
```

## 启动问题

### 权限错误

**错误信息**:
```
Error: permission denied
```

**解决方案**:

1. 检查二进制文件权限：
```bash
chmod +x logviewer
```

2. 检查配置文件权限：
```bash
chmod 644 config.yaml
```

3. 以正确用户运行：
```bash
# 创建专用用户
sudo useradd -r -s /bin/false logviewer
sudo -u logviewer ./logviewer
```

### 配置文件错误

**错误信息**:
```
Error: yaml: unmarshal errors
```

**解决方案**:

1. 验证 YAML 语法：
```bash
# 使用在线 YAML 验证器或
python -c "import yaml; yaml.safe_load(open('config.yaml'))"
```

2. 检查缩进和格式：
```yaml
# 正确格式
server:
  host: "0.0.0.0"
  port: 8080
```

3. 使用默认配置：
```bash
./logviewer -generate-config > config.yaml
```

### 依赖问题

**错误信息**:
```
Error: shared library not found
```

**解决方案**:

1. 检查系统依赖：
```bash
# Linux
ldd logviewer

# macOS
otool -L logviewer
```

2. 安装缺失的库：
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install libc6

# CentOS/RHEL
sudo yum install glibc
```

## 连接问题

### 网络连接超时

**症状**: 客户端连接超时或断开

**解决方案**:

1. 检查网络连通性：
```bash
ping <server-ip>
telnet <server-ip> 8080
```

2. 检查 MTU 设置：
```bash
# 调整 MTU 大小
sudo ip link set dev eth0 mtu 1400
```

3. 检查代理设置：
```bash
# 清除代理设置
unset http_proxy https_proxy
```

### SSL/TLS 问题

**错误信息**:
```
Error: x509: certificate signed by unknown authority
```

**解决方案**:

1. 更新 CA 证书：
```bash
# Ubuntu/Debian
sudo apt-get update && sudo apt-get install ca-certificates

# CentOS/RHEL
sudo yum update ca-certificates
```

2. 添加自签名证书：
```bash
# 将证书添加到信任存储
sudo cp server.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates
```

### 负载均衡问题

**症状**: 请求分发不均匀或某些实例无响应

**解决方案**:

1. 检查后端实例状态：
```bash
# 检查所有实例
for port in 8080 8081 8082; do
  curl -s http://localhost:$port/api/health
done
```

2. 检查负载均衡器配置：
```nginx
# Nginx 配置检查
nginx -t
```

3. 查看负载均衡器日志：
```bash
tail -f /var/log/nginx/error.log
```

## 性能问题

### 高内存使用

**症状**: 内存使用率持续增长

**解决方案**:

1. 监控内存使用：
```bash
# 实时监控
top -p $(pgrep logviewer)

# 详细内存信息
cat /proc/$(pgrep logviewer)/status | grep -E "(VmRSS|VmSize)"
```

2. 调整缓存设置：
```yaml
server:
  cacheSize: 50  # 减少缓存大小
  maxFileSize: 536870912  # 限制文件大小为 512MB
```

3. 启用内存分析：
```bash
# 使用 pprof 分析内存
go tool pprof http://localhost:8080/debug/pprof/heap
```

### 高 CPU 使用

**症状**: CPU 使用率过高

**解决方案**:

1. 分析 CPU 使用：
```bash
# 查看 CPU 使用情况
htop
perf top -p $(pgrep logviewer)
```

2. 优化搜索查询：
```bash
# 避免复杂的正则表达式
# 使用时间范围限制搜索
# 减少搜索结果数量
```

3. 启用 CPU 分析：
```bash
go tool pprof http://localhost:8080/debug/pprof/profile
```

### 磁盘 I/O 问题

**症状**: 响应缓慢，磁盘使用率高

**解决方案**:

1. 监控磁盘 I/O：
```bash
# Linux
iostat -x 1
iotop

# macOS
sudo fs_usage -w -f filesystem
```

2. 优化文件访问：
```bash
# 使用 SSD 存储日志文件
# 增加系统缓存
echo 3 > /proc/sys/vm/drop_caches
```

3. 调整读取策略：
```yaml
server:
  readBufferSize: 65536  # 增加读取缓冲区
  maxConcurrentReads: 10  # 限制并发读取
```

## 功能问题

### 日志解析错误

**症状**: 日志格式显示不正确或解析失败

**解决方案**:

1. 检查日志格式：
```bash
# 查看日志文件前几行
head -n 10 /var/log/app.log
```

2. 手动指定解析器：
```yaml
parsers:
  - name: "custom"
    pattern: "^(?P<timestamp>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<level>\\w+) (?P<message>.*)"
```

3. 验证时间戳格式：
```bash
# 常见时间格式
2024-01-01 10:00:00
2024-01-01T10:00:00Z
01/Jan/2024:10:00:00 +0000
```

### WebSocket 连接问题

**症状**: 实时更新断断续续或完全不工作

**解决方案**:

1. 检查 WebSocket 支持：
```javascript
// 在浏览器控制台中测试
if (window.WebSocket) {
  console.log("WebSocket is supported");
} else {
  console.log("WebSocket is not supported");
}
```

2. 检查代理配置：
```nginx
# Nginx WebSocket 代理配置
location /ws {
    proxy_pass http://backend;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

3. 调整超时设置：
```yaml
server:
  websocketTimeout: "60s"
  pingInterval: "30s"
```

### 搜索结果不准确

**症状**: 搜索结果缺失或包含不相关内容

**解决方案**:

1. 检查搜索索引：
```bash
# 重建搜索索引
curl -X POST http://localhost:8080/api/reindex
```

2. 验证搜索语法：
```bash
# 精确匹配
"exact phrase"

# 通配符搜索
error*

# 正则表达式
\b(error|fail|exception)\b
```

3. 检查文件编码：
```bash
# 检查文件编码
file -bi /var/log/app.log

# 转换编码
iconv -f ISO-8859-1 -t UTF-8 input.log > output.log
```

## 调试工具

### 启用调试模式

```bash
# 启用详细日志
./logviewer -debug -log-level debug

# 启用性能分析
./logviewer -pprof -pprof-addr localhost:6060
```

### 日志分析

```bash
# 查看应用日志
tail -f /var/log/logviewer.log

# 过滤错误信息
grep -i error /var/log/logviewer.log

# 分析访问模式
awk '{print $1}' /var/log/nginx/access.log | sort | uniq -c | sort -nr
```

### 网络诊断

```bash
# 检查网络连接
ss -tlnp | grep 8080

# 抓包分析
sudo tcpdump -i any -w capture.pcap port 8080

# 测试 API 响应时间
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:8080/api/health
```

### 性能分析

```bash
# CPU 分析
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# 内存分析
go tool pprof http://localhost:8080/debug/pprof/heap

# 协程分析
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

### 配置验证

```bash
# 验证配置文件
./logviewer -config config.yaml -check-config

# 生成默认配置
./logviewer -generate-config

# 测试配置
./logviewer -config config.yaml -dry-run
```

## 获取帮助

### 收集诊断信息

在寻求帮助时，请收集以下信息：

```bash
#!/bin/bash
# 诊断信息收集脚本

echo "=== 系统信息 ==="
uname -a
cat /etc/os-release

echo "=== 应用版本 ==="
./logviewer -version

echo "=== 配置信息 ==="
cat config.yaml

echo "=== 进程信息 ==="
ps aux | grep logviewer

echo "=== 网络信息 ==="
netstat -tlnp | grep 8080

echo "=== 日志信息 ==="
tail -n 50 /var/log/logviewer.log

echo "=== 磁盘空间 ==="
df -h

echo "=== 内存使用 ==="
free -h
```

### 提交问题

提交 Issue 时请包含：

1. **环境信息**：操作系统、版本、架构
2. **应用版本**：`./logviewer -version` 输出
3. **配置文件**：去除敏感信息的配置
4. **错误信息**：完整的错误日志
5. **重现步骤**：详细的操作步骤
6. **预期行为**：期望的正确行为

### 社区资源

- **GitHub Issues**: [项目 Issues 页面]
- **文档**: [在线文档]
- **FAQ**: [常见问题解答]
- **讨论区**: [GitHub Discussions]

### 紧急支持

对于生产环境的紧急问题：

1. 检查 [状态页面] 了解已知问题
2. 查看 [紧急修复指南]
3. 联系技术支持团队

记住，大多数问题都有简单的解决方案。按照本指南逐步排查，通常能够快速解决问题。