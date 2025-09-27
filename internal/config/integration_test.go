package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIntegrationCommandLineOptions(t *testing.T) {
	// 创建临时目录用于测试
	tempDir, err := os.MkdirTemp("", "config_integration_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 测试完整的命令行选项集合
	options := &CommandLineOptions{
		Port:        9999,
		Host:        "127.0.0.1",
		LogPaths:    tempDir + "," + tempDir + "/subdir",
		MaxFileSize: "256M",
		CacheSize:   200,
		EnableAuth:  true,
		Username:    "testuser",
		Password:    "testpassword123",
		AllowedIPs:  "192.168.1.0/24,10.0.0.1",
		LogLevel:    "debug",
	}

	// 创建子目录
	subDir := filepath.Join(tempDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("创建子目录失败: %v", err)
	}

	cfg, err := LoadWithOptions(options)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证所有选项都被正确应用
	if cfg.Server.Port != 9999 {
		t.Errorf("端口设置错误: 期望 9999，实际 %d", cfg.Server.Port)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("主机设置错误: 期望 '127.0.0.1'，实际 '%s'", cfg.Server.Host)
	}

	expectedPaths := []string{tempDir, subDir}
	if len(cfg.Server.LogPaths) != 2 {
		t.Errorf("日志路径数量错误: 期望 2，实际 %d", len(cfg.Server.LogPaths))
	} else {
		for i, expected := range expectedPaths {
			if cfg.Server.LogPaths[i] != expected {
				t.Errorf("日志路径[%d]错误: 期望 '%s'，实际 '%s'", i, expected, cfg.Server.LogPaths[i])
			}
		}
	}

	expectedSize := int64(256 * 1024 * 1024)
	if cfg.Server.MaxFileSize != expectedSize {
		t.Errorf("最大文件大小错误: 期望 %d，实际 %d", expectedSize, cfg.Server.MaxFileSize)
	}

	if cfg.Server.CacheSize != 200 {
		t.Errorf("缓存大小错误: 期望 200，实际 %d", cfg.Server.CacheSize)
	}

	if !cfg.Security.EnableAuth {
		t.Errorf("认证设置错误: 期望启用，实际未启用")
	}

	if cfg.Security.Username != "testuser" {
		t.Errorf("用户名错误: 期望 'testuser'，实际 '%s'", cfg.Security.Username)
	}

	if cfg.Security.Password != "testpassword123" {
		t.Errorf("密码错误: 期望 'testpassword123'，实际 '%s'", cfg.Security.Password)
	}

	expectedIPs := []string{"192.168.1.0/24", "10.0.0.1"}
	if len(cfg.Security.AllowedIPs) != 2 {
		t.Errorf("允许IP数量错误: 期望 2，实际 %d", len(cfg.Security.AllowedIPs))
	} else {
		for i, expected := range expectedIPs {
			if cfg.Security.AllowedIPs[i] != expected {
				t.Errorf("允许IP[%d]错误: 期望 '%s'，实际 '%s'", i, expected, cfg.Security.AllowedIPs[i])
			}
		}
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("日志级别错误: 期望 'debug'，实际 '%s'", cfg.Logging.Level)
	}
}

func TestConfigFileAndCommandLineInteraction(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "config_interaction_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建配置文件
	configContent := `
server:
  host: "192.168.1.100"
  port: 3000
  logPaths:
    - "` + tempDir + `"
  maxFileSize: 50000000
  cacheSize: 25

logging:
  level: "info"
  format: "json"
  outputPath: "stdout"

security:
  enableAuth: false
  username: ""
  password: ""
  allowedIPs: []
`

	configFile := filepath.Join(tempDir, "test_config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	// 使用配置文件和命令行参数
	options := &CommandLineOptions{
		ConfigPath: configFile,
		Port:       8888,        // 覆盖配置文件中的端口
		Host:       "127.0.0.1", // 覆盖配置文件中的主机
		EnableAuth: true,        // 覆盖配置文件中的认证设置
		Username:   "cmduser",
		Password:   "cmdpass123",
		LogLevel:   "warn", // 覆盖配置文件中的日志级别
	}

	cfg, err := LoadWithOptions(options)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证命令行参数覆盖了配置文件
	if cfg.Server.Port != 8888 {
		t.Errorf("端口应该被命令行覆盖: 期望 8888，实际 %d", cfg.Server.Port)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("主机应该被命令行覆盖: 期望 '127.0.0.1'，实际 '%s'", cfg.Server.Host)
	}

	if !cfg.Security.EnableAuth {
		t.Errorf("认证应该被命令行启用")
	}

	if cfg.Security.Username != "cmduser" {
		t.Errorf("用户名应该被命令行覆盖: 期望 'cmduser'，实际 '%s'", cfg.Security.Username)
	}

	if cfg.Logging.Level != "warn" {
		t.Errorf("日志级别应该被命令行覆盖: 期望 'warn'，实际 '%s'", cfg.Logging.Level)
	}

	// 验证未被覆盖的配置文件值仍然有效
	if cfg.Server.MaxFileSize != 50000000 {
		t.Errorf("最大文件大小应该保持配置文件的值: 期望 50000000，实际 %d", cfg.Server.MaxFileSize)
	}

	if cfg.Server.CacheSize != 25 {
		t.Errorf("缓存大小应该保持配置文件的值: 期望 25，实际 %d", cfg.Server.CacheSize)
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		options *CommandLineOptions
		wantErr bool
	}{
		{
			name: "无效的文件大小格式",
			options: &CommandLineOptions{
				LogPaths:    "./logs",
				MaxFileSize: "invalid-size",
			},
			wantErr: true,
		},
		{
			name: "不存在的配置文件",
			options: &CommandLineOptions{
				ConfigPath: "/nonexistent/config.yaml",
			},
			wantErr: true,
		},
		{
			name: "无效的端口号",
			options: &CommandLineOptions{
				Port:     99999,
				LogPaths: "./logs",
			},
			wantErr: true,
		},
		{
			name: "启用认证但密码太短",
			options: &CommandLineOptions{
				LogPaths:   "./logs",
				EnableAuth: true,
				Username:   "user",
				Password:   "123", // 太短
			},
			wantErr: true,
		},
	}

	// 创建logs目录用于测试
	os.MkdirAll("./logs", 0755)
	defer os.RemoveAll("./logs")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadWithOptions(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadWithOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
