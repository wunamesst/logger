# -daemon 参数使用指南

`-daemon` 参数已成功添加到 LogViewer，允许程序以守护进程模式（后台运行）启动。

## ✨ 功能特性

- ✅ **一键后台运行**: 无需额外脚本，直接使用 `-daemon` 参数
- ✅ **自动脱离终端**: 进程自动成为会话领导者，父进程变为 init/launchd
- ✅ **标准输出重定向**: stdout/stderr/stdin 自动重定向到 /dev/null
- ✅ **PID 文件管理**: 自动创建和清理 PID 文件
- ✅ **优雅关闭**: 支持 SIGTERM 信号优雅关闭
- ✅ **防止重复启动**: 通过 PID 文件检查避免重复启动
- ✅ **跨平台提示**: Windows 系统提示使用服务方式

## 🚀 基本用法

### 最简单的后台运行

```bash
./logviewer -daemon
```

### 指定配置后台运行

```bash
./logviewer -daemon -config config.yaml
```

### 指定端口和日志目录

```bash
./logviewer -daemon -port 9090 -logs /var/log
```

### 完整示例

```bash
./logviewer -daemon \
  -config /etc/logviewer/config.yaml \
  -logs /var/log \
  -port 8080 \
  -pid-file /var/run/logviewer.pid
```

## 📝 工作原理

### Unix/Linux/macOS 系统

当使用 `-daemon` 参数时，程序会：

1. **Fork 子进程**: 使用 `syscall.ForkExec` 创建子进程
2. **设置会话**: 子进程调用 `setsid()` 创建新会话
3. **重定向 I/O**: stdin/stdout/stderr 重定向到 `/dev/null`
4. **父进程退出**: 打印子进程 PID 后父进程立即退出
5. **子进程继续**: 子进程在后台继续运行服务

### 进程特征

守护进程启动后：
- **PPID**: 1 (init 或 launchd)
- **STAT**: Ss (S=睡眠, s=会话领导者)
- **终端**: 无控制终端

### Windows 系统

Windows 不支持 Unix 风格的 fork，使用 `-daemon` 会显示错误提示：

```
Windows 系统不支持 -daemon 参数，请使用 Windows 服务方式运行。
请参考文档: docs/SERVICE_MANAGEMENT.md
或运行: scripts\service\windows\install-service.bat
```

## 🔧 管理守护进程

### 启动

```bash
./logviewer -daemon -logs ./logs -port 8080
# 输出: 守护进程已启动 (PID: 12345)
```

### 查看状态

```bash
# 方法 1: 通过 PID 文件
cat logviewer.pid
# 输出: 12345

# 方法 2: 使用 ps 命令
ps -p $(cat logviewer.pid)
```

### 检查进程详情

```bash
ps -p $(cat logviewer.pid) -o pid,ppid,stat,command
```

### 停止进程

```bash
# 优雅关闭 (推荐)
kill -TERM $(cat logviewer.pid)

# 强制关闭
kill -9 $(cat logviewer.pid)
```

### 重启进程

```bash
# 停止
kill -TERM $(cat logviewer.pid)

# 等待进程完全停止
sleep 2

# 重新启动
./logviewer -daemon -logs ./logs -port 8080
```

### 查看服务是否运行

```bash
if ps -p $(cat logviewer.pid 2>/dev/null) > /dev/null 2>&1; then
    echo "服务正在运行"
else
    echo "服务已停止"
fi
```

## 📊 验证守护进程

### 测试完整流程

```bash
# 1. 启动守护进程
./logviewer -daemon -logs ./logs -port 8080

# 2. 验证进程状态
ps -p $(cat logviewer.pid) -o pid,ppid,stat,command

# 3. 测试服务响应
curl http://localhost:8080/api/health

# 4. 停止进程
kill -TERM $(cat logviewer.pid)

# 5. 确认已停止
ps -p $(cat logviewer.pid) || echo "进程已停止"
```

### 预期输出

**启动输出**:
```
守护进程已启动 (PID: 12345)
```

**进程状态**:
```
PID  PPID STAT COMMAND
12345     1 Ss   ./logviewer -logs ./logs -port 8080
```

**健康检查**:
```json
{
  "status": "healthy",
  "timestamp": 1234567890,
  "uptime": 123.45
}
```

## 🆚 对比其他启动方式

| 方式 | 命令 | 优点 | 缺点 |
|------|------|------|------|
| **前台运行** | `./logviewer` | 调试方便，直接看输出 | 占用终端，关闭终端即停止 |
| **nohup** | `nohup ./logviewer &` | 简单 | 需要手动管理 PID |
| **daemon 参数** | `./logviewer -daemon` | ✅ 一键启动，自动管理 | 需要手动停止 |
| **systemd** | `systemctl start logviewer` | ✅ 开机自启，自动重启 | 需要安装服务 |
| **daemon-control.sh** | `./scripts/daemon-control.sh start` | ✅ 统一接口 | 需要脚本支持 |

## 💡 最佳实践

### 开发环境

推荐使用前台运行或 `-daemon` 参数：

```bash
# 开发调试（前台）
./logviewer -logs ./logs -port 8080

# 后台运行（不占用终端）
./logviewer -daemon -logs ./logs -port 8080
```

### 测试环境

使用 `-daemon` 参数或控制脚本：

```bash
# 方式 1: daemon 参数
./logviewer -daemon -config test-config.yaml

# 方式 2: 控制脚本
./scripts/daemon-control.sh start
```

### 生产环境

**强烈推荐使用系统服务**（systemd/launchd）：

```bash
# Linux
sudo make install-service
sudo make service-start

# 或手动
cd scripts/service/systemd
sudo ./install-systemd.sh
```

理由：
- ✅ 开机自动启动
- ✅ 崩溃自动重启
- ✅ 日志集中管理
- ✅ 资源限制控制
- ✅ 权限安全隔离

## 🔍 故障排除

### 问题 1: 启动失败，提示端口被占用

```bash
# 检查端口占用
lsof -i :8080

# 或
netstat -tlnp | grep 8080

# 解决：使用其他端口
./logviewer -daemon -port 9090
```

### 问题 2: 无法找到 PID 文件

```bash
# 指定 PID 文件路径
./logviewer -daemon -pid-file /tmp/logviewer.pid
```

### 问题 3: 进程已停止但 PID 文件仍存在

```bash
# 手动清理陈旧的 PID 文件
rm -f logviewer.pid

# 重新启动
./logviewer -daemon
```

### 问题 4: Windows 提示不支持

Windows 系统应使用服务方式：

```cmd
REM 安装服务
cd scripts\service\windows
install-service.bat

REM 启动服务
net start LogViewer
```

### 问题 5: 无法查看日志输出

守护进程模式下，输出会重定向到 `/dev/null`。

解决方案：
1. 查看配置文件中指定的日志文件
2. 或使用前台模式调试：
   ```bash
   ./logviewer -logs ./logs -port 8080
   ```

## 📚 相关文档

- [服务管理指南](SERVICE_MANAGEMENT.md) - systemd/launchd 服务配置
- [部署指南](DEPLOYMENT_GUIDE.md) - 生产环境部署
- [后台运行说明](DAEMON_MODE.md) - daemon-control.sh 脚本使用

## 🎯 总结

`-daemon` 参数提供了一个简单、直接的方式来后台运行 LogViewer，特别适合：

- ✅ 快速测试和开发
- ✅ 不需要系统服务权限的场景
- ✅ 临时后台运行需求
- ✅ 简单的单机部署

对于生产环境，仍然推荐使用 `make install-service` 安装为系统服务，以获得更好的管理和监控能力。
