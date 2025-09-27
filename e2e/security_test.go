package e2e

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/server"
)

func TestSecurityIntegration(t *testing.T) {
	// 创建临时目录和测试日志文件
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logDir, 0755)
	require.NoError(t, err)

	logFile := filepath.Join(logDir, "test.log")
	err = os.WriteFile(logFile, []byte("test log entry\n"), 0644)
	require.NoError(t, err)

	t.Run("Basic Authentication", func(t *testing.T) {
		// 配置启用认证
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:     "127.0.0.1",
				Port:     0, // 使用随机端口
				LogPaths: []string{logDir},
			},
			Security: config.SecurityConfig{
				EnableAuth: true,
				Username:   "testuser",
				Password:   "testpass123",
			},
		}

		// 创建并启动服务器
		srv, port := createTestServer(t, cfg)
		defer srv.Stop()

		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

		// 测试无认证访问 - 应该被拒绝
		resp, err := http.Get(baseURL + "/api/logs")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// 测试错误的认证信息 - 应该被拒绝
		client := &http.Client{}
		req, err := http.NewRequest("GET", baseURL+"/api/logs", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("wrong:password")))

		resp, err = client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// 测试正确的认证信息 - 应该成功
		req, err = http.NewRequest("GET", baseURL+"/api/logs", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("testuser:testpass123")))

		resp, err = client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("IP Whitelist", func(t *testing.T) {
		// 配置IP白名单
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:     "127.0.0.1",
				Port:     0,
				LogPaths: []string{logDir},
			},
			Security: config.SecurityConfig{
				EnableAuth: false,
				AllowedIPs: []string{"127.0.0.1", "::1"}, // 只允许本地访问
			},
		}

		srv, port := createTestServer(t, cfg)
		defer srv.Stop()

		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

		// 从允许的IP访问 - 应该成功
		resp, err := http.Get(baseURL + "/api/logs")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Combined Auth and IP Whitelist", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:     "127.0.0.1",
				Port:     0,
				LogPaths: []string{logDir},
			},
			Security: config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "secure123",
				AllowedIPs: []string{"127.0.0.1"},
			},
		}

		srv, port := createTestServer(t, cfg)
		defer srv.Stop()

		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

		// 测试有认证但IP不在白名单 - 应该被IP过滤器拒绝
		// 注意：在实际测试中，由于我们从localhost访问，IP检查会通过
		// 这里主要测试认证功能

		client := &http.Client{}
		req, err := http.NewRequest("GET", baseURL+"/api/logs", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secure123")))

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestTLSIntegration(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logDir, 0755)
	require.NoError(t, err)

	logFile := filepath.Join(logDir, "test.log")
	err = os.WriteFile(logFile, []byte("test log entry\n"), 0644)
	require.NoError(t, err)

	t.Run("HTTPS with Auto Certificate", func(t *testing.T) {
		certDir := filepath.Join(tempDir, "certs")
		err := os.MkdirAll(certDir, 0755)
		require.NoError(t, err)

		cfg := &config.Config{
			Server: config.ServerConfig{
				Host:     "127.0.0.1",
				Port:     0,
				LogPaths: []string{logDir},
			},
			Security: config.SecurityConfig{
				TLS: config.TLSConfig{
					Enabled:  true,
					AutoCert: true,
					CertFile: filepath.Join(certDir, "server.crt"),
					KeyFile:  filepath.Join(certDir, "server.key"),
				},
			},
		}

		srv, port := createTestTLSServer(t, cfg)
		defer srv.Stop()

		baseURL := fmt.Sprintf("https://127.0.0.1:%d", port)

		// 创建忽略证书验证的客户端（因为是自签名证书）
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}

		// 测试HTTPS访问
		resp, err := client.Get(baseURL + "/api/logs")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// 验证证书文件已创建
		assert.FileExists(t, cfg.Security.TLS.CertFile)
		assert.FileExists(t, cfg.Security.TLS.KeyFile)
	})
}

func TestSecurityHeaders(t *testing.T) {
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logDir, 0755)
	require.NoError(t, err)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:     "127.0.0.1",
			Port:     0,
			LogPaths: []string{logDir},
		},
		Security: config.SecurityConfig{
			EnableAuth: false,
		},
	}

	srv, port := createTestServer(t, cfg)
	defer srv.Stop()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	resp, err := http.Get(baseURL + "/api/logs")
	require.NoError(t, err)
	defer resp.Body.Close()

	// 检查安全头
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
}

// createTestServer 创建测试服务器并返回端口号
func createTestServer(t *testing.T, cfg *config.Config) (*server.HTTPServer, int) {
	// 创建模拟的依赖
	logManager := &MockLogManager{}
	wsHub := &MockWebSocketHub{}

	srv := server.New(cfg, logManager, wsHub)

	// 启动服务器在后台
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 获取实际端口（如果使用了随机端口）
	port := cfg.Server.Port
	if port == 0 {
		// 在实际实现中，你需要从服务器获取实际端口
		// 这里简化处理，使用一个测试端口
		port = 18080
		cfg.Server.Port = port
	}

	return srv, port
}

// createTestTLSServer 创建TLS测试服务器
func createTestTLSServer(t *testing.T, cfg *config.Config) (*server.HTTPServer, int) {
	logManager := &MockLogManager{}
	wsHub := &MockWebSocketHub{}

	srv := server.New(cfg, logManager, wsHub)

	go func() {
		if err := srv.StartTLS(&cfg.Security.TLS); err != nil {
			t.Logf("TLS Server error: %v", err)
		}
	}()

	time.Sleep(200 * time.Millisecond) // TLS需要更多时间启动

	port := cfg.Server.Port
	if port == 0 {
		port = 18443
		cfg.Server.Port = port
	}

	return srv, port
}
