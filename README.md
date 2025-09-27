# 本地日志查看工具

一个轻量级的日志管理和查看解决方案，专为开发环境设计。提供类似 Kibana + Elasticsearch 的功能，但无需复杂的部署和持久化存储。

## ✨ 核心亮点

- **零依赖部署** - 单个二进制文件，开箱即用
- **高性能实时监控** - 基于 `fsnotify` 的毫秒级文件变化检测
- **智能虚拟滚动** - 轻松处理百万级日志，内存占用低
- **哈希去重算法** - O(1) 复杂度的实时日志去重，性能卓越
- **WebSocket 实时推送** - 类似 `tail -f` 的实时日志流体验
- **多格式支持** - 自动识别 JSON、纯文本、Apache/Nginx 等格式

## 功能特性

- 🚀 单二进制文件部署，无需额外依赖
- 📁 支持多目录日志文件浏览
- 🔍 强大的搜索和过滤功能（支持正则表达式）
- ⚡ 实时日志监控和更新（WebSocket + 哈希去重）
- 🎯 虚拟滚动技术，支持大文件（100MB+）
- 🌐 Web 界面，支持局域网访问
- 📱 响应式设计，支持移动设备
- 🔒 可选的安全认证功能
- 💾 智能缓存机制，减少磁盘 I/O
- 🎨 日志格式化和语法高亮

## 快速开始

> **前置要求**：Go 1.21+ 和 Node.js 16+

### 1. 克隆项目

```bash
git clone https://github.com/your-username/logger.git
cd logger
```

### 2. 一键启动（推荐）

```bash
# 第一次运行：自动构建前端和后端，然后启动应用
make dev
```

这将会：
- 自动检查并构建前端资源
- 使用测试日志数据
- 在端口 8080 启动服务
- 自动在浏览器中打开 http://localhost:8080

### 3. 手动构建和运行

如果你想更精细地控制构建过程：

```bash
# 构建完整应用（包含前端和后端）
make build

# 或者分步构建
make build-frontend  # 构建前端
make quick          # 快速构建后端

# 运行构建好的二进制文件
./build/logviewer --config config.example.yaml
```

### 4. 自定义运行

```bash
# 使用默认配置
./build/logviewer

# 指定端口和日志路径
./build/logviewer -port 9090 -logs "/var/log,./app/logs"

# 使用自定义配置文件
./build/logviewer -config your-config.yaml
```

### 5. 访问应用

打开浏览器访问 `http://localhost:8080`

## 构建选项详解

### 可用的 Make 命令

```bash
# 构建相关
make build           # 完整构建（前端+后端）
make build-frontend  # 仅构建前端（Vue.js）
make build-all       # 构建所有平台版本
make quick          # 快速构建（跳过前端如果已存在）
make release        # 构建发布版本并打包

# 运行相关
make run            # 直接运行源码
make dev            # 开发模式（使用测试数据）

# 测试相关
make test           # 运行所有测试
make test-coverage  # 测试覆盖率报告

# 代码质量
make fmt            # 格式化代码
make lint           # 代码检查

# 清理
make clean          # 清理构建文件
make help           # 显示所有可用命令
```

### 多平台构建

支持构建多个平台的二进制文件：

```bash
# 构建所有支持的平台
make build-all

# 生成的文件位于 build/ 目录：
# logviewer-linux-amd64
# logviewer-linux-arm64
# logviewer-darwin-amd64
# logviewer-darwin-arm64
# logviewer-windows-amd64.exe
```

### 发布打包

```bash
# 构建并打包发布版本
make release

# 生成的压缩包位于 build/release/ 目录
# 包含：二进制文件 + README.md + config.example.yaml
```

## 配置

### 使用配置文件

复制示例配置文件并根据需要修改：

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml 文件
./build/logviewer --config config.yaml
```

### 主要配置项

```yaml
server:
  host: "0.0.0.0"        # 绑定地址
  port: 8080             # 端口
  logPaths:              # 日志目录
    - "./logs"
    - "/var/log"
  maxFileSize: 104857600 # 最大文件大小(100MB)

logging:
  level: "info"          # 日志级别
  format: "json"         # 日志格式

security:
  enableAuth: false      # 是否启用认证
  username: ""           # 用户名（启用认证时）
  password: ""           # 密码（启用认证时）
```

### 命令行参数

命令行参数会覆盖配置文件设置：

```bash
./build/logviewer \
  -host 0.0.0.0 \
  -port 9090 \
  -logs "/var/log,./app/logs" \
  -max-file-size 52428800 \
  -log-level debug
```

## 项目结构

```
├── cmd/logviewer/          # 主程序入口
├── internal/               # 内部包
│   ├── config/            # 配置管理
│   ├── server/            # HTTP服务器
│   ├── types/             # 数据类型定义
│   └── interfaces/        # 接口定义
├── pkg/                   # 公共库
├── web/                   # 前端资源
└── config.example.yaml    # 配置文件示例
```

## 常见问题

### Q: 首次运行出现"前端文件不存在"错误？
A: 运行 `make build-frontend` 或 `make build` 来构建前端资源。

### Q: 端口被占用怎么办？
A: 使用 `-port` 参数指定其他端口：`./build/logviewer -port 9090`

### Q: 如何添加新的日志目录？
A: 修改配置文件中的 `logPaths` 或使用 `-logs` 参数：
```bash
./build/logviewer -logs "/var/log,./app/logs,/custom/logs"
```

### Q: 支持哪些日志格式？
A: 支持纯文本、JSON、通用格式（Apache、Nginx等）的日志文件。

### Q: 如何启用认证？
A: 在配置文件中设置：
```yaml
security:
  enableAuth: true
  username: "admin"
  password: "your-password"
```

## 故障排除

如果遇到问题，请检查：

1. **Go 版本**：确保使用 Go 1.21+
2. **Node.js 版本**：前端构建需要 Node.js 16+
3. **端口冲突**：确保指定的端口未被占用
4. **文件权限**：确保有读取日志文件的权限
5. **磁盘空间**：确保有足够的磁盘空间用于缓存

更详细的故障排除指南请查看 [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)

## 文档

完整的文档请查看 [docs/](docs/) 目录：

- **[用户使用指南](docs/USER_GUIDE.md)** - 详细的使用说明和功能介绍
- **[API 参考文档](docs/API_REFERENCE.md)** - 完整的 REST API 和 WebSocket API 文档
- **[部署指南](docs/DEPLOYMENT_GUIDE.md)** - 生产环境部署和配置指南
- **[故障排除指南](docs/TROUBLESHOOTING.md)** - 常见问题和解决方案
- **[开发者指南](docs/DEVELOPER_GUIDE.md)** - 开发环境设置和贡献指南

## 性能和资源使用

- **内存占用**：典型运行时约 20-50MB（取决于缓存大小）
- **文件监控**：使用 `fsnotify` 进行高效的文件变化监控
- **并发处理**：支持多个客户端同时连接和搜索
- **缓存机制**：智能缓存热点日志内容，减少磁盘 I/O

## 许可证

MIT License