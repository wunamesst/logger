# 构建系统文档

本文档描述了本地日志查看工具的构建系统，包括静态资源嵌入、跨平台编译和部署测试。

## 概述

构建系统实现了以下功能：

- **静态资源嵌入**：将前端构建产物嵌入到 Go 二进制文件中
- **跨平台编译**：支持 Linux、macOS、Windows 多平台构建
- **版本信息嵌入**：在构建时嵌入版本、提交哈希、构建时间等信息
- **自动化构建脚本**：提供 Shell 和 Batch 脚本支持
- **部署测试**：验证构建产物的功能完整性

## 构建流程

### 1. 前端构建

前端使用 Vue.js 3 + Vite 构建，输出到 `web/` 目录：

```bash
# 仅构建前端
make build-frontend
# 或
./scripts/build.sh frontend
```

### 2. 静态资源嵌入

构建过程中会将 `web/` 目录复制到 `cmd/logviewer/web/`，然后通过 Go 的 `embed` 指令嵌入到二进制文件中：

```go
//go:embed web
var StaticFiles embed.FS
```

### 3. Go 应用构建

Go 应用构建时会嵌入版本信息和静态资源：

```bash
# 构建当前平台
make build
# 或
./scripts/build.sh all

# 构建所有平台
make build-all
# 或
./scripts/build.sh all-platforms
```

## 构建命令

### Make 命令

```bash
make build            # 构建应用（包含前端）
make build-frontend   # 仅构建前端
make build-all        # 构建所有平台版本
make release          # 构建发布版本并打包
make quick            # 快速构建（跳过前端如果已存在）
make test-deployment  # 测试部署
make test-binaries    # 测试二进制文件
make clean            # 清理构建文件
```

### 构建脚本

#### Linux/macOS

```bash
./scripts/build.sh [命令] [选项]

命令:
  frontend          仅构建前端
  go [os] [arch]    构建 Go 应用 (默认: 当前平台)
  all               构建前端和当前平台的 Go 应用
  all-platforms     构建前端和所有平台的 Go 应用
  clean             清理构建文件
  test              运行测试
  help              显示帮助信息

示例:
  ./scripts/build.sh all                    # 构建前端和当前平台应用
  ./scripts/build.sh all-platforms          # 构建所有平台版本
  ./scripts/build.sh go linux amd64         # 构建 Linux amd64 版本
  ./scripts/build.sh frontend               # 仅构建前端
```

#### Windows

```batch
scripts\build.bat [命令]

命令:
  frontend          仅构建前端
  go                构建 Go 应用
  all               构建前端和 Go 应用 (默认)
  all-platforms     构建所有平台版本
  clean             清理构建文件
  help              显示帮助信息
```

## 支持的平台

构建系统支持以下平台：

- **Linux**
  - amd64 (x86_64)
  - arm64 (aarch64)
- **macOS**
  - amd64 (Intel)
  - arm64 (Apple Silicon)
- **Windows**
  - amd64 (x86_64)

## 版本信息

构建时会嵌入以下版本信息：

- **版本号**：从 Git 标签获取，或使用 "dev"
- **Git 提交**：当前提交的短哈希
- **构建时间**：UTC 时间戳
- **Go 版本**：编译时使用的 Go 版本
- **系统架构**：目标操作系统和架构

查看版本信息：

```bash
./logviewer -version
```

API 端点：

```bash
curl http://localhost:8080/api/version
```

## 构建产物

### 目录结构

```
build/
├── logviewer                    # 当前平台二进制文件
├── logviewer-linux-amd64        # Linux amd64
├── logviewer-linux-arm64        # Linux arm64
├── logviewer-darwin-amd64       # macOS Intel
├── logviewer-darwin-arm64       # macOS Apple Silicon
└── logviewer-windows-amd64.exe  # Windows amd64
```

### 发布包

使用 `make release` 创建发布包：

```
build/release/
├── logviewer-linux-amd64.tar.gz
├── logviewer-linux-arm64.tar.gz
├── logviewer-darwin-amd64.tar.gz
├── logviewer-darwin-arm64.tar.gz
└── logviewer-windows-amd64.zip
```

每个发布包包含：
- 二进制文件
- README.md
- config.example.yaml

## 部署测试

构建系统提供了完整的部署测试套件：

```bash
# 测试所有兼容的二进制文件
./scripts/test-deployment.sh all-binaries

# 测试静态资源嵌入
./scripts/test-deployment.sh static

# 运行所有测试
./scripts/test-deployment.sh all
# 或
make test-deployment
```

测试内容：

1. **版本信息测试**：验证版本信息正确嵌入
2. **帮助信息测试**：验证命令行帮助功能
3. **配置文件生成测试**：验证配置文件生成功能
4. **静态文件服务测试**：验证嵌入的静态文件正确服务
5. **API 端点测试**：验证健康检查和版本信息 API

## 开发模式

开发模式下，如果嵌入的静态文件不存在，服务器会回退到使用本地 `web/` 目录：

```go
// 如果嵌入文件不存在，尝试使用本地文件系统（开发模式）
if _, err := os.Stat("web"); err == nil {
    s.router.Static("/assets", "./web/assets")
    s.router.StaticFile("/favicon.ico", "./web/favicon.ico")
    s.router.GET("/", func(c *gin.Context) {
        c.File("./web/index.html")
    })
    return
}
```

这允许在开发时无需重新构建即可看到前端更改。

## 环境变量

构建脚本支持以下环境变量：

- `VERSION`：覆盖版本号（默认从 Git 获取）
- `CGO_ENABLED`：控制 CGO（发布构建时设为 0）

示例：

```bash
VERSION=1.0.0 ./scripts/build.sh all
```

## 故障排除

### 常见问题

1. **embed 错误**：确保 `web/` 目录存在且包含文件
2. **跨平台构建失败**：检查 Go 版本是否支持目标平台
3. **静态文件无法访问**：验证嵌入路径和服务路径匹配
4. **版本信息不正确**：检查 ldflags 参数是否正确传递

### 调试构建

启用详细输出：

```bash
# 查看构建过程
./scripts/build.sh all 2>&1 | tee build.log

# 检查嵌入文件
go list -f '{{.EmbedFiles}}' cmd/logviewer

# 验证二进制文件
file build/logviewer
strings build/logviewer | grep -E "(version|commit|date)"
```

## 性能优化

构建系统包含以下优化：

1. **静态链接**：`CGO_ENABLED=0` 生成静态链接的二进制文件
2. **符号剥离**：`-s -w` 减小二进制文件大小
3. **压缩**：发布包使用 gzip/zip 压缩
4. **缓存**：构建脚本避免重复构建前端

## 持续集成

构建脚本设计为 CI/CD 友好：

```yaml
# GitHub Actions 示例
- name: Build all platforms
  run: ./scripts/build.sh all-platforms

- name: Test deployment
  run: ./scripts/test-deployment.sh all

- name: Create release packages
  run: make release
```

## 扩展

要添加新的目标平台：

1. 在 `scripts/build.sh` 的 `platforms` 数组中添加平台
2. 更新 `scripts/test-deployment.sh` 的平台检测逻辑
3. 在 `Makefile` 的 `release` 目标中添加打包逻辑

要修改嵌入的静态文件：

1. 更新前端构建配置（`frontend/vite.config.ts`）
2. 修改服务器的静态文件服务逻辑（`internal/server/server.go`）
3. 更新构建脚本的文件复制逻辑