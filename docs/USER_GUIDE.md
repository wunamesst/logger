# 用户使用指南

本地日志查看工具是一个轻量级的日志管理和查看解决方案，专为开发环境设计。本指南将帮助您快速上手使用。

## 目录

- [快速开始](#快速开始)
- [基本功能](#基本功能)
- [高级功能](#高级功能)
- [配置选项](#配置选项)
- [故障排除](#故障排除)

## 快速开始

### 1. 下载和安装

从 [发布页面](https://github.com/your-org/logviewer/releases) 下载适合您操作系统的二进制文件：

- **Linux**: `logviewer-linux-amd64`
- **macOS**: `logviewer-darwin-amd64` (Intel) 或 `logviewer-darwin-arm64` (Apple Silicon)
- **Windows**: `logviewer-windows-amd64.exe`

### 2. 基本使用

```bash
# 使用默认配置启动（监控当前目录的 logs 文件夹）
./logviewer

# 指定日志目录
./logviewer -logs "/var/log,./app/logs"

# 指定端口
./logviewer -port 9090

# 查看帮助
./logviewer -help
```

### 3. 访问 Web 界面

启动后，打开浏览器访问：
- 本地访问：`http://localhost:8080`
- 局域网访问：`http://YOUR_IP:8080`

## 基本功能

### 文件浏览

![文件浏览器](images/file-browser.png)

左侧面板显示可用的日志文件：
- **树形结构**：按目录层级显示文件
- **文件信息**：显示文件大小和修改时间
- **状态指示**：绿点表示文件正在被监控

**操作方式**：
- 点击文件名查看内容
- 展开/折叠目录
- 文件大小和时间信息一目了然

### 日志查看

![日志查看器](images/log-viewer.png)

主要特性：
- **虚拟滚动**：支持大文件流畅查看
- **语法高亮**：自动识别日志格式并高亮显示
- **分页加载**：按需加载内容，避免浏览器卡顿
- **实时更新**：文件变化时自动显示新内容

**操作技巧**：
- 使用滚动条快速定位
- 点击行号可以复制该行内容
- 使用 `Ctrl+F` 进行页面内搜索

### 搜索和过滤

![搜索面板](images/search-panel.png)

#### 关键词搜索
```
# 搜索包含 "error" 的日志
error

# 搜索多个关键词（OR 关系）
error|warning|fail
```

#### 正则表达式搜索
启用"正则表达式"选项后：
```
# 搜索 IP 地址
\d+\.\d+\.\d+\.\d+

# 搜索邮箱地址
[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}

# 搜索时间戳
\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}
```

#### 时间范围过滤
- 选择开始时间和结束时间
- 支持多种时间格式自动识别
- 可以只设置开始时间或结束时间

#### 日志级别过滤
- **ERROR**：错误信息
- **WARN**：警告信息  
- **INFO**：一般信息
- **DEBUG**：调试信息

可以选择多个级别进行组合过滤。

### 实时监控

![实时模式](images/realtime-mode.png)

**启用实时模式**：
1. 点击"实时模式"按钮
2. 新的日志条目会自动出现
3. 页面会自动滚动到最新内容

**暂停实时模式**：
- 点击"暂停"按钮停止自动更新
- 手动滚动时会自动暂停
- 点击"恢复"继续实时更新

## 高级功能

### 多文件监控

同时监控多个日志文件：
```bash
# 监控多个目录
./logviewer -logs "/var/log,/opt/app/logs,./logs"

# 使用配置文件
./logviewer -config config.yaml
```

### 日志格式支持

工具自动识别以下格式：

#### JSON 格式
```json
{"timestamp":"2024-01-01T10:00:00Z","level":"info","message":"Application started"}
```

#### 标准应用日志
```
2024-01-01 10:00:00 INFO Application started successfully
```

#### Apache/Nginx 访问日志
```
192.168.1.100 - - [01/Jan/2024:10:00:00 +0000] "GET /api/users HTTP/1.1" 200 1234
```

#### 自定义格式
对于不识别的格式，会以纯文本形式显示，但仍支持搜索和过滤。

### 性能优化

#### 大文件处理
- 使用流式读取，支持 GB 级别文件
- 虚拟滚动技术，只渲染可见内容
- 智能缓存，提高重复访问速度

#### 搜索优化
- 增量搜索，实时显示结果
- 搜索结果高亮显示
- 支持搜索结果分页

## 配置选项

### 命令行参数

```bash
./logviewer [选项]

选项:
  -host string
        服务器绑定地址 (默认 "0.0.0.0")
  -port int
        服务器端口 (默认 8080)
  -logs string
        日志目录路径，多个路径用逗号分隔 (默认 "./logs")
  -config string
        配置文件路径
  -help
        显示帮助信息
  -version
        显示版本信息
```

### 配置文件

创建 `config.yaml` 文件：

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  logPaths:
    - "/var/log"
    - "./app/logs"
  maxFileSize: 1073741824  # 1GB
  cacheSize: 100           # 缓存条目数

logging:
  level: "info"
  file: "./logviewer.log"

security:
  enableAuth: false
  username: "admin"
  password: "password"
  allowedIPs:
    - "192.168.1.0/24"
    - "10.0.0.0/8"
```

### 环境变量

```bash
# 设置端口
export LOGVIEWER_PORT=9090

# 设置日志路径
export LOGVIEWER_LOGS="/var/log,./logs"

# 启用调试模式
export LOGVIEWER_DEBUG=true
```

## 故障排除

### 常见问题

#### 1. 服务启动失败

**问题**：`bind: address already in use`
**解决**：端口被占用，使用不同端口
```bash
./logviewer -port 9090
```

**问题**：`permission denied`
**解决**：检查文件权限
```bash
chmod +x logviewer
```

#### 2. 无法访问日志文件

**问题**：文件列表为空
**解决**：检查日志路径和权限
```bash
# 检查路径是否存在
ls -la /var/log

# 检查权限
sudo chmod 644 /var/log/*.log
```

#### 3. 实时更新不工作

**问题**：新日志不显示
**解决**：
1. 检查 WebSocket 连接状态
2. 确认文件正在被写入
3. 检查浏览器控制台错误

#### 4. 搜索结果不准确

**问题**：搜索结果缺失或错误
**解决**：
1. 检查搜索语法
2. 确认时间范围设置
3. 验证日志级别过滤

### 性能问题

#### 大文件加载慢
- 增加缓存大小：`-cache-size 200`
- 减少单页显示条目：调整前端设置
- 使用 SSD 存储日志文件

#### 内存使用过高
- 减少监控的文件数量
- 调整缓存设置
- 定期清理旧日志文件

### 网络问题

#### 局域网无法访问
1. 检查防火墙设置
2. 确认服务绑定到 `0.0.0.0`
3. 验证网络连通性

#### WebSocket 连接失败
1. 检查代理服务器设置
2. 确认浏览器支持 WebSocket
3. 查看浏览器控制台错误

### 日志收集

启用详细日志以便调试：

```bash
# 启用调试模式
./logviewer -debug

# 查看服务日志
tail -f logviewer.log
```

### 获取帮助

如果遇到问题：

1. 查看 [FAQ](FAQ.md)
2. 搜索 [Issues](https://github.com/your-org/logviewer/issues)
3. 提交新的 [Issue](https://github.com/your-org/logviewer/issues/new)

提交问题时请包含：
- 操作系统和版本
- 工具版本 (`./logviewer -version`)
- 错误信息和日志
- 重现步骤