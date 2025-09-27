package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Logging  LogConfig      `yaml:"logging"`
	Security SecurityConfig `yaml:"security"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host        string   `yaml:"host"`
	Port        int      `yaml:"port"`
	LogPaths    []string `yaml:"logPaths"`
	MaxFileSize int64    `yaml:"maxFileSize"`
	CacheSize   int      `yaml:"cacheSize"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	OutputPath string `yaml:"outputPath"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EnableAuth bool      `yaml:"enableAuth"`
	Username   string    `yaml:"username"`
	Password   string    `yaml:"password"`
	AllowedIPs []string  `yaml:"allowedIPs"`
	TLS        TLSConfig `yaml:"tls"`
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
	AutoCert bool   `yaml:"autoCert"`
}

// CommandLineOptions 命令行选项
type CommandLineOptions struct {
	ConfigPath  string
	Port        int
	Host        string
	LogPaths    string
	MaxFileSize string
	CacheSize   int
	EnableAuth  bool
	Username    string
	Password    string
	AllowedIPs  string
	LogLevel    string
	EnableTLS   bool
	CertFile    string
	KeyFile     string
	AutoCert    bool
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:        "0.0.0.0",
			Port:        8080,
			LogPaths:    []string{"./logs"},
			MaxFileSize: 100 * 1024 * 1024, // 100MB
			CacheSize:   50,                // 50个文件缓存
		},
		Logging: LogConfig{
			Level:      "info",
			Format:     "json",
			OutputPath: "stdout",
		},
		Security: SecurityConfig{
			EnableAuth: false,
			Username:   "",
			Password:   "",
			AllowedIPs: []string{},
			TLS: TLSConfig{
				Enabled:  false,
				CertFile: "",
				KeyFile:  "",
				AutoCert: false,
			},
		},
	}
}

// Load 加载配置 (保持向后兼容)
func Load(configPath string, port int, logPaths string) (*Config, error) {
	options := &CommandLineOptions{
		ConfigPath: configPath,
		Port:       port,
		LogPaths:   logPaths,
	}
	return LoadWithOptions(options)
}

// LoadWithOptions 使用命令行选项加载配置
func LoadWithOptions(options *CommandLineOptions) (*Config, error) {
	cfg := DefaultConfig()

	// 如果指定了配置文件，则加载配置文件
	// 如果未指定,尝试查找当前目录的 config.yaml
	configPath := options.ConfigPath
	if configPath == "" {
		defaultConfigPath := "config.yaml"
		if _, err := os.Stat(defaultConfigPath); err == nil {
			configPath = defaultConfigPath
		}
	}

	// 如果找到配置文件,则加载
	if configPath != "" {
		if err := loadFromFile(cfg, configPath); err != nil {
			return nil, fmt.Errorf("加载配置文件失败: %w", err)
		}
	}

	// 命令行参数覆盖配置文件
	if err := applyCommandLineOptions(cfg, options); err != nil {
		return nil, fmt.Errorf("应用命令行参数失败: %w", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return cfg, nil
}

// applyCommandLineOptions 应用命令行选项到配置
func applyCommandLineOptions(cfg *Config, options *CommandLineOptions) error {
	// 服务器配置
	if options.Port > 0 {
		cfg.Server.Port = options.Port
	}

	if options.Host != "" {
		cfg.Server.Host = options.Host
	}

	if options.LogPaths != "" {
		cfg.Server.LogPaths = strings.Split(options.LogPaths, ",")
		// 清理路径
		for i, path := range cfg.Server.LogPaths {
			cfg.Server.LogPaths[i] = strings.TrimSpace(path)
		}
	}

	if options.MaxFileSize != "" {
		size, err := parseFileSize(options.MaxFileSize)
		if err != nil {
			return fmt.Errorf("无效的文件大小格式: %s", options.MaxFileSize)
		}
		cfg.Server.MaxFileSize = size
	}

	if options.CacheSize > 0 {
		cfg.Server.CacheSize = options.CacheSize
	}

	// 安全配置
	if options.EnableAuth {
		cfg.Security.EnableAuth = true
	}

	if options.Username != "" {
		cfg.Security.Username = options.Username
	}

	if options.Password != "" {
		cfg.Security.Password = options.Password
	}

	if options.AllowedIPs != "" {
		cfg.Security.AllowedIPs = strings.Split(options.AllowedIPs, ",")
		// 清理IP地址
		for i, ip := range cfg.Security.AllowedIPs {
			cfg.Security.AllowedIPs[i] = strings.TrimSpace(ip)
		}
	}

	// 日志配置
	if options.LogLevel != "" {
		cfg.Logging.Level = options.LogLevel
	}

	// TLS配置
	if options.EnableTLS {
		cfg.Security.TLS.Enabled = true
	}

	if options.CertFile != "" {
		cfg.Security.TLS.CertFile = options.CertFile
	}

	if options.KeyFile != "" {
		cfg.Security.TLS.KeyFile = options.KeyFile
	}

	if options.AutoCert {
		cfg.Security.TLS.AutoCert = true
	}

	return nil
}

// parseFileSize 解析文件大小字符串 (支持 K, M, G 单位)
func parseFileSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))

	// 匹配数字和单位
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([KMG]?)B?$`)
	matches := re.FindStringSubmatch(sizeStr)
	if len(matches) != 3 {
		return 0, fmt.Errorf("无效的文件大小格式，支持格式: 100, 100K, 100M, 100G")
	}

	// 解析数字部分
	num, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("无效的数字: %s", matches[1])
	}

	// 应用单位
	unit := matches[2]
	switch unit {
	case "K":
		num *= 1024
	case "M":
		num *= 1024 * 1024
	case "G":
		num *= 1024 * 1024 * 1024
	}

	return int64(num), nil
}

