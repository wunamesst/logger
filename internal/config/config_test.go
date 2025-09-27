package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// 验证默认值
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("期望默认主机为 '0.0.0.0'，实际为 '%s'", cfg.Server.Host)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("期望默认端口为 8080，实际为 %d", cfg.Server.Port)
	}

	if len(cfg.Server.LogPaths) != 1 || cfg.Server.LogPaths[0] != "./logs" {
		t.Errorf("期望默认日志路径为 ['./logs']，实际为 %v", cfg.Server.LogPaths)
	}

	if cfg.Server.MaxFileSize != 100*1024*1024 {
		t.Errorf("期望默认最大文件大小为 100MB，实际为 %d", cfg.Server.MaxFileSize)
	}

	if cfg.Server.CacheSize != 50 {
		t.Errorf("期望默认缓存大小为 50，实际为 %d", cfg.Server.CacheSize)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("期望默认日志级别为 'info'，实际为 '%s'", cfg.Logging.Level)
	}

	if cfg.Security.EnableAuth != false {
		t.Errorf("期望默认不启用认证，实际为 %v", cfg.Security.EnableAuth)
	}
}

func TestParseFileSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"100", 100, false},
		{"100B", 100, false},
		{"1K", 1024, false},
		{"1KB", 1024, false},
		{"1M", 1024 * 1024, false},
		{"1MB", 1024 * 1024, false},
		{"1G", 1024 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"1.5M", int64(1.5 * 1024 * 1024), false},
		{"invalid", 0, true},
		{"", 0, true},
		{"1T", 0, true}, // 不支持TB
	}

	for _, test := range tests {
		result, err := parseFileSize(test.input)
		if test.hasError {
			if err == nil {
				t.Errorf("期望 '%s' 解析失败，但成功了", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("期望 '%s' 解析成功，但失败了: %v", test.input, err)
			}
			if result != test.expected {
				t.Errorf("期望 '%s' 解析为 %d，实际为 %d", test.input, test.expected, result)
			}
		}
	}
}

func TestLoadWithOptions(t *testing.T) {
	// 创建临时目录用于测试
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 测试基本选项
	options := &CommandLineOptions{
		Port:        9090,
		Host:        "127.0.0.1",
		LogPaths:    tempDir,
		MaxFileSize: "50M",
		CacheSize:   100,
		LogLevel:    "debug",
	}

	cfg, err := LoadWithOptions(options)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("期望端口为 9090，实际为 %d", cfg.Server.Port)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("期望主机为 '127.0.0.1'，实际为 '%s'", cfg.Server.Host)
	}

	if len(cfg.Server.LogPaths) != 1 || cfg.Server.LogPaths[0] != tempDir {
		t.Errorf("期望日志路径为 ['%s']，实际为 %v", tempDir, cfg.Server.LogPaths)
	}

	if cfg.Server.MaxFileSize != 50*1024*1024 {
		t.Errorf("期望最大文件大小为 50MB，实际为 %d", cfg.Server.MaxFileSize)
	}

	if cfg.Server.CacheSize != 100 {
		t.Errorf("期望缓存大小为 100，实际为 %d", cfg.Server.CacheSize)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("期望日志级别为 'debug'，实际为 '%s'", cfg.Logging.Level)
	}
}

