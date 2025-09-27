#!/bin/bash

# 部署测试脚本
set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 测试单二进制文件部署
test_deployment() {
    local binary_path=${1:-"build/logviewer"}
    
    if [ ! -f "$binary_path" ]; then
        print_error "二进制文件不存在: $binary_path"
        exit 1
    fi
    
    print_info "测试二进制文件: $binary_path"
    
    # 测试版本信息
    print_info "测试版本信息..."
    if $binary_path -version; then
        print_success "版本信息测试通过"
    else
        print_error "版本信息测试失败"
        return 1
    fi
    
    # 测试帮助信息
    print_info "测试帮助信息..."
    if $binary_path -help > /dev/null 2>&1; then
        print_success "帮助信息测试通过"
    else
        print_error "帮助信息测试失败"
        return 1
    fi
    
    # 测试配置文件生成
    print_info "测试配置文件生成..."
    local temp_dir=$(mktemp -d)
    local current_dir=$(pwd)
    local full_binary_path="$current_dir/$binary_path"
    cd "$temp_dir"
    
    if "$full_binary_path" -generate-config; then
        if [ -f "config.yaml" ]; then
            print_success "配置文件生成测试通过"
            print_info "生成的配置文件内容:"
            head -10 config.yaml
        else
            print_error "配置文件未生成"
            return 1
        fi
    else
        print_error "配置文件生成失败"
        return 1
    fi
    
    # 清理临时目录
    cd - > /dev/null
    rm -rf "$temp_dir"
    
    print_success "单二进制文件部署测试完成"
}

# 测试所有构建的二进制文件
test_all_binaries() {
    print_info "测试所有构建的二进制文件..."
    
    local build_dir="build"
    local tested=0
    local passed=0
    
    for binary in "$build_dir"/logviewer*; do
        if [ -f "$binary" ] && [ -x "$binary" ]; then
            # 跳过 Windows 二进制文件（在 macOS/Linux 上无法执行）
            if [[ "$binary" == *.exe ]]; then
                print_info "跳过 Windows 二进制文件: $(basename "$binary")"
                continue
            fi
            
            # 跳过不兼容的架构
            local current_os=$(uname -s | tr '[:upper:]' '[:lower:]')
            local current_arch=$(uname -m)
            
            # 转换架构名称
            case "$current_arch" in
                x86_64) current_arch="amd64" ;;
                arm64|aarch64) current_arch="arm64" ;;
            esac
            
            # 检查是否为当前平台的二进制文件
            if [[ "$binary" == *"-linux-"* ]] && [[ "$current_os" != "linux" ]]; then
                print_info "跳过 Linux 二进制文件: $(basename "$binary")"
                continue
            fi
            
            if [[ "$binary" == *"-darwin-"* ]] && [[ "$current_os" != "darwin" ]]; then
                print_info "跳过 macOS 二进制文件: $(basename "$binary")"
                continue
            fi
            
            if [[ "$binary" == *"-arm64"* ]] && [[ "$current_arch" != "arm64" ]]; then
                print_info "跳过 ARM64 二进制文件: $(basename "$binary")"
                continue
            fi
            
            if [[ "$binary" == *"-amd64"* ]] && [[ "$current_arch" != "amd64" ]]; then
                print_info "跳过 AMD64 二进制文件: $(basename "$binary")"
                continue
            fi
            
            print_info "测试二进制文件: $(basename "$binary")"
            tested=$((tested + 1))
            
            if test_deployment "$binary"; then
                passed=$((passed + 1))
                print_success "$(basename "$binary") 测试通过"
            else
                print_error "$(basename "$binary") 测试失败"
            fi
            echo ""
        fi
    done
    
    print_info "测试结果: $passed/$tested 个二进制文件通过测试"
    
    if [ $passed -eq $tested ]; then
        print_success "所有二进制文件测试通过！"
        return 0
    else
        print_error "部分二进制文件测试失败"
        return 1
    fi
}

# 测试静态资源嵌入
test_static_embedding() {
    local binary_path=${1:-"build/logviewer"}
    
    print_info "测试静态资源嵌入..."
    
    # 创建临时目录
    local temp_dir=$(mktemp -d)
    local log_dir="$temp_dir/logs"
    mkdir -p "$log_dir"
    
    # 创建测试日志文件
    echo "$(date): Test log entry 1" > "$log_dir/test.log"
    echo "$(date): Test log entry 2" >> "$log_dir/test.log"
    
    # 启动服务器（后台运行）
    print_info "启动测试服务器..."
    local current_dir=$(pwd)
    local full_binary_path="$current_dir/$binary_path"
    cd "$temp_dir"
    "$full_binary_path" -port 18080 -logs "$log_dir" > server.log 2>&1 &
    local server_pid=$!
    
    # 等待服务器启动
    sleep 3
    
    # 测试服务器是否启动
    if ! kill -0 $server_pid 2>/dev/null; then
        print_error "服务器启动失败"
        cat server.log
        return 1
    fi
    
    # 测试 API 端点
    print_info "测试 API 端点..."
    
    # 测试健康检查
    if curl -s http://localhost:18080/api/health > /dev/null; then
        print_success "健康检查 API 测试通过"
    else
        print_error "健康检查 API 测试失败"
        kill $server_pid 2>/dev/null
        return 1
    fi
    
    # 测试版本信息 API
    if curl -s http://localhost:18080/api/version | grep -q "success"; then
        print_success "版本信息 API 测试通过"
    else
        print_error "版本信息 API 测试失败"
        kill $server_pid 2>/dev/null
        return 1
    fi
    
    # 测试静态文件服务
    if curl -s http://localhost:18080/ | grep -q "<!DOCTYPE html>"; then
        print_success "静态文件服务测试通过"
    else
        print_error "静态文件服务测试失败"
        print_info "服务器响应:"
        curl -s http://localhost:18080/ | head -5
        kill $server_pid 2>/dev/null
        return 1
    fi
    
    # 停止服务器
    kill $server_pid 2>/dev/null
    wait $server_pid 2>/dev/null || true
    
    # 清理
    cd - > /dev/null
    rm -rf "$temp_dir"
    
    print_success "静态资源嵌入测试完成"
}

# 主函数
main() {
    case "${1:-all}" in
        "binary")
            test_deployment "${2:-build/logviewer}"
            ;;
        "all-binaries")
            test_all_binaries
            ;;
        "static")
            test_static_embedding "${2:-build/logviewer}"
            ;;
        "all")
            test_all_binaries
            echo ""
            test_static_embedding "build/logviewer"
            ;;
        "help"|"-h"|"--help")
            echo "部署测试脚本使用说明:"
            echo ""
            echo "用法: $0 [命令] [二进制文件路径]"
            echo ""
            echo "命令:"
            echo "  binary [path]     测试指定的二进制文件"
            echo "  all-binaries      测试所有构建的二进制文件"
            echo "  static [path]     测试静态资源嵌入"
            echo "  all               运行所有测试 (默认)"
            echo "  help              显示帮助信息"
            ;;
        *)
            print_error "未知命令: $1"
            echo "使用 '$0 help' 查看帮助信息"
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"