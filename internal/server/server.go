package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/errors"
	"github.com/local-log-viewer/internal/health"
	"github.com/local-log-viewer/internal/interfaces"
	"github.com/local-log-viewer/internal/logger"
	"github.com/local-log-viewer/internal/middleware"
	"github.com/local-log-viewer/internal/shutdown"
	"github.com/local-log-viewer/internal/types"
)

// staticFiles will be set from main package
var staticFiles embed.FS

var startTime = time.Now()

// BuildInfo 构建信息
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
	GoVersion string `json:"goVersion"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// 这些变量将在构建时通过 ldflags 设置
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// getBuildInfo 获取构建信息
func getBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildTime: date,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// getLocalIPs 获取本机所有网络接口的IP地址
func getLocalIPs() []string {
	var ips []string

	// 添加localhost
	ips = append(ips, "localhost")
	ips = append(ips, "127.0.0.1")

	// 获取所有网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range interfaces {
		// 跳过回环接口和未启用的接口
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// 只获取IPv4地址，跳过回环地址
			if ip == nil || ip.IsLoopback() {
				continue
			}

			// 只获取IPv4地址
			if ipv4 := ip.To4(); ipv4 != nil {
				ips = append(ips, ipv4.String())
			}
		}
	}

	return ips
}

// HTTPServer HTTP服务器
type HTTPServer struct {
	router          *gin.Engine
	config          *config.Config
	server          *http.Server
	logManager      interfaces.LogManager
	wsHub           interfaces.WebSocketHub
	healthService   *health.HealthService
	shutdownManager *shutdown.Manager
}

// New 创建新的HTTP服务器
func New(cfg *config.Config, logManager interfaces.LogManager, wsHub interfaces.WebSocketHub) *HTTPServer {
	// 创建健康检查服务
	healthService := health.NewHealthService()

	// 注册内置健康检查
	healthService.RegisterCheck(health.NewFileSystemCheck(cfg.Server.LogPaths))
	healthService.RegisterCheck(health.NewMemoryCheck(200)) // 200MB内存限制

	// 创建关闭管理器
	shutdownManager := shutdown.NewManager(30 * time.Second)

	return &HTTPServer{
		config:          cfg,
		router:          gin.New(), // 使用gin.New()而不是gin.Default()以便自定义中间件
		logManager:      logManager,
		wsHub:           wsHub,
		healthService:   healthService,
		shutdownManager: shutdownManager,
	}
}

// NewWithStaticFiles 创建带有嵌入静态文件的HTTP服务器
func NewWithStaticFiles(cfg *config.Config, logManager interfaces.LogManager, wsHub interfaces.WebSocketHub, files embed.FS) *HTTPServer {
	staticFiles = files
	return New(cfg, logManager, wsHub)
}

// Start 启动服务器
func (s *HTTPServer) Start() error {
	// 检查是否启用TLS
	if s.config.Security.TLS.Enabled {
		return s.StartTLS(&s.config.Security.TLS)
	}

	s.setupRoutes()

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 注册关闭钩子
	s.shutdownManager.AddHook(s.shutdownHook)

	// 启动关闭管理器
	s.shutdownManager.Start()

	// 获取所有可访问的IP地址
	localIPs := getLocalIPs()

	// 构建访问地址列表
	var accessURLs []string
	protocol := "http"
	if s.config.Security.TLS.Enabled {
		protocol = "https"
	}

	for _, ip := range localIPs {
		accessURLs = append(accessURLs, fmt.Sprintf("%s://%s:%d", protocol, ip, s.config.Server.Port))
	}

	logger.Info("服务器启动成功",
		zap.String("host", s.config.Server.Host),
		zap.Int("port", s.config.Server.Port),
		zap.Strings("log_paths", s.config.Server.LogPaths),
		zap.Bool("tls_enabled", false),
		zap.Strings("access_urls", accessURLs),
	)

	// 在控制台输出友好的访问信息
	fmt.Printf("\n🚀 日志查看器启动成功!\n")
	fmt.Printf("📂 监控日志路径: %s\n", strings.Join(s.config.Server.LogPaths, ", "))
	fmt.Printf("🌐 可通过以下地址访问:\n")
	for _, url := range accessURLs {
		fmt.Printf("   • %s\n", url)
	}
	fmt.Printf("\n按 Ctrl+C 停止服务器\n\n")

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return errors.WrapError(err, errors.ErrorTypeInternalError, "failed to start HTTP server")
	}

	return nil
}

// Stop 停止服务器
func (s *HTTPServer) Stop() error {
	logger.Info("stopping server")
	s.shutdownManager.Shutdown()
	s.shutdownManager.Wait()
	return nil
}

// shutdownHook 服务器关闭钩子
func (s *HTTPServer) shutdownHook(ctx context.Context) error {
	if s.server != nil {
		logger.Info("shutting down HTTP server")
		return s.server.Shutdown(ctx)
	}
	return nil
}

// setupRoutes 设置路由
func (s *HTTPServer) setupRoutes() {
	// 添加中间件
	s.router.Use(middleware.RequestLogger())
	s.router.Use(middleware.ErrorHandler())
	s.router.Use(middleware.SecurityHeaders())
	s.router.Use(middleware.IPWhitelist(&s.config.Security))
	s.router.Use(s.corsHandler())

	// 静态文件服务 - 使用嵌入的静态文件
	s.setupStaticFiles()

	// NoRoute处理 - 对于前端路由返回index.html
	s.router.NoRoute(func(c *gin.Context) {
		// 对于前端路由，返回index.html
		if !strings.HasPrefix(c.Request.URL.Path, "/api") && !strings.HasPrefix(c.Request.URL.Path, "/ws") {
			s.serveIndexHTML(c)
			return
		}
		c.JSON(http.StatusNotFound, types.ErrorResponse{
			Code:    http.StatusNotFound,
			Message: "接口不存在",
		})
	})

	// API 路由 - 应用认证中间件
	api := s.router.Group("/api")
	api.Use(middleware.BasicAuth(&s.config.Security))
	{
		api.GET("/logs", s.getLogFiles)
		api.GET("/logs/content/*path", s.getLogContent)
		api.GET("/logs/tail/*path", s.getLogContentFromTail)
		api.GET("/search", s.searchLogs)
		api.GET("/health", s.healthCheck)
		api.GET("/health/detailed", s.detailedHealthCheck)
		api.GET("/version", s.getBuildInfo)
	}

	// WebSocket 路由 - 应用认证中间件
	ws := s.router.Group("/ws")
	ws.Use(middleware.BasicAuth(&s.config.Security))
	{
		ws.GET("", s.handleWebSocket)
	}
}

// getLogFiles 获取日志文件列表 API
func (s *HTTPServer) getLogFiles(c *gin.Context) {
	files, err := s.logManager.GetLogFiles()
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInternalError, "failed to get log files"))
		return
	}

	logger.Debug("retrieved log files", zap.Int("count", len(files)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    files,
		"count":   len(files),
	})
}

// getLogContent 获取日志内容 API
func (s *HTTPServer) getLogContent(c *gin.Context) {
	// 获取路径参数
	path := c.Param("path")

	// 移除路径前的斜杠
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		c.Error(errors.NewConfigError("path", fmt.Errorf("missing file path parameter")))
		return
	}

	// URL解码
	decodedPath, err := url.QueryUnescape(path)
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInvalidFormat, "invalid path parameter format"))
		return
	}

	// 获取查询参数
	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "100")

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInvalidFormat, "invalid offset parameter"))
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInvalidFormat, "invalid limit parameter"))
		return
	}

	// 限制每次请求的最大行数
	if limit > 1000 {
		limit = 1000
	}
	if limit <= 0 {
		limit = 100
	}

	// 读取日志文件内容
	content, err := s.logManager.ReadLogFile(decodedPath, offset, limit)
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInternalError, "failed to read log file"))
		return
	}

	logger.Debug("read log file content",
		zap.String("path", decodedPath),
		zap.Int64("offset", offset),
		zap.Int("limit", limit),
		zap.Int("entries_count", len(content.Entries)),
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    content,
	})
}

// getLogContentFromTail 从文件尾部获取日志内容 API
func (s *HTTPServer) getLogContentFromTail(c *gin.Context) {
	// 获取路径参数
	path := c.Param("path")

	// 移除路径前的斜杠
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		c.Error(errors.NewConfigError("path", fmt.Errorf("missing file path parameter")))
		return
	}

	// URL解码
	decodedPath, err := url.QueryUnescape(path)
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInvalidFormat, "invalid path parameter format"))
		return
	}

	// 获取查询参数
	linesStr := c.DefaultQuery("lines", "100")

	lines, err := strconv.Atoi(linesStr)
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInvalidFormat, "invalid lines parameter"))
		return
	}

	// 限制每次请求的最大行数
	if lines > 1000 {
		lines = 1000
	}
	if lines <= 0 {
		lines = 100
	}

	// 从文件尾部读取日志内容
	content, err := s.logManager.ReadLogFileFromTail(decodedPath, lines)
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInternalError, "failed to read log file from tail"))
		return
	}

	logger.Debug("read log file content from tail",
		zap.String("path", decodedPath),
		zap.Int("lines", lines),
		zap.Int("entries_count", len(content.Entries)),
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    content,
	})
}

// searchLogs 搜索日志 API
func (s *HTTPServer) searchLogs(c *gin.Context) {
	// 获取查询参数
	path := c.Query("path")
	query := c.Query("query")
	isRegexStr := c.DefaultQuery("isRegex", "false")
	startTimeStr := c.Query("startTime")
	endTimeStr := c.Query("endTime")
	levelsStr := c.Query("levels")
	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "100")

	// 验证必需参数
	if path == "" {
		c.Error(errors.NewSearchError("path", fmt.Errorf("missing path parameter")))
		return
	}

	if query == "" {
		c.Error(errors.NewSearchError("query", fmt.Errorf("missing query parameter")))
		return
	}

	// 解析参数
	isRegex := isRegexStr == "true"

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			c.Error(errors.WrapError(err, errors.ErrorTypeInvalidFormat, "invalid startTime format, should use RFC3339"))
			return
		}
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			c.Error(errors.WrapError(err, errors.ErrorTypeInvalidFormat, "invalid endTime format, should use RFC3339"))
			return
		}
	}

	var levels []string
	if levelsStr != "" {
		levels = strings.Split(levelsStr, ",")
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInvalidFormat, "invalid offset parameter"))
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInvalidFormat, "invalid limit parameter"))
		return
	}

	// 限制每次搜索的最大结果数
	if limit > 500 {
		limit = 500
	}
	if limit <= 0 {
		limit = 100
	}

	// 构建搜索查询
	searchQuery := types.SearchQuery{
		Path:      path,
		Query:     query,
		IsRegex:   isRegex,
		StartTime: startTime,
		EndTime:   endTime,
		Levels:    levels,
		Offset:    offset,
		Limit:     limit,
	}

	// 执行搜索
	result, err := s.logManager.SearchLogs(searchQuery)
	if err != nil {
		c.Error(errors.WrapError(err, errors.ErrorTypeInternalError, "failed to search logs"))
		return
	}

	logger.Debug("search completed",
		zap.String("path", path),
		zap.String("query", query),
		zap.Bool("is_regex", isRegex),
		zap.Int64("total_count", result.TotalCount),
		zap.Int("returned_count", len(result.Entries)),
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// healthCheck 简单健康检查 API
func (s *HTTPServer) healthCheck(c *gin.Context) {
	status, _ := s.healthService.GetOverallStatus(c.Request.Context())

	httpStatus := http.StatusOK
	if status == health.StatusUnhealthy {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":    string(status),
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(startTime).Seconds(),
	})
}

// detailedHealthCheck 详细健康检查 API
func (s *HTTPServer) detailedHealthCheck(c *gin.Context) {
	status, results := s.healthService.GetOverallStatus(c.Request.Context())
	systemInfo := s.healthService.GetSystemInfo()

	httpStatus := http.StatusOK
	if status == health.StatusUnhealthy {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":    string(status),
		"timestamp": time.Now().Unix(),
		"checks":    results,
		"system":    systemInfo,
	})
}

// getBuildInfo 获取构建信息 API
func (s *HTTPServer) getBuildInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    getBuildInfo(),
	})
}

// handleWebSocket WebSocket连接处理
func (s *HTTPServer) handleWebSocket(c *gin.Context) {
	HandleWebSocketConnection(c, s.wsHub, s.logManager)
}

// corsHandler CORS处理中间件
func (s *HTTPServer) corsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// setupStaticFiles 设置静态文件服务
func (s *HTTPServer) setupStaticFiles() {
	// 创建嵌入文件系统的子文件系统
	webFS, err := fs.Sub(staticFiles, "web")
	if err != nil {
		// 如果嵌入文件不存在，尝试使用本地文件系统（开发模式）
		if _, err := os.Stat("web"); err == nil {
			s.router.Static("/assets", "./web/assets")
			s.router.StaticFile("/favicon.ico", "./web/favicon.ico")
			s.router.GET("/", func(c *gin.Context) {
				c.File("./web/index.html")
			})
			return
		}
		panic(fmt.Sprintf("无法设置静态文件服务: %v", err))
	}

	// 使用嵌入的文件系统 - 为assets创建专门的子文件系统
	assetsFS, err := fs.Sub(webFS, "assets")
	if err != nil {
		panic(fmt.Sprintf("无法创建assets文件系统: %v", err))
	}
	s.router.StaticFS("/assets", http.FS(assetsFS))

	// 处理favicon
	s.router.GET("/favicon.ico", func(c *gin.Context) {
		data, err := staticFiles.ReadFile("web/favicon.ico")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Content-Type", "image/x-icon")
		c.Data(http.StatusOK, "image/x-icon", data)
	})

	// 处理根路径
	s.router.GET("/", func(c *gin.Context) {
		s.serveIndexHTML(c)
	})
}

// serveIndexHTML 服务index.html文件
func (s *HTTPServer) serveIndexHTML(c *gin.Context) {
	data, err := staticFiles.ReadFile("web/index.html")
	if err != nil {
		// 如果嵌入文件不存在，尝试使用本地文件（开发模式）
		if _, err := os.Stat("web/index.html"); err == nil {
			c.File("web/index.html")
			return
		}
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Message: "无法加载前端页面",
			Details: err.Error(),
		})
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}