func TestLoadWithConfigFile(t *testing.T) {
	// 创建临时目录和配置文件
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configContent := `
server:
  host: "192.168.1.100"
  port: 3000
  logPaths:
    - "` + tempDir + `"
  maxFileSize: 200000000
  cacheSize: 25

logging:
  level: "warn"
  format: "text"
  outputPath: "stderr"

security:
  enableAuth: true
  username: "testuser"
  password: "testpass123"
  allowedIPs:
    - "192.168.1.0/24"
`

	configFile := filepath.Join(tempDir, "test_config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	options := &CommandLineOptions{
		ConfigPath: configFile,
	}

	cfg, err := LoadWithOptions(options)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证配置文件中的值
	if cfg.Server.Host != "192.168.1.100" {
		t.Errorf("期望主机为 '192.168.1.100'，实际为 '%s'", cfg.Server.Host)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("期望端口为 3000，实际为 %d", cfg.Server.Port)
	}

	if cfg.Logging.Level != "warn" {
		t.Errorf("期望日志级别为 'warn'，实际为 '%s'", cfg.Logging.Level)
	}

	if cfg.Security.EnableAuth != true {
		t.Errorf("期望启用认证，实际为 %v", cfg.Security.EnableAuth)
	}

	if cfg.Security.Username != "testuser" {
		t.Errorf("期望用户名为 'testuser'，实际为 '%s'", cfg.Security.Username)
	}
}

func TestCommandLineOverridesConfigFile(t *testing.T) {
	// 创建临时目录和配置文件
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configContent := `
server:
  host: "192.168.1.100"
  port: 3000
  logPaths:
    - "` + tempDir + `"
`

	configFile := filepath.Join(tempDir, "test_config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	// 命令行参数应该覆盖配置文件
	options := &CommandLineOptions{
		ConfigPath: configFile,
		Port:       9999,
		Host:       "127.0.0.1",
	}

	cfg, err := LoadWithOptions(options)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证命令行参数覆盖了配置文件
	if cfg.Server.Port != 9999 {
		t.Errorf("期望端口为 9999（命令行覆盖），实际为 %d", cfg.Server.Port)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("期望主机为 '127.0.0.1'（命令行覆盖），实际为 '%s'", cfg.Server.Host)
	}
}

func TestValidateServerConfig(t *testing.T) {
	// 创建临时目录用于测试
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name:      "有效配置",
			config:    DefaultConfig(),
			expectErr: false,
		},
		{
			name: "无效端口 - 太小",
			config: &Config{
				Server: ServerConfig{
					Port:        0,
					Host:        "0.0.0.0",
					LogPaths:    []string{tempDir},
					MaxFileSize: 100 * 1024 * 1024,
					CacheSize:   50,
				},
				Logging:  DefaultConfig().Logging,
				Security: DefaultConfig().Security,
			},
			expectErr: true,
		},
		{
			name: "无效端口 - 太大",
			config: &Config{
				Server: ServerConfig{
					Port:        70000,
					Host:        "0.0.0.0",
					LogPaths:    []string{tempDir},
					MaxFileSize: 100 * 1024 * 1024,
					CacheSize:   50,
				},
				Logging:  DefaultConfig().Logging,
				Security: DefaultConfig().Security,
			},
			expectErr: true,
		},
		{
			name: "无效主机地址",
			config: &Config{
				Server: ServerConfig{
					Port:        8080,
					Host:        "invalid-host",
					LogPaths:    []string{tempDir},
					MaxFileSize: 100 * 1024 * 1024,
					CacheSize:   50,
				},
				Logging:  DefaultConfig().Logging,
				Security: DefaultConfig().Security,
			},
			expectErr: true,
		},
		{
			name: "空日志路径",
			config: &Config{
				Server: ServerConfig{
					Port:        8080,
					Host:        "0.0.0.0",
					LogPaths:    []string{},
					MaxFileSize: 100 * 1024 * 1024,
					CacheSize:   50,
				},
				Logging:  DefaultConfig().Logging,
				Security: DefaultConfig().Security,
			},
			expectErr: true,
		},
		{
			name: "不存在的日志路径",
			config: &Config{
				Server: ServerConfig{
					Port:        8080,
					Host:        "0.0.0.0",
					LogPaths:    []string{"/nonexistent/path"},
					MaxFileSize: 100 * 1024 * 1024,
					CacheSize:   50,
				},
				Logging:  DefaultConfig().Logging,
				Security: DefaultConfig().Security,
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 对于默认配置，需要创建日志目录
			if test.name == "有效配置" {
				logDir := "./logs"
				os.MkdirAll(logDir, 0755)
				defer os.RemoveAll(logDir)
			}

			err := test.config.Validate()
			if test.expectErr && err == nil {
				t.Errorf("期望验证失败，但成功了")
			}
			if !test.expectErr && err != nil {
				t.Errorf("期望验证成功，但失败了: %v", err)
			}
		})
	}
}