// loadFromFile 从文件加载配置
func loadFromFile(cfg *Config, configPath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s", configPath)
	}

	// 读取文件内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	return nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 验证服务器配置
	if err := c.validateServerConfig(); err != nil {
		return fmt.Errorf("服务器配置错误: %w", err)
	}

	// 验证日志配置
	if err := c.validateLoggingConfig(); err != nil {
		return fmt.Errorf("日志配置错误: %w", err)
	}

	// 验证安全配置
	if err := c.validateSecurityConfig(); err != nil {
		return fmt.Errorf("安全配置错误: %w", err)
	}

	return nil
}

// validateServerConfig 验证服务器配置
func (c *Config) validateServerConfig() error {
	// 验证端口范围
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("端口号必须在1-65535范围内: %d", c.Server.Port)
	}

	// 验证主机地址
	if c.Server.Host != "" {
		if net.ParseIP(c.Server.Host) == nil && c.Server.Host != "localhost" {
			return fmt.Errorf("无效的主机地址: %s", c.Server.Host)
		}
	}

	// 验证日志路径
	if len(c.Server.LogPaths) == 0 {
		return fmt.Errorf("至少需要指定一个日志路径")
	}

	// 检查日志路径是否存在
	for _, path := range c.Server.LogPaths {
		if path == "" {
			return fmt.Errorf("日志路径不能为空")
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("无效的日志路径: %s", path)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return fmt.Errorf("日志路径不存在: %s", absPath)
		}
	}

	// 验证最大文件大小
	if c.Server.MaxFileSize <= 0 {
		return fmt.Errorf("最大文件大小必须大于0")
	}

	// 验证缓存大小
	if c.Server.CacheSize <= 0 {
		return fmt.Errorf("缓存大小必须大于0")
	}

	return nil
}

// validateLoggingConfig 验证日志配置
func (c *Config) validateLoggingConfig() error {
	// 验证日志级别
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLevels[strings.ToLower(c.Logging.Level)] {
		return fmt.Errorf("无效的日志级别: %s，支持的级别: debug, info, warn, error", c.Logging.Level)
	}

	// 验证日志格式
	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}

	if !validFormats[strings.ToLower(c.Logging.Format)] {
		return fmt.Errorf("无效的日志格式: %s，支持的格式: json, text", c.Logging.Format)
	}

	// 验证输出路径
	if c.Logging.OutputPath != "" && c.Logging.OutputPath != "stdout" && c.Logging.OutputPath != "stderr" {
		// 如果是文件路径，检查目录是否存在
		dir := filepath.Dir(c.Logging.OutputPath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("日志输出目录不存在: %s", dir)
		}
	}

	return nil
}

// validateSecurityConfig 验证安全配置
func (c *Config) validateSecurityConfig() error {
	// 验证认证配置
	if c.Security.EnableAuth {
		if c.Security.Username == "" {
			return fmt.Errorf("启用认证时必须设置用户名")
		}
		if c.Security.Password == "" {
			return fmt.Errorf("启用认证时必须设置密码")
		}
		if len(c.Security.Password) < 6 {
			return fmt.Errorf("密码长度至少为6位")
		}
	}

	// 验证IP白名单
	for _, ip := range c.Security.AllowedIPs {
		if ip == "" {
			continue
		}

		// 支持CIDR格式
		if strings.Contains(ip, "/") {
			_, _, err := net.ParseCIDR(ip)
			if err != nil {
				return fmt.Errorf("无效的CIDR格式IP: %s", ip)
			}
		} else {
			if net.ParseIP(ip) == nil {
				return fmt.Errorf("无效的IP地址: %s", ip)
			}
		}
	}

	// 验证TLS配置
	if c.Security.TLS.Enabled {
		if !c.Security.TLS.AutoCert {
			if c.Security.TLS.CertFile == "" {
				return fmt.Errorf("启用TLS时必须设置证书文件路径或启用自动证书生成")
			}
			if c.Security.TLS.KeyFile == "" {
				return fmt.Errorf("启用TLS时必须设置私钥文件路径或启用自动证书生成")
			}
		}
	}

	return nil
}

// Save 保存配置到文件
func (c *Config) Save(configPath string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}
