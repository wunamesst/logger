#!/bin/bash

# 构建脚本 - 自动化前端构建和 Go 编译
set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 获取版本信息
get_version_info() {
    VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
    BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    
    print_info "版本信息:"
    print_info "  版本: $VERSION"
    print_info "  构建时间: $BUILD_TIME"
    print_info "  Git提交: $GIT_COMMIT"
}

# 构建前端
build_frontend() {
    print_info "开始构建前端..."
    
    if [ ! -d "frontend" ]; then
        print_error "frontend 目录不存在"
        exit 1
    fi
    
    cd frontend
    
    # 检查 Node.js 版本
    if ! command -v node &> /dev/null; then
        print_error "Node.js 未安装"
        exit 1
    fi
    
    NODE_VERSION=$(node --version)
    print_info "Node.js 版本: $NODE_VERSION"
    
    # 安装依赖
    if [ ! -d "node_modules" ]; then
        print_info "安装前端依赖..."
        npm install
    else
        print_info "前端依赖已存在，跳过安装"
    fi
    
    # 构建前端
    print_info "构建 Vue.js 应用..."
    npm run build-for-go
    
    cd ..
    
    # 验证构建结果
    if [ ! -f "web/index.html" ]; then
        print_error "前端构建失败：web/index.html 不存在"
        exit 1
    fi
    
    print_success "前端构建完成"
    
    # 显示构建产物信息
    print_info "构建产物:"
    ls -la web/
}

# 构建 Go 应用
build_go() {
    local target_os=${1:-$(go env GOOS)}
    local target_arch=${2:-$(go env GOARCH)}
    local output_name=${3:-"logviewer"}
    
    print_info "构建 Go 应用 ($target_os/$target_arch)..."
    
    # 准备嵌入文件
    prepare_embed_files
    
    # 创建构建目录
    mkdir -p build
    
    # 设置输出文件名
    if [ "$target_os" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    if [ "$target_os" != "$(go env GOOS)" ] || [ "$target_arch" != "$(go env GOARCH)" ]; then
        output_name="${output_name}-${target_os}-${target_arch}"
        if [ "$target_os" = "windows" ]; then
            output_name="${output_name}.exe"
        fi
    fi
    
    # 构建
    CGO_ENABLED=0 GOOS=$target_os GOARCH=$target_arch go build \
        -ldflags "-X main.version=$VERSION -X main.commit=$GIT_COMMIT -X main.date=$BUILD_TIME -s -w" \
        -o "build/$output_name" cmd/logviewer/main.go
    
    # 验证构建结果
    if [ ! -f "build/$output_name" ]; then
        print_error "Go 应用构建失败"
        exit 1
    fi
    
    # 清理嵌入文件
    clean_embed
    
    # 显示文件信息
    file_size=$(ls -lh "build/$output_name" | awk '{print $5}')
    print_success "Go 应用构建完成: build/$output_name ($file_size)"
}

# 准备嵌入文件
prepare_embed_files() {
    print_info "准备静态文件嵌入..."
    
    # 确保 web 目录存在
    if [ ! -d "web" ]; then
        print_error "web 目录不存在，请先构建前端"
        exit 1
    fi
    
    # 复制 web 目录到 cmd/logviewer 用于嵌入
    if [ -d "cmd/logviewer/web" ]; then
        rm -rf cmd/logviewer/web
    fi
    cp -r web cmd/logviewer/
    
    print_info "静态文件准备完成"
}

# 构建所有平台
build_all_platforms() {
    print_info "构建所有平台版本..."
    
    # 准备嵌入文件（只需要一次）
    prepare_embed_files
    
    # 定义目标平台
    platforms=(
        "linux/amd64"
        "linux/arm64"
        "darwin/amd64"
        "darwin/arm64"
        "windows/amd64"
    )
    
    for platform in "${platforms[@]}"; do
        IFS='/' read -r os arch <<< "$platform"
        
        # 设置输出文件名
        output_name="logviewer-${os}-${arch}"
        if [ "$os" = "windows" ]; then
            output_name="${output_name}.exe"
        fi
        
        print_info "构建 $platform..."
        
        # 构建
        CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build \
            -ldflags "-X main.version=$VERSION -X main.commit=$GIT_COMMIT -X main.date=$BUILD_TIME -s -w" \
            -o "build/$output_name" cmd/logviewer/main.go
        
        if [ -f "build/$output_name" ]; then
            file_size=$(ls -lh "build/$output_name" | awk '{print $5}')
            print_success "$platform 构建完成 ($file_size)"
        else
            print_error "$platform 构建失败"
        fi
    done
    
    # 清理嵌入文件
    clean_embed
    
    print_success "所有平台构建完成"
    print_info "构建产物:"
    ls -la build/
}

# 清理构建文件
clean() {
    print_info "清理构建文件..."
    rm -rf build/
    rm -rf web/
    rm -rf cmd/logviewer/web/
    print_success "清理完成"
}

# 清理嵌入文件
clean_embed() {
    if [ -d "cmd/logviewer/web" ]; then
        rm -rf cmd/logviewer/web
        print_info "清理嵌入文件完成"
    fi
}

# 运行测试
run_tests() {
    print_info "运行 Go 测试..."
    go test -v ./...
    
    print_info "运行前端测试..."
    cd frontend
    npm run test:unit -- --run
    cd ..
    
    print_success "所有测试通过"
}

# 显示帮助信息
show_help() {
    echo "构建脚本使用说明:"
    echo ""
    echo "用法: $0 [命令] [选项]"
    echo ""
    echo "命令:"
    echo "  frontend          仅构建前端"
    echo "  go [os] [arch]    构建 Go 应用 (默认: 当前平台)"
    echo "  all               构建前端和当前平台的 Go 应用"
    echo "  all-platforms     构建前端和所有平台的 Go 应用"
    echo "  clean             清理构建文件"
    echo "  test              运行测试"
    echo "  help              显示帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 all                    # 构建前端和当前平台应用"
    echo "  $0 all-platforms          # 构建所有平台版本"
    echo "  $0 go linux amd64         # 构建 Linux amd64 版本"
    echo "  $0 frontend               # 仅构建前端"
    echo ""
    echo "环境变量:"
    echo "  VERSION               版本号 (默认: git describe)"
    echo ""
}

# 主函数
main() {
    case "${1:-all}" in
        "frontend")
            get_version_info
            build_frontend
            ;;
        "go")
            get_version_info
            build_go "$2" "$3"
            ;;
        "all")
            get_version_info
            build_frontend
            build_go
            ;;
        "all-platforms")
            get_version_info
            build_frontend
            build_all_platforms
            ;;
        "clean")
            clean
            ;;
        "test")
            run_tests
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            print_error "未知命令: $1"
            show_help
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"