func TestValidateSecurityConfig(t *testing.T) {
	// 创建临时目录用于测试
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name      string
		security  SecurityConfig
		expectErr bool
	}{
		{
			name: "有效的安全配置 - 未启用认证",
			security: SecurityConfig{
				EnableAuth: false,
			},
			expectErr: false,
		},
		{
			name: "有效的安全配置 - 启用认证",
			security: SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "password123",
				AllowedIPs: []string{"192.168.1.1", "10.0.0.0/8"},
			},
			expectErr: false,
		},
		{
			name: "启用认证但缺少用户名",
			security: SecurityConfig{
				EnableAuth: true,
				Password:   "password123",
			},
			expectErr: true,
		},
		{
			name: "启用认证但缺少密码",
			security: SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
			},
			expectErr: true,
		},
		{
			name: "密码太短",
			security: SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "123",
			},
			expectErr: true,
		},
		{
			name: "无效的IP地址",
			security: SecurityConfig{
				EnableAuth: false,
				AllowedIPs: []string{"invalid-ip"},
			},
			expectErr: true,
		},
		{
			name: "无效的CIDR格式",
			security: SecurityConfig{
				EnableAuth: false,
				AllowedIPs: []string{"192.168.1.0/99"},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				Server: ServerConfig{
					Port:        8080,
					Host:        "0.0.0.0",
					LogPaths:    []string{tempDir},
					MaxFileSize: 100 * 1024 * 1024,
					CacheSize:   50,
				},
				Logging:  DefaultConfig().Logging,
				Security: test.security,
			}

			err := cfg.Validate()
			if test.expectErr && err == nil {
				t.Errorf("期望验证失败，但成功了")
			}
			if !test.expectErr && err != nil {
				t.Errorf("期望验证成功，但失败了: %v", err)
			}
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建配置
	originalCfg := &Config{
		Server: ServerConfig{
			Host:        "192.168.1.100",
			Port:        3000,
			LogPaths:    []string{tempDir},
			MaxFileSize: 200 * 1024 * 1024,
			CacheSize:   25,
		},
		Logging: LogConfig{
			Level:      "debug",
			Format:     "text",
			OutputPath: "stderr",
		},
		Security: SecurityConfig{
			EnableAuth: true,
			Username:   "testuser",
			Password:   "testpass123",
			AllowedIPs: []string{"192.168.1.0/24"},
		},
	}

	// 保存配置
	configFile := filepath.Join(tempDir, "test_save.yaml")
	err = originalCfg.Save(configFile)
	if err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}

	// 加载配置
	options := &CommandLineOptions{
		ConfigPath: configFile,
	}
	loadedCfg, err := LoadWithOptions(options)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 比较配置
	if loadedCfg.Server.Host != originalCfg.Server.Host {
		t.Errorf("主机地址不匹配: 期望 %s，实际 %s", originalCfg.Server.Host, loadedCfg.Server.Host)
	}

	if loadedCfg.Server.Port != originalCfg.Server.Port {
		t.Errorf("端口不匹配: 期望 %d，实际 %d", originalCfg.Server.Port, loadedCfg.Server.Port)
	}

	if loadedCfg.Logging.Level != originalCfg.Logging.Level {
		t.Errorf("日志级别不匹配: 期望 %s，实际 %s", originalCfg.Logging.Level, loadedCfg.Logging.Level)
	}

	if loadedCfg.Security.EnableAuth != originalCfg.Security.EnableAuth {
		t.Errorf("认证设置不匹配: 期望 %v，实际 %v", originalCfg.Security.EnableAuth, loadedCfg.Security.EnableAuth)
	}
}
