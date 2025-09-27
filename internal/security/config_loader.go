package security

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/logger"
)

// ConfigLoader 安全配置动态加载器
type ConfigLoader struct {
	configPath      string
	currentConfig   *config.SecurityConfig
	mu              sync.RWMutex
	updateCallbacks []func(*config.SecurityConfig)
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// NewConfigLoader 创建新的配置加载器
func NewConfigLoader(configPath string, initialConfig *config.SecurityConfig) *ConfigLoader {
	return &ConfigLoader{
		configPath:      configPath,
		currentConfig:   initialConfig,
		updateCallbacks: make([]func(*config.SecurityConfig), 0),
		stopCh:          make(chan struct{}),
	}
}

// GetConfig 获取当前安全配置
func (cl *ConfigLoader) GetConfig() *config.SecurityConfig {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	// 返回配置的副本以避免并发修改
	configCopy := *cl.currentConfig
	return &configCopy
}

// UpdateConfig 更新安全配置
func (cl *ConfigLoader) UpdateConfig(newConfig *config.SecurityConfig) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	// 验证新配置
	if err := ValidateConfig(newConfig); err != nil {
		return fmt.Errorf("invalid security configuration: %w", err)
	}

	// 更新配置
	oldConfig := cl.currentConfig
	cl.currentConfig = newConfig

	logger.Info("security configuration updated",
		zap.Bool("auth_enabled", newConfig.EnableAuth),
		zap.Int("allowed_ips_count", len(newConfig.AllowedIPs)),
		zap.Bool("tls_enabled", newConfig.TLS.Enabled))

	// 通知所有回调函数
	for _, callback := range cl.updateCallbacks {
		go func(cb func(*config.SecurityConfig)) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("security config callback panic", zap.Any("panic", r))
				}
			}()
			cb(newConfig)
		}(callback)
	}

	// 记录配置变更
	cl.logConfigChanges(oldConfig, newConfig)

	return nil
}

// RegisterUpdateCallback 注册配置更新回调
func (cl *ConfigLoader) RegisterUpdateCallback(callback func(*config.SecurityConfig)) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.updateCallbacks = append(cl.updateCallbacks, callback)
}

// StartWatching 开始监控配置文件变化
func (cl *ConfigLoader) StartWatching(ctx context.Context, interval time.Duration) {
	if cl.configPath == "" {
		logger.Debug("no config path specified, skipping file watching")
		return
	}

	cl.wg.Add(1)
	go func() {
		defer cl.wg.Done()
		cl.watchConfigFile(ctx, interval)
	}()
}

// Stop 停止配置加载器
func (cl *ConfigLoader) Stop() {
	close(cl.stopCh)
	cl.wg.Wait()
}

// watchConfigFile 监控配置文件变化
func (cl *ConfigLoader) watchConfigFile(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastModTime time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-cl.stopCh:
			return
		case <-ticker.C:
			if err := cl.checkAndReloadConfig(&lastModTime); err != nil {
				logger.Error("failed to reload security config", zap.Error(err))
			}
		}
	}
}

// checkAndReloadConfig 检查并重新加载配置
func (cl *ConfigLoader) checkAndReloadConfig(lastModTime *time.Time) error {
	// 这里简化实现，实际应该检查文件修改时间
	// 并在文件变化时重新加载配置

	// 尝试重新加载配置
	newConfig, err := config.LoadWithOptions(&config.CommandLineOptions{
		ConfigPath: cl.configPath,
	})
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 检查安全配置是否有变化
	currentConfig := cl.GetConfig()
	if !cl.isConfigEqual(currentConfig, &newConfig.Security) {
		logger.Info("security configuration file changed, reloading")
		return cl.UpdateConfig(&newConfig.Security)
	}

	return nil
}

// isConfigEqual 比较两个安全配置是否相等
func (cl *ConfigLoader) isConfigEqual(a, b *config.SecurityConfig) bool {
	if a.EnableAuth != b.EnableAuth {
		return false
	}
	if a.Username != b.Username {
		return false
	}
	if a.Password != b.Password {
		return false
	}
	if len(a.AllowedIPs) != len(b.AllowedIPs) {
		return false
	}
	for i, ip := range a.AllowedIPs {
		if ip != b.AllowedIPs[i] {
			return false
		}
	}
	if a.TLS.Enabled != b.TLS.Enabled {
		return false
	}
	if a.TLS.CertFile != b.TLS.CertFile {
		return false
	}
	if a.TLS.KeyFile != b.TLS.KeyFile {
		return false
	}
	if a.TLS.AutoCert != b.TLS.AutoCert {
		return false
	}
	return true
}

// logConfigChanges 记录配置变更
func (cl *ConfigLoader) logConfigChanges(oldConfig, newConfig *config.SecurityConfig) {
	changes := make([]string, 0)

	if oldConfig.EnableAuth != newConfig.EnableAuth {
		changes = append(changes, fmt.Sprintf("auth_enabled: %v -> %v", oldConfig.EnableAuth, newConfig.EnableAuth))
	}

	if oldConfig.Username != newConfig.Username {
		changes = append(changes, "username changed")
	}

	if oldConfig.Password != newConfig.Password {
		changes = append(changes, "password changed")
	}

	if len(oldConfig.AllowedIPs) != len(newConfig.AllowedIPs) {
		changes = append(changes, fmt.Sprintf("allowed_ips_count: %d -> %d", len(oldConfig.AllowedIPs), len(newConfig.AllowedIPs)))
	}

	if oldConfig.TLS.Enabled != newConfig.TLS.Enabled {
		changes = append(changes, fmt.Sprintf("tls_enabled: %v -> %v", oldConfig.TLS.Enabled, newConfig.TLS.Enabled))
	}

	if len(changes) > 0 {
		logger.Info("security configuration changes detected", zap.Strings("changes", changes))
	}
}

// ValidateConfig 验证安全配置
func ValidateConfig(cfg *config.SecurityConfig) error {
	return validateSecurityConfig(cfg)
}

// validateSecurityConfig 验证安全配置（从config包复制的逻辑）
func validateSecurityConfig(c *config.SecurityConfig) error {
	// 验证认证配置
	if c.EnableAuth {
		if c.Username == "" {
			return fmt.Errorf("启用认证时必须设置用户名")
		}
		if c.Password == "" {
			return fmt.Errorf("启用认证时必须设置密码")
		}
		if len(c.Password) < 6 {
			return fmt.Errorf("密码长度至少为6位")
		}
	}

	// 验证IP白名单
	for _, ip := range c.AllowedIPs {
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
	if c.TLS.Enabled {
		if !c.TLS.AutoCert {
			if c.TLS.CertFile == "" {
				return fmt.Errorf("启用TLS时必须设置证书文件路径或启用自动证书生成")
			}
			if c.TLS.KeyFile == "" {
				return fmt.Errorf("启用TLS时必须设置私钥文件路径或启用自动证书生成")
			}
		}
	}

	return nil
}
