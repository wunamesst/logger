package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/local-log-viewer/internal/cache"
	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/logger"
	"github.com/local-log-viewer/internal/manager"
	"github.com/local-log-viewer/internal/server"
	"github.com/local-log-viewer/internal/watcher"
	"go.uber.org/zap"
)

//go:embed all:web
var StaticFiles embed.FS

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// BuildInfo 构建信息
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
	GoVersion string `json:"goVersion"`
}

// GetBuildInfo 获取构建信息
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildTime: date,
		GoVersion: runtime.Version(),
	}
}

func main() {
	var (
		configPath     = flag.String("config", "", "配置文件路径")
		port           = flag.Int("port", 0, "服务端口 (默认: 8080)")
		host           = flag.String("host", "", "服务器绑定地址 (默认: 0.0.0.0)")
		logPaths       = flag.String("logs", "", "日志文件路径，多个路径用逗号分隔 (默认: ./logs)")
		maxFileSize    = flag.String("max-file-size", "", "最大文件大小，支持单位 K/M/G (默认: 100M)")
		cacheSize      = flag.Int("cache-size", 0, "文件缓存数量 (默认: 50)")
		enableAuth     = flag.Bool("enable-auth", false, "启用基本认证")
		username       = flag.String("username", "", "认证用户名")
		password       = flag.String("password", "", "认证密码")
		allowedIPs     = flag.String("allowed-ips", "", "允许访问的IP列表，用逗号分隔")
		enableTLS      = flag.Bool("enable-tls", false, "启用HTTPS")
		certFile       = flag.String("cert-file", "", "TLS证书文件路径")
		keyFile        = flag.String("key-file", "", "TLS私钥文件路径")
		autoCert       = flag.Bool("auto-cert", false, "自动生成自签名证书")
		logLevel       = flag.String("log-level", "", "日志级别 (debug/info/warn/error)")
		generateConfig = flag.Bool("generate-config", false, "生成示例配置文件")
		versionFlag    = flag.Bool("version", false, "显示版本信息")
		help           = flag.Bool("help", false, "显示帮助信息")
	)

	// 自定义用法信息
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "本地日志查看工具 - 轻量级日志管理和查看解决方案\n\n")
		fmt.Fprintf(os.Stderr, "用法: %s [选项]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "选项:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n示例:\n")
		fmt.Fprintf(os.Stderr, "  %s                                    # 使用默认配置启动\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -port 9090 -logs /var/log          # 指定端口和日志路径\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -config config.yaml                # 使用配置文件\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -generate-config                   # 生成示例配置文件\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -enable-auth -username admin -password secret  # 启用认证\n", os.Args[0])
	}

	flag.Parse()

	// 显示版本信息
	if *versionFlag {
		buildInfo := GetBuildInfo()
		fmt.Printf("本地日志查看工具\n")
		fmt.Printf("版本: %s\n", buildInfo.Version)
		fmt.Printf("Git提交: %s\n", buildInfo.Commit)
		fmt.Printf("构建时间: %s\n", buildInfo.BuildTime)
		fmt.Printf("Go版本: %s\n", buildInfo.GoVersion)
		fmt.Printf("系统架构: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	// 显示帮助信息
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// 生成配置文件
	if *generateConfig {
		if err := generateConfigFile(); err != nil {
			log.Fatalf("生成配置文件失败: %v", err)
		}
		fmt.Println("配置文件已生成: config.yaml")
		os.Exit(0)
	}

	// 构建命令行选项
	cmdOptions := &config.CommandLineOptions{
		ConfigPath:  *configPath,
		Port:        *port,
		Host:        *host,
		LogPaths:    *logPaths,
		MaxFileSize: *maxFileSize,
		CacheSize:   *cacheSize,
		EnableAuth:  *enableAuth,
		Username:    *username,
		Password:    *password,
		AllowedIPs:  *allowedIPs,
		EnableTLS:   *enableTLS,
		CertFile:    *certFile,
		KeyFile:     *keyFile,
		AutoCert:    *autoCert,
		LogLevel:    *logLevel,
	}

	// 加载配置
	cfg, err := config.LoadWithOptions(cmdOptions)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志系统
	if err := logger.Initialize(cfg.Logging); err != nil {
		log.Fatalf("初始化日志系统失败: %v", err)
	}
	defer logger.Sync()

	logger.Info("application starting",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("build_time", date),
		zap.String("go_version", runtime.Version()),
	)

	// 创建依赖组件

	// 创建缓存 (最大缓存数量, TTL时间)
	logCache := cache.NewMemoryCache(cfg.Server.CacheSize, time.Hour)

	// 创建文件监控器
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		logger.Fatal("failed to create file watcher", zap.Error(err))
	}

	// 启动文件监控器
	if err := fileWatcher.Start(); err != nil {
		logger.Fatal("failed to start file watcher", zap.Error(err))
	}
	defer fileWatcher.Stop()

	// 创建WebSocket中心
	wsHub := server.NewWebSocketHub()

	// 启动WebSocket中心
	if err := wsHub.Start(); err != nil {
		logger.Fatal("failed to start WebSocket hub", zap.Error(err))
	}
	defer wsHub.Stop()

	// 创建日志管理器
	logManager := manager.NewLogManager(cfg, fileWatcher, logCache)

	// 启动日志管理器
	if err := logManager.Start(); err != nil {
		logger.Fatal("failed to start log manager", zap.Error(err))
	}
	defer logManager.Stop()

	// 集成WebSocket和日志管理器，实现实时日志更新推送
	// 注释掉自动监控，让前端通过WebSocket主动订阅需要的文件
	/*
		go func() {
			// 获取所有日志文件并监控
			logFiles, err := logManager.GetLogFiles()
			if err != nil {
				logger.Error("failed to get log files", zap.Error(err))
				return
			}

			// 监控每个具体的日志文件
			var watchFile func([]types.LogFile)
			watchFile = func(files []types.LogFile) {
				for _, file := range files {
					if file.IsDirectory && len(file.Children) > 0 {
						watchFile(file.Children)
					} else if !file.IsDirectory {
						go func(filePath string) {
							logger.Info("Started watching file", zap.String("path", filePath))
							updateCh, err := logManager.WatchFile(filePath)
							if err != nil {
								logger.Error("failed to watch file", zap.String("path", filePath), zap.Error(err))
								return
							}

							// 监听文件更新并广播到WebSocket客户端
							for update := range updateCh {
								logger.Debug("Broadcasting file update", zap.String("path", update.Path), zap.Int("entries", len(update.Entries)))
								wsHub.BroadcastLogUpdate(update)
							}
						}(file.Path)
					}
				}
			}

			watchFile(logFiles)
		}()
	*/

	// 启动服务器
	srv := server.NewWithStaticFiles(cfg, logManager, wsHub, StaticFiles)
	if err := srv.Start(); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}

// generateConfigFile 生成示例配置文件
func generateConfigFile() error {
	cfg := config.DefaultConfig()
	return cfg.Save("config.yaml")
}
