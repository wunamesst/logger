@echo off
REM 构建脚本 - Windows 版本
setlocal enabledelayedexpansion

REM 设置颜色（如果支持）
set "INFO=[INFO]"
set "SUCCESS=[SUCCESS]"
set "WARNING=[WARNING]"
set "ERROR=[ERROR]"

REM 获取版本信息
if "%VERSION%"=="" (
    for /f "tokens=*" %%i in ('git describe --tags --always --dirty 2^>nul') do set VERSION=%%i
    if "!VERSION!"=="" set VERSION=dev
)

for /f "tokens=*" %%i in ('powershell -command "Get-Date -UFormat '%%Y-%%m-%%d_%%H:%%M:%%S'"') do set BUILD_TIME=%%i

for /f "tokens=*" %%i in ('git rev-parse --short HEAD 2^>nul') do set GIT_COMMIT=%%i
if "%GIT_COMMIT%"=="" set GIT_COMMIT=unknown

echo %INFO% 版本信息:
echo %INFO%   版本: %VERSION%
echo %INFO%   构建时间: %BUILD_TIME%
echo %INFO%   Git提交: %GIT_COMMIT%

REM 构建前端
:build_frontend
echo %INFO% 开始构建前端...

if not exist "frontend" (
    echo %ERROR% frontend 目录不存在
    exit /b 1
)

cd frontend

REM 检查 Node.js
where node >nul 2>&1
if errorlevel 1 (
    echo %ERROR% Node.js 未安装
    exit /b 1
)

for /f "tokens=*" %%i in ('node --version') do set NODE_VERSION=%%i
echo %INFO% Node.js 版本: %NODE_VERSION%

REM 安装依赖
if not exist "node_modules" (
    echo %INFO% 安装前端依赖...
    npm install
    if errorlevel 1 (
        echo %ERROR% 前端依赖安装失败
        exit /b 1
    )
) else (
    echo %INFO% 前端依赖已存在，跳过安装
)

REM 构建前端
echo %INFO% 构建 Vue.js 应用...
npm run build-for-go
if errorlevel 1 (
    echo %ERROR% 前端构建失败
    exit /b 1
)

cd ..

REM 验证构建结果
if not exist "web\index.html" (
    echo %ERROR% 前端构建失败：web\index.html 不存在
    exit /b 1
)

echo %SUCCESS% 前端构建完成

REM 显示构建产物
echo %INFO% 构建产物:
dir web\

REM 构建 Go 应用
:build_go
echo %INFO% 构建 Go 应用...

REM 创建构建目录
if not exist "build" mkdir build

REM 构建标志
set LDFLAGS=-ldflags "-X main.version=%VERSION% -X main.commit=%GIT_COMMIT% -X main.date=%BUILD_TIME%"

REM 构建当前平台
go build %LDFLAGS% -o build\logviewer.exe cmd\logviewer\main.go
if errorlevel 1 (
    echo %ERROR% Go 应用构建失败
    exit /b 1
)

echo %SUCCESS% Go 应用构建完成: build\logviewer.exe

REM 显示文件信息
dir build\

goto :eof

REM 构建所有平台
:build_all_platforms
echo %INFO% 构建所有平台版本...

REM Linux amd64
set GOOS=linux
set GOARCH=amd64
go build %LDFLAGS% -o build\logviewer-linux-amd64 cmd\logviewer\main.go

REM Linux arm64
set GOOS=linux
set GOARCH=arm64
go build %LDFLAGS% -o build\logviewer-linux-arm64 cmd\logviewer\main.go

REM macOS amd64
set GOOS=darwin
set GOARCH=amd64
go build %LDFLAGS% -o build\logviewer-darwin-amd64 cmd\logviewer\main.go

REM macOS arm64
set GOOS=darwin
set GOARCH=arm64
go build %LDFLAGS% -o build\logviewer-darwin-arm64 cmd\logviewer\main.go

REM Windows amd64
set GOOS=windows
set GOARCH=amd64
go build %LDFLAGS% -o build\logviewer-windows-amd64.exe cmd\logviewer\main.go

echo %SUCCESS% 所有平台构建完成
dir build\

goto :eof

REM 清理
:clean
echo %INFO% 清理构建文件...
if exist "build" rmdir /s /q build
if exist "web" rmdir /s /q web
echo %SUCCESS% 清理完成
goto :eof

REM 显示帮助
:help
echo 构建脚本使用说明:
echo.
echo 用法: %~nx0 [命令]
echo.
echo 命令:
echo   frontend          仅构建前端
echo   go                构建 Go 应用
echo   all               构建前端和 Go 应用 (默认)
echo   all-platforms     构建所有平台版本
echo   clean             清理构建文件
echo   help              显示帮助信息
echo.
echo 示例:
echo   %~nx0 all                    # 构建前端和应用
echo   %~nx0 all-platforms          # 构建所有平台版本
echo   %~nx0 frontend               # 仅构建前端
echo.
goto :eof

REM 主逻辑
if "%1"=="frontend" goto build_frontend
if "%1"=="go" (
    call :build_frontend
    goto build_go
)
if "%1"=="all" (
    call :build_frontend
    goto build_go
)
if "%1"=="all-platforms" (
    call :build_frontend
    goto build_all_platforms
)
if "%1"=="clean" goto clean
if "%1"=="help" goto help
if "%1"=="-h" goto help
if "%1"=="--help" goto help

REM 默认执行 all
call :build_frontend
goto build_go