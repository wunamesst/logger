# 部署指南

本指南详细介绍了如何在不同环境中部署本地日志查看工具。

## 目录

- [系统要求](#系统要求)
- [单机部署](#单机部署)
- [容器化部署](#容器化部署)
- [生产环境部署](#生产环境部署)
- [安全配置](#安全配置)
- [监控和维护](#监控和维护)

## 系统要求

### 最低要求

- **操作系统**: Linux (Ubuntu 18.04+, CentOS 7+), macOS 10.15+, Windows 10+
- **内存**: 512MB RAM
- **存储**: 100MB 可用空间
- **网络**: 可选，用于远程访问

### 推荐配置

- **操作系统**: Linux (Ubuntu 20.04+, CentOS 8+)
- **内存**: 2GB RAM
- **存储**: 1GB 可用空间 + 日志文件存储空间
- **CPU**: 2 核心
- **网络**: 千兆网络（用于大文件传输）

## 单机部署

### 1. 下载二进制文件

```bash
# Linux AMD64
wget https://github.com/your-org/logviewer/releases/latest/download/logviewer-linux-amd64
chmod +x logviewer-linux-amd64
mv logviewer-linux-amd64 /usr/local/bin/logviewer

# macOS
curl -L https://github.com/your-org/logviewer/releases/latest/download/logviewer-darwin-amd64 -o logviewer
chmod +x logviewer
mv logviewer /usr/local/bin/
```

### 2. 创建配置文件

```bash
# 创建配置目录
sudo mkdir -p /etc/logviewer

# 创建配置文件
sudo tee /etc/logviewer/config.yaml > /dev/null << 'EOF'
server:
  host: "0.0.0.0"
  port: 8080
  logPaths:
    - "/var/log"
    - "/opt/app/logs"
  maxFileSize: 1073741824  # 1GB
  cacheSize: 100

logging:
  level: "info"
  file: "/var/log/logviewer.log"

security:
  enableAuth: false
  allowedIPs:
    - "192.168.0.0/16"
    - "10.0.0.0/8"
EOF
```

### 3. 创建系统服务

#### systemd (Linux)

```bash
# 创建服务文件
sudo tee /etc/systemd/system/logviewer.service > /dev/null << 'EOF'
[Unit]
Description=Log Viewer Service
After=network.target

[Service]
Type=simple
User=logviewer
Group=logviewer
ExecStart=/usr/local/bin/logviewer -config /etc/logviewer/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log
ReadOnlyPaths=/etc/logviewer

[Install]
WantedBy=multi-user.target
EOF

# 创建用户
sudo useradd -r -s /bin/false logviewer

# 设置权限
sudo chown -R logviewer:logviewer /etc/logviewer
sudo chmod 640 /etc/logviewer/config.yaml

# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable logviewer
sudo systemctl start logviewer
```

#### launchd (macOS)

```bash
# 创建 plist 文件
sudo tee /Library/LaunchDaemons/com.yourorg.logviewer.plist > /dev/null << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.yourorg.logviewer</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/logviewer</string>
        <string>-config</string>
        <string>/etc/logviewer/config.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/logviewer.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/logviewer.error.log</string>
</dict>
</plist>
EOF

# 加载服务
sudo launchctl load /Library/LaunchDaemons/com.yourorg.logviewer.plist
```

### 4. 验证部署

```bash
# 检查服务状态
sudo systemctl status logviewer

# 检查日志
sudo journalctl -u logviewer -f

# 测试 API
curl http://localhost:8080/api/health

# 测试 Web 界面
curl -I http://localhost:8080/
```

## 容器化部署

### 1. 创建 Dockerfile

```dockerfile
FROM alpine:3.18

# 安装必要的包
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -g 1001 logviewer && \
    adduser -D -u 1001 -G logviewer logviewer

# 设置工作目录
WORKDIR /app

# 复制二进制文件
COPY logviewer /app/logviewer
RUN chmod +x /app/logviewer

# 创建日志目录
RUN mkdir -p /var/log/app && \
    chown -R logviewer:logviewer /var/log/app

# 切换到非 root 用户
USER logviewer

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# 启动命令
CMD ["/app/logviewer", "-host", "0.0.0.0", "-port", "8080", "-logs", "/var/log/app"]
```

### 2. 构建镜像

```bash
# 构建镜像
docker build -t logviewer:latest .

# 运行容器
docker run -d \
  --name logviewer \
  -p 8080:8080 \
  -v /var/log:/var/log/app:ro \
  --restart unless-stopped \
  logviewer:latest
```

### 3. Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  logviewer:
    image: logviewer:latest
    container_name: logviewer
    ports:
      - "8080:8080"
    volumes:
      - /var/log:/var/log/app:ro
      - ./config.yaml:/app/config.yaml:ro
    environment:
      - TZ=Asia/Shanghai
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

  # 可选：添加 Nginx 反向代理
  nginx:
    image: nginx:alpine
    container_name: logviewer-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - logviewer
    restart: unless-stopped
```

### 4. Kubernetes 部署

```yaml
# logviewer-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: logviewer
  labels:
    app: logviewer
spec:
  replicas: 2
  selector:
    matchLabels:
      app: logviewer
  template:
    metadata:
      labels:
        app: logviewer
    spec:
      containers:
      - name: logviewer
        image: logviewer:latest
        ports:
        - containerPort: 8080
        env:
        - name: TZ
          value: "Asia/Shanghai"
        volumeMounts:
        - name: log-volume
          mountPath: /var/log/app
          readOnly: true
        - name: config-volume
          mountPath: /app/config.yaml
          subPath: config.yaml
        livenessProbe:
          httpGet:
            path: /api/health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: log-volume
        hostPath:
          path: /var/log
      - name: config-volume
        configMap:
          name: logviewer-config

---
apiVersion: v1
kind: Service
metadata:
  name: logviewer-service
spec:
  selector:
    app: logviewer
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: logviewer-config
data:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
      logPaths:
        - "/var/log/app"
    logging:
      level: "info"
```

## 生产环境部署

### 1. 负载均衡配置

#### Nginx 配置

```nginx
# /etc/nginx/sites-available/logviewer
upstream logviewer_backend {
    server 127.0.0.1:8080;
    server 127.0.0.1:8081 backup;
}

server {
    listen 80;
    server_name logs.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name logs.example.com;

    # SSL 配置
    ssl_certificate /etc/ssl/certs/logviewer.crt;
    ssl_certificate_key /etc/ssl/private/logviewer.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512;
    ssl_prefer_server_ciphers off;

    # 安全头
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload";

    # 限制请求大小
    client_max_body_size 10M;

    # 代理配置
    location / {
        proxy_pass http://logviewer_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket 支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # 超时设置
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # 静态文件缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg)$ {
        proxy_pass http://logviewer_backend;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # API 限流
    location /api/ {
        limit_req zone=api burst=20 nodelay;
        proxy_pass http://logviewer_backend;
    }
}

# 限流配置
http {
    limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
}
```

### 2. 高可用配置

#### 多实例部署

```bash
# 实例 1
/usr/local/bin/logviewer -config /etc/logviewer/config1.yaml -port 8080 &

# 实例 2
/usr/local/bin/logviewer -config /etc/logviewer/config2.yaml -port 8081 &

# 健康检查脚本
#!/bin/bash
# /usr/local/bin/logviewer-healthcheck.sh

check_instance() {
    local port=$1
    local response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$port/api/health)
    
    if [ "$response" = "200" ]; then
        echo "Instance on port $port is healthy"
        return 0
    else
        echo "Instance on port $port is unhealthy"
        return 1
    fi
}

# 检查所有实例
for port in 8080 8081; do
    if ! check_instance $port; then
        # 重启不健康的实例
        systemctl restart logviewer@$port
    fi
done
```

### 3. 数据备份

```bash
#!/bin/bash
# /usr/local/bin/logviewer-backup.sh

BACKUP_DIR="/backup/logviewer"
DATE=$(date +%Y%m%d_%H%M%S)

# 创建备份目录
mkdir -p "$BACKUP_DIR"

# 备份配置文件
cp -r /etc/logviewer "$BACKUP_DIR/config_$DATE"

# 备份日志文件（可选）
if [ "$1" = "--include-logs" ]; then
    tar -czf "$BACKUP_DIR/logs_$DATE.tar.gz" /var/log
fi

# 清理旧备份（保留 7 天）
find "$BACKUP_DIR" -type f -mtime +7 -delete

echo "Backup completed: $BACKUP_DIR"
```

## 安全配置

### 1. 防火墙设置

```bash
# UFW (Ubuntu)
sudo ufw allow 22/tcp
sudo ufw allow 8080/tcp
sudo ufw enable

# iptables
sudo iptables -A INPUT -p tcp --dport 22 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
sudo iptables -A INPUT -j DROP
```

### 2. 访问控制

```yaml
# config.yaml
security:
  enableAuth: true
  username: "admin"
  password: "your-secure-password"
  allowedIPs:
    - "192.168.1.0/24"
    - "10.0.0.0/8"
  
  # JWT 配置
  jwtSecret: "your-jwt-secret"
  jwtExpiry: "24h"
  
  # HTTPS 配置
  enableHTTPS: true
  certFile: "/etc/ssl/certs/logviewer.crt"
  keyFile: "/etc/ssl/private/logviewer.key"
```

### 3. 日志审计

```yaml
# config.yaml
logging:
  level: "info"
  file: "/var/log/logviewer.log"
  
  # 审计日志
  auditLog:
    enabled: true
    file: "/var/log/logviewer-audit.log"
    events:
      - "login"
      - "file_access"
      - "search"
      - "config_change"
```

## 监控和维护

### 1. 健康检查

```bash
#!/bin/bash
# /usr/local/bin/logviewer-monitor.sh

# 检查服务状态
if ! systemctl is-active --quiet logviewer; then
    echo "CRITICAL: LogViewer service is not running"
    exit 2
fi

# 检查 HTTP 响应
response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/health)
if [ "$response" != "200" ]; then
    echo "CRITICAL: LogViewer HTTP check failed (status: $response)"
    exit 2
fi

# 检查内存使用
memory_usage=$(ps -o pid,ppid,cmd,%mem --sort=-%mem -C logviewer | awk 'NR==2{print $4}')
if (( $(echo "$memory_usage > 80" | bc -l) )); then
    echo "WARNING: High memory usage: ${memory_usage}%"
    exit 1
fi

echo "OK: LogViewer is healthy"
exit 0
```

### 2. 日志轮转

```bash
# /etc/logrotate.d/logviewer
/var/log/logviewer*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 logviewer logviewer
    postrotate
        systemctl reload logviewer
    endscript
}
```

### 3. 性能监控

```bash
#!/bin/bash
# /usr/local/bin/logviewer-metrics.sh

# 获取基本指标
cpu_usage=$(top -bn1 | grep "logviewer" | awk '{print $9}')
memory_usage=$(ps -o pid,ppid,cmd,%mem --sort=-%mem -C logviewer | awk 'NR==2{print $4}')
disk_usage=$(df -h /var/log | awk 'NR==2{print $5}')

# 获取连接数
connections=$(netstat -an | grep :8080 | grep ESTABLISHED | wc -l)

# 输出指标
echo "logviewer_cpu_usage $cpu_usage"
echo "logviewer_memory_usage $memory_usage"
echo "logviewer_disk_usage ${disk_usage%?}"
echo "logviewer_connections $connections"
```

### 4. 自动更新

```bash
#!/bin/bash
# /usr/local/bin/logviewer-update.sh

CURRENT_VERSION=$(logviewer -version | grep -o 'v[0-9.]*')
LATEST_VERSION=$(curl -s https://api.github.com/repos/your-org/logviewer/releases/latest | grep -o '"tag_name": "v[^"]*' | cut -d'"' -f4)

if [ "$CURRENT_VERSION" != "$LATEST_VERSION" ]; then
    echo "Updating from $CURRENT_VERSION to $LATEST_VERSION"
    
    # 下载新版本
    wget -O /tmp/logviewer "https://github.com/your-org/logviewer/releases/download/$LATEST_VERSION/logviewer-linux-amd64"
    
    # 停止服务
    systemctl stop logviewer
    
    # 备份当前版本
    cp /usr/local/bin/logviewer /usr/local/bin/logviewer.backup
    
    # 安装新版本
    chmod +x /tmp/logviewer
    mv /tmp/logviewer /usr/local/bin/logviewer
    
    # 启动服务
    systemctl start logviewer
    
    echo "Update completed"
else
    echo "Already up to date: $CURRENT_VERSION"
fi
```

## 故障排除

### 常见问题

1. **服务无法启动**
   - 检查端口是否被占用：`netstat -tlnp | grep 8080`
   - 检查配置文件语法：`logviewer -config /etc/logviewer/config.yaml -check`
   - 查看系统日志：`journalctl -u logviewer -f`

2. **无法访问日志文件**
   - 检查文件权限：`ls -la /var/log`
   - 检查 SELinux 状态：`getenforce`
   - 添加用户到日志组：`usermod -a -G adm logviewer`

3. **性能问题**
   - 增加内存限制
   - 调整缓存大小
   - 使用 SSD 存储

4. **网络连接问题**
   - 检查防火墙规则
   - 验证 DNS 解析
   - 测试网络连通性

### 调试模式

```bash
# 启用调试模式
logviewer -config /etc/logviewer/config.yaml -debug -log-level debug

# 查看详细日志
tail -f /var/log/logviewer.log | grep DEBUG
```

通过遵循本部署指南，您可以在各种环境中成功部署和运行本地日志查看工具。