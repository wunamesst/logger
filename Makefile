.PHONY: build clean test run dev build-frontend build-all release

# 应用名称
APP_NAME = logviewer

# 构建目录
BUILD_DIR = build

# 版本信息
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 构建标志
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.commit=$(GIT_COMMIT) -X main.date=$(BUILD_TIME) -s -w"

# 检测操作系统
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
    BUILD_SCRIPT = ./scripts/build.sh
endif
ifeq ($(UNAME_S),Darwin)
    BUILD_SCRIPT = ./scripts/build.sh
endif
ifeq ($(OS),Windows_NT)
    BUILD_SCRIPT = scripts\build.bat
endif

# 默认目标
all: build

# 使用构建脚本构建
build:
	@$(BUILD_SCRIPT) all

# 构建前端
build-frontend:
	@$(BUILD_SCRIPT) frontend

# 构建所有平台
build-all:
	@$(BUILD_SCRIPT) all-platforms

# 发布构建（优化版本）
release: export CGO_ENABLED=0
release:
	@echo "构建发布版本..."
	@$(BUILD_SCRIPT) all-platforms
	@echo "创建发布包..."
	@mkdir -p $(BUILD_DIR)/release
	@for file in $(BUILD_DIR)/$(APP_NAME)-*; do \
		if [ -f "$$file" ]; then \
			basename=$$(basename "$$file"); \
			platform=$$(echo "$$basename" | sed 's/$(APP_NAME)-//'); \
			echo "打包 $$basename..."; \
			if [[ "$$basename" == *".exe" ]]; then \
				platform=$${platform%.exe}; \
				tmpdir=$$(mktemp -d); \
				cp "$$file" "$$tmpdir/$(APP_NAME).exe"; \
				cp README.md config.example.yaml "$$tmpdir/"; \
				(cd "$$tmpdir" && zip -r - .) > "$(BUILD_DIR)/release/$(APP_NAME)-$$platform.zip"; \
				rm -rf "$$tmpdir"; \
			else \
				tmpdir=$$(mktemp -d); \
				cp "$$file" "$$tmpdir/$(APP_NAME)"; \
				cp README.md config.example.yaml "$$tmpdir/"; \
				tar -czf "$(BUILD_DIR)/release/$(APP_NAME)-$$platform.tar.gz" -C "$$tmpdir" .; \
				rm -rf "$$tmpdir"; \
			fi; \
		fi; \
	done
	@echo "发布包创建完成: $(BUILD_DIR)/release/"

# 快速构建（仅当前平台，跳过前端如果已存在）
quick:
	@echo "快速构建 $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@if [ ! -f "web/index.html" ]; then \
		echo "前端文件不存在，构建前端..."; \
		$(BUILD_SCRIPT) frontend; \
	fi
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) cmd/logviewer/main.go
	@echo "快速构建完成: $(BUILD_DIR)/$(APP_NAME)"

# 测试部署
test-deployment:
	@./scripts/test-deployment.sh all

# 测试二进制文件
test-binaries:
	@./scripts/test-deployment.sh all-binaries

# 清理（使用构建脚本）
clean:
	@$(BUILD_SCRIPT) clean

# 运行应用
run:
	go run cmd/logviewer/main.go

# 开发模式运行
dev:
	go run cmd/logviewer/main.go -logs "./logs" -port 8080

# 运行测试
test:
	go test -v ./...

# 运行测试并生成覆盖率报告
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# 格式化代码
fmt:
	go fmt ./...

# 代码检查
lint:
	golangci-lint run

# 清理构建文件 (legacy)
clean-legacy:
	@echo "清理构建文件..."
	rm -rf $(BUILD_DIR)
	rm -rf web
	rm -f coverage.out coverage.html

# 安装依赖
deps:
	go mod download
	go mod tidy

# 生成模拟文件
generate:
	go generate ./...

# 帮助信息
help:
	@echo "可用的命令:"
	@echo "  build            - 构建应用（包含前端）"
	@echo "  build-frontend   - 仅构建前端"
	@echo "  build-all        - 构建所有平台版本"
	@echo "  release          - 构建发布版本并打包"
	@echo "  quick            - 快速构建（跳过前端如果已存在）"
	@echo "  run              - 运行应用"
	@echo "  dev              - 开发模式运行"
	@echo "  test             - 运行测试"
	@echo "  test-coverage    - 运行测试并生成覆盖率报告"
	@echo "  test-deployment  - 测试部署"
	@echo "  test-binaries    - 测试二进制文件"
	@echo "  fmt              - 格式化代码"
	@echo "  lint             - 代码检查"
	@echo "  clean            - 清理构建文件"
	@echo "  deps             - 安装依赖"
	@echo "  generate         - 生成模拟文件"
	@echo "  help             - 显示帮助信息"