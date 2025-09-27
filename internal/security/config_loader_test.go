package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/local-log-viewer/internal/config"
)

func TestConfigLoader(t *testing.T) {
	initialConfig := &config.SecurityConfig{
		EnableAuth: false,
		Username:   "",
		Password:   "",
		AllowedIPs: []string{},
		TLS: config.TLSConfig{
			Enabled: false,
		},
	}

	loader := NewConfigLoader("", initialConfig)

	t.Run("GetConfig", func(t *testing.T) {
		cfg := loader.GetConfig()
		assert.Equal(t, initialConfig.EnableAuth, cfg.EnableAuth)
		assert.Equal(t, initialConfig.Username, cfg.Username)
		assert.Equal(t, initialConfig.Password, cfg.Password)
		assert.Equal(t, len(initialConfig.AllowedIPs), len(cfg.AllowedIPs))
	})

	t.Run("UpdateConfig", func(t *testing.T) {
		newConfig := &config.SecurityConfig{
			EnableAuth: true,
			Username:   "admin",
			Password:   "password123",
			AllowedIPs: []string{"192.168.1.0/24"},
			TLS: config.TLSConfig{
				Enabled: false,
			},
		}

		err := loader.UpdateConfig(newConfig)
		require.NoError(t, err)

		cfg := loader.GetConfig()
		assert.Equal(t, newConfig.EnableAuth, cfg.EnableAuth)
		assert.Equal(t, newConfig.Username, cfg.Username)
		assert.Equal(t, newConfig.Password, cfg.Password)
		assert.Equal(t, len(newConfig.AllowedIPs), len(cfg.AllowedIPs))
	})

	t.Run("UpdateConfig with invalid config", func(t *testing.T) {
		invalidConfig := &config.SecurityConfig{
			EnableAuth: true,
			Username:   "admin",
			Password:   "123", // 密码太短
			AllowedIPs: []string{},
			TLS: config.TLSConfig{
				Enabled: false,
			},
		}

		err := loader.UpdateConfig(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid security configuration")
	})

	t.Run("RegisterUpdateCallback", func(t *testing.T) {
		callbackCalled := false
		var receivedConfig *config.SecurityConfig

		loader.RegisterUpdateCallback(func(cfg *config.SecurityConfig) {
			callbackCalled = true
			receivedConfig = cfg
		})

		newConfig := &config.SecurityConfig{
			EnableAuth: true,
			Username:   "test",
			Password:   "testpass123",
			AllowedIPs: []string{"127.0.0.1"},
			TLS: config.TLSConfig{
				Enabled: false,
			},
		}

		err := loader.UpdateConfig(newConfig)
		require.NoError(t, err)

		// 等待回调执行
		time.Sleep(10 * time.Millisecond)

		assert.True(t, callbackCalled)
		assert.NotNil(t, receivedConfig)
		assert.Equal(t, newConfig.Username, receivedConfig.Username)
	})
}

func TestIsConfigEqual(t *testing.T) {
	loader := NewConfigLoader("", &config.SecurityConfig{})

	tests := []struct {
		name     string
		config1  *config.SecurityConfig
		config2  *config.SecurityConfig
		expected bool
	}{
		{
			name: "identical configs",
			config1: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "pass123",
				AllowedIPs: []string{"192.168.1.1"},
				TLS: config.TLSConfig{
					Enabled: false,
				},
			},
			config2: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "pass123",
				AllowedIPs: []string{"192.168.1.1"},
				TLS: config.TLSConfig{
					Enabled: false,
				},
			},
			expected: true,
		},
		{
			name: "different auth enabled",
			config1: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "pass123",
			},
			config2: &config.SecurityConfig{
				EnableAuth: false,
				Username:   "admin",
				Password:   "pass123",
			},
			expected: false,
		},
		{
			name: "different username",
			config1: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "pass123",
			},
			config2: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "user",
				Password:   "pass123",
			},
			expected: false,
		},
		{
			name: "different password",
			config1: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "pass123",
			},
			config2: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "different",
			},
			expected: false,
		},
		{
			name: "different allowed IPs",
			config1: &config.SecurityConfig{
				AllowedIPs: []string{"192.168.1.1"},
			},
			config2: &config.SecurityConfig{
				AllowedIPs: []string{"192.168.1.2"},
			},
			expected: false,
		},
		{
			name: "different TLS enabled",
			config1: &config.SecurityConfig{
				TLS: config.TLSConfig{
					Enabled: true,
				},
			},
			config2: &config.SecurityConfig{
				TLS: config.TLSConfig{
					Enabled: false,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := loader.isConfigEqual(tt.config1, tt.config2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStartWatching(t *testing.T) {
	initialConfig := &config.SecurityConfig{
		EnableAuth: false,
	}

	loader := NewConfigLoader("", initialConfig)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 启动监控（由于没有配置文件路径，应该直接返回）
	loader.StartWatching(ctx, 10*time.Millisecond)

	// 等待上下文超时
	<-ctx.Done()

	// 停止加载器
	loader.Stop()
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.SecurityConfig
		expectError bool
	}{
		{
			name: "valid config - auth disabled",
			config: &config.SecurityConfig{
				EnableAuth: false,
			},
			expectError: false,
		},
		{
			name: "valid config - auth enabled",
			config: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "password123",
			},
			expectError: false,
		},
		{
			name: "invalid config - auth enabled without username",
			config: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "",
				Password:   "password123",
			},
			expectError: true,
		},
		{
			name: "invalid config - auth enabled with short password",
			config: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "123",
			},
			expectError: true,
		},
		{
			name: "invalid config - invalid IP",
			config: &config.SecurityConfig{
				EnableAuth: false,
				AllowedIPs: []string{"invalid-ip"},
			},
			expectError: true,
		},
		{
			name: "valid config - TLS with auto cert",
			config: &config.SecurityConfig{
				EnableAuth: false,
				TLS: config.TLSConfig{
					Enabled:  true,
					AutoCert: true,
				},
			},
			expectError: false,
		},
		{
			name: "invalid config - TLS without cert files",
			config: &config.SecurityConfig{
				EnableAuth: false,
				TLS: config.TLSConfig{
					Enabled:  true,
					AutoCert: false,
					CertFile: "",
					KeyFile:  "",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
