package server

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/local-log-viewer/internal/config"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "test.crt")
	keyFile := filepath.Join(tempDir, "test.key")

	hosts := []string{"localhost", "127.0.0.1", "example.com"}

	// 生成证书
	err := generateSelfSignedCert(certFile, keyFile, hosts)
	require.NoError(t, err)

	// 检查文件是否存在
	assert.FileExists(t, certFile)
	assert.FileExists(t, keyFile)

	// 验证证书可以加载
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	assert.NotNil(t, cert)

	// 解析证书内容
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)

	// 验证证书包含正确的主机名
	assert.Contains(t, x509Cert.DNSNames, "localhost")
	assert.Contains(t, x509Cert.DNSNames, "example.com")

	// 验证证书包含正确的IP地址
	found := false
	for _, ip := range x509Cert.IPAddresses {
		if ip.String() == "127.0.0.1" {
			found = true
			break
		}
	}
	assert.True(t, found, "Certificate should contain 127.0.0.1 IP address")

	// 验证证书有效期
	assert.True(t, x509Cert.NotAfter.After(x509Cert.NotBefore))
}

func TestGenerateSelfSignedCertWithDefaults(t *testing.T) {
	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "default.crt")
	keyFile := filepath.Join(tempDir, "default.key")

	// 不提供主机名，应该使用默认值
	err := generateSelfSignedCert(certFile, keyFile, []string{})
	require.NoError(t, err)

	// 验证证书
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)

	// 应该包含默认的localhost和127.0.0.1
	assert.Contains(t, x509Cert.DNSNames, "localhost")
	found := false
	for _, ip := range x509Cert.IPAddresses {
		if ip.String() == "127.0.0.1" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestSetupTLS(t *testing.T) {
	// 创建测试服务器
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}
	server := New(cfg, nil, nil)

	t.Run("TLS disabled", func(t *testing.T) {
		tlsConfig := &config.TLSConfig{
			Enabled: false,
		}

		result, err := server.setupTLS(tlsConfig)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("TLS enabled with auto cert", func(t *testing.T) {
		tempDir := t.TempDir()

		tlsConfig := &config.TLSConfig{
			Enabled:  true,
			AutoCert: true,
			CertFile: filepath.Join(tempDir, "auto.crt"),
			KeyFile:  filepath.Join(tempDir, "auto.key"),
		}

		result, err := server.setupTLS(tlsConfig)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, uint16(tls.VersionTLS12), result.MinVersion)
		assert.True(t, result.PreferServerCipherSuites)
		assert.Len(t, result.Certificates, 1)

		// 验证证书文件已创建
		assert.FileExists(t, tlsConfig.CertFile)
		assert.FileExists(t, tlsConfig.KeyFile)
	})

	t.Run("TLS enabled with existing cert", func(t *testing.T) {
		tempDir := t.TempDir()
		certFile := filepath.Join(tempDir, "existing.crt")
		keyFile := filepath.Join(tempDir, "existing.key")

		// 先生成证书
		err := generateSelfSignedCert(certFile, keyFile, []string{"localhost"})
		require.NoError(t, err)

		tlsConfig := &config.TLSConfig{
			Enabled:  true,
			AutoCert: false,
			CertFile: certFile,
			KeyFile:  keyFile,
		}

		result, err := server.setupTLS(tlsConfig)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Certificates, 1)
	})

	t.Run("TLS enabled with missing cert file", func(t *testing.T) {
		tlsConfig := &config.TLSConfig{
			Enabled:  true,
			AutoCert: false,
			CertFile: "/nonexistent/cert.crt",
			KeyFile:  "/nonexistent/key.key",
		}

		result, err := server.setupTLS(tlsConfig)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "certificate file not found")
	})

	t.Run("TLS enabled with missing key file", func(t *testing.T) {
		tempDir := t.TempDir()
		certFile := filepath.Join(tempDir, "cert.crt")

		// 创建一个空的证书文件
		err := os.WriteFile(certFile, []byte("dummy"), 0644)
		require.NoError(t, err)

		tlsConfig := &config.TLSConfig{
			Enabled:  true,
			AutoCert: false,
			CertFile: certFile,
			KeyFile:  "/nonexistent/key.key",
		}

		result, err := server.setupTLS(tlsConfig)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "key file not found")
	})
}

func TestTLSConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		tlsConfig   config.TLSConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "disabled TLS",
			tlsConfig: config.TLSConfig{
				Enabled: false,
			},
			expectError: false,
		},
		{
			name: "enabled with auto cert",
			tlsConfig: config.TLSConfig{
				Enabled:  true,
				AutoCert: true,
			},
			expectError: false,
		},
		{
			name: "enabled with cert files",
			tlsConfig: config.TLSConfig{
				Enabled:  true,
				AutoCert: false,
				CertFile: "cert.crt",
				KeyFile:  "key.key",
			},
			expectError: false,
		},
		{
			name: "enabled without cert file",
			tlsConfig: config.TLSConfig{
				Enabled:  true,
				AutoCert: false,
				CertFile: "",
				KeyFile:  "key.key",
			},
			expectError: true,
			errorMsg:    "启用TLS时必须设置证书文件路径或启用自动证书生成",
		},
		{
			name: "enabled without key file",
			tlsConfig: config.TLSConfig{
				Enabled:  true,
				AutoCert: false,
				CertFile: "cert.crt",
				KeyFile:  "",
			},
			expectError: true,
			errorMsg:    "启用TLS时必须设置私钥文件路径或启用自动证书生成",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			cfg := &config.Config{
				Server: config.ServerConfig{
					Host:        "127.0.0.1",
					Port:        8080,
					LogPaths:    []string{tempDir},
					MaxFileSize: 100 * 1024 * 1024,
					CacheSize:   50,
				},
				Logging: config.LogConfig{
					Level:      "info",
					Format:     "json",
					OutputPath: "stdout",
				},
				Security: config.SecurityConfig{
					TLS: tt.tlsConfig,
				},
			}

			err := cfg.Validate()
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
