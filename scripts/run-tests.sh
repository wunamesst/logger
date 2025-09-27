#!/bin/bash

# 测试运行脚本
# 用于运行项目的各种测试

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_message() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# 打印分隔线
print_separator() {
    echo "=================================================="
}

# 检查命令是否存在
check_command() {
    if ! command -v $1 &> /dev/null; then
        print_message $RED "错误: $1 命令未找到，请先安装"
        exit 1
    fi
}

# 运行 Go 单元测试
run_go_unit_tests() {
    print_message $BLUE "运行 Go 单元测试..."
    print_separator
    
    if go test ./internal/... -v -race -coverprofile=coverage.out; then
        print_message $GREEN "✓ Go 单元测试通过"
        
        # 生成覆盖率报告
        if [ -f coverage.out ]; then
            coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
            print_message $YELLOW "测试覆盖率: $coverage"
        fi
    else
        print_message $RED "✗ Go 单元测试失败"
        return 1
    fi
}

# 运行前端测试
run_frontend_tests() {
    print_message $BLUE "运行前端测试..."
    print_separator
    
    if [ ! -d "frontend/node_modules" ]; then
        print_message $YELLOW "安装前端依赖..."
        cd frontend && npm install && cd ..
    fi
    
    if cd frontend && npm run test:unit && cd ..; then
        print_message $GREEN "✓ 前端测试通过"
    else
        print_message $RED "✗ 前端测试失败"
        return 1
    fi
}

# 运行端到端测试
run_e2e_tests() {
    print_message $BLUE "运行端到端测试..."
    print_separator
    
    # 检查是否需要构建应用
    if [ ! -f "logviewer" ]; then
        print_message $YELLOW "构建应用程序..."
        if ! go build -o logviewer cmd/logviewer/main.go; then
            print_message $RED "构建失败"
            return 1
        fi
    fi
    
    if go test ./e2e/... -v -timeout=5m; then
        print_message $GREEN "✓ 端到端测试通过"
    else
        print_message $RED "✗ 端到端测试失败"
        return 1
    fi
}

# 运行性能基准测试
run_benchmark_tests() {
    print_message $BLUE "运行性能基准测试..."
    print_separator
    
    if go test ./benchmark/... -bench=. -benchmem -timeout=10m; then
        print_message $GREEN "✓ 性能基准测试完成"
    else
        print_message $RED "✗ 性能基准测试失败"
        return 1
    fi
}

# 运行代码质量检查
run_quality_checks() {
    print_message $BLUE "运行代码质量检查..."
    print_separator
    
    # Go 代码格式检查
    if ! gofmt -l . | grep -q .; then
        print_message $GREEN "✓ Go 代码格式正确"
    else
        print_message $RED "✗ Go 代码格式不正确，请运行 gofmt -w ."
        return 1
    fi
    
    # Go 代码静态分析
    if command -v golangci-lint &> /dev/null; then
        if golangci-lint run; then
            print_message $GREEN "✓ Go 代码静态分析通过"
        else
            print_message $RED "✗ Go 代码静态分析发现问题"
            return 1
        fi
    else
        print_message $YELLOW "警告: golangci-lint 未安装，跳过静态分析"
    fi
    
    # 前端代码检查
    if [ -d "frontend" ]; then
        if cd frontend && npm run lint && cd ..; then
            print_message $GREEN "✓ 前端代码检查通过"
        else
            print_message $RED "✗ 前端代码检查失败"
            return 1
        fi
    fi
}

# 清理测试文件
cleanup() {
    print_message $BLUE "清理测试文件..."
    rm -f coverage.out
    rm -f cpu.prof mem.prof
    rm -rf /tmp/*logviewer*
    rm -rf /tmp/*test*
    print_message $GREEN "✓ 清理完成"
}

# 显示帮助信息
show_help() {
    echo "测试运行脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  unit        运行单元测试"
    echo "  frontend    运行前端测试"
    echo "  e2e         运行端到端测试"
    echo "  benchmark   运行性能基准测试"
    echo "  quality     运行代码质量检查"
    echo "  all         运行所有测试"
    echo "  clean       清理测试文件"
    echo "  help        显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 unit              # 只运行单元测试"
    echo "  $0 all               # 运行所有测试"
    echo "  $0 unit frontend     # 运行单元测试和前端测试"
}

# 主函数
main() {
    # 检查必要的命令
    check_command go
    
    if [ $# -eq 0 ]; then
        show_help
        exit 0
    fi
    
    local failed=0
    
    for arg in "$@"; do
        case $arg in
            unit)
                run_go_unit_tests || failed=1
                print_separator
                ;;
            frontend)
                check_command npm
                run_frontend_tests || failed=1
                print_separator
                ;;
            e2e)
                run_e2e_tests || failed=1
                print_separator
                ;;
            benchmark)
                run_benchmark_tests || failed=1
                print_separator
                ;;
            quality)
                run_quality_checks || failed=1
                print_separator
                ;;
            all)
                check_command npm
                run_quality_checks || failed=1
                print_separator
                run_go_unit_tests || failed=1
                print_separator
                run_frontend_tests || failed=1
                print_separator
                run_e2e_tests || failed=1
                print_separator
                run_benchmark_tests || failed=1
                print_separator
                ;;
            clean)
                cleanup
                ;;
            help)
                show_help
                ;;
            *)
                print_message $RED "未知选项: $arg"
                show_help
                exit 1
                ;;
        esac
    done
    
    if [ $failed -eq 0 ]; then
        print_message $GREEN "🎉 所有测试都通过了！"
    else
        print_message $RED "❌ 有测试失败，请检查上面的输出"
        exit 1
    fi
}

# 运行主函数
main "$@"