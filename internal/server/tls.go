package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/logger"
)

// TLSConfig is an alias for config.TLSConfig to avoid duplication
type TLSConfig = config.TLSConfig

// generateSelfSignedCert 生成自签名证书
func generateSelfSignedCert(certFile, keyFile string, hosts []string) error {
	logger.Info("generating self-signed certificate",
		zap.String("cert_file", certFile),
		zap.String("key_file", keyFile),
		zap.Strings("hosts", hosts))

	// 生成私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Local Log Viewer"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour), // 1年有效期
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{},
		DNSNames:    []string{},
	}

	// 添加主机名和IP地址
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}

	// 如果没有指定主机，添加默认值
	if len(template.IPAddresses) == 0 && len(template.DNSNames) == 0 {
		template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"))
		template.DNSNames = append(template.DNSNames, "localhost")
	}

	// 生成证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// 确保目录存在
	certDir := filepath.Dir(certFile)
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create certificate directory: %w", err)
	}

	keyDir := filepath.Dir(keyFile)
	if err := os.MkdirAll(keyDir, 0755); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// 写入证书文件
	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("failed to create certificate file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// 写入私钥文件
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// 设置文件权限
	if err := os.Chmod(certFile, 0644); err != nil {
		logger.Warn("failed to set certificate file permissions", zap.Error(err))
	}
	if err := os.Chmod(keyFile, 0600); err != nil {
		logger.Warn("failed to set key file permissions", zap.Error(err))
	}

	logger.Info("self-signed certificate generated successfully")
	return nil
}

// setupTLS 设置TLS配置
func (s *HTTPServer) setupTLS(tlsConfig *TLSConfig) (*tls.Config, error) {
	if !tlsConfig.Enabled {
		return nil, nil
	}

	// 如果启用自动证书生成
	if tlsConfig.AutoCert {
		// 默认证书路径
		if tlsConfig.CertFile == "" {
			tlsConfig.CertFile = "certs/server.crt"
		}
		if tlsConfig.KeyFile == "" {
			tlsConfig.KeyFile = "certs/server.key"
		}

		// 检查证书是否存在
		if _, err := os.Stat(tlsConfig.CertFile); os.IsNotExist(err) {
			// 生成自签名证书
			hosts := []string{"localhost", "127.0.0.1"}
			if s.config.Server.Host != "0.0.0.0" && s.config.Server.Host != "" {
				hosts = append(hosts, s.config.Server.Host)
			}

			if err := generateSelfSignedCert(tlsConfig.CertFile, tlsConfig.KeyFile, hosts); err != nil {
				return nil, fmt.Errorf("failed to generate self-signed certificate: %w", err)
			}
		}
	}

	// 验证证书文件存在
	if _, err := os.Stat(tlsConfig.CertFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("certificate file not found: %s", tlsConfig.CertFile)
	}
	if _, err := os.Stat(tlsConfig.KeyFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("key file not found: %s", tlsConfig.KeyFile)
	}

	// 加载证书
	cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	// 创建TLS配置
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12, // 最低TLS 1.2
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		},
		PreferServerCipherSuites: true,
	}

	return config, nil
}

// StartTLS 启动HTTPS服务器
func (s *HTTPServer) StartTLS(tlsConfig *TLSConfig) error {
	s.setupRoutes()

	// 设置TLS配置
	tlsConf, err := s.setupTLS(tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to setup TLS: %w", err)
	}

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port),
		Handler:      s.router,
		TLSConfig:    tlsConf,
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

	// 构建访问地址列表 (HTTPS)
	var accessURLs []string
	for _, ip := range localIPs {
		accessURLs = append(accessURLs, fmt.Sprintf("https://%s:%d", ip, s.config.Server.Port))
	}

	logger.Info("HTTPS服务器启动成功",
		zap.String("host", s.config.Server.Host),
		zap.Int("port", s.config.Server.Port),
		zap.String("cert_file", tlsConfig.CertFile),
		zap.Strings("log_paths", s.config.Server.LogPaths),
		zap.Strings("access_urls", accessURLs),
	)

	// 在控制台输出友好的访问信息
	fmt.Printf("\n🔒 HTTPS日志查看器启动成功!\n")
	fmt.Printf("📂 监控日志路径: %s\n", strings.Join(s.config.Server.LogPaths, ", "))
	fmt.Printf("🔐 TLS证书文件: %s\n", tlsConfig.CertFile)
	fmt.Printf("🌐 可通过以下地址访问:\n")
	for _, url := range accessURLs {
		fmt.Printf("   • %s\n", url)
	}
	fmt.Printf("\n⚠️  如使用自签名证书，浏览器会显示安全警告，点击继续访问即可\n")
	fmt.Printf("按 Ctrl+C 停止服务器\n\n")

	if err := s.server.ListenAndServeTLS(tlsConfig.CertFile, tlsConfig.KeyFile); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTPS server: %w", err)
	}

	return nil
}
