package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/logger"
	"github.com/local-log-viewer/internal/types"
)

// BasicAuth 基本认证中间件
func BasicAuth(cfg *config.SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果未启用认证，直接通过
		if !cfg.EnableAuth {
			c.Next()
			return
		}

		// 获取Authorization头
		auth := c.GetHeader("Authorization")
		if auth == "" {
			logger.Debug("missing authorization header", zap.String("client_ip", c.ClientIP()))
			c.Header("WWW-Authenticate", `Basic realm="Log Viewer"`)
			c.JSON(http.StatusUnauthorized, types.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "需要认证",
				Details: "请提供用户名和密码",
			})
			c.Abort()
			return
		}

		// 检查Basic认证格式
		const prefix = "Basic "
		if !strings.HasPrefix(auth, prefix) {
			logger.Debug("invalid authorization format", zap.String("client_ip", c.ClientIP()))
			c.Header("WWW-Authenticate", `Basic realm="Log Viewer"`)
			c.JSON(http.StatusUnauthorized, types.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "认证格式错误",
				Details: "仅支持Basic认证",
			})
			c.Abort()
			return
		}

		// 解码Base64编码的凭据
		encoded := auth[len(prefix):]
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			logger.Debug("failed to decode credentials",
				zap.String("client_ip", c.ClientIP()),
				zap.Error(err))
			c.Header("WWW-Authenticate", `Basic realm="Log Viewer"`)
			c.JSON(http.StatusUnauthorized, types.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "认证信息解码失败",
			})
			c.Abort()
			return
		}

		// 解析用户名和密码
		credentials := string(decoded)
		parts := strings.SplitN(credentials, ":", 2)
		if len(parts) != 2 {
			logger.Debug("invalid credentials format", zap.String("client_ip", c.ClientIP()))
			c.Header("WWW-Authenticate", `Basic realm="Log Viewer"`)
			c.JSON(http.StatusUnauthorized, types.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "认证信息格式错误",
			})
			c.Abort()
			return
		}

		username := parts[0]
		password := parts[1]

		// 验证用户名和密码（使用常量时间比较防止时序攻击）
		usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Username)) == 1
		passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Password)) == 1

		if !usernameMatch || !passwordMatch {
			logger.Warn("authentication failed",
				zap.String("client_ip", c.ClientIP()),
				zap.String("username", username))
			c.Header("WWW-Authenticate", `Basic realm="Log Viewer"`)
			c.JSON(http.StatusUnauthorized, types.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "用户名或密码错误",
			})
			c.Abort()
			return
		}

		logger.Debug("authentication successful",
			zap.String("client_ip", c.ClientIP()),
			zap.String("username", username))

		// 认证成功，继续处理请求
		c.Next()
	}
}

// IPWhitelist IP白名单中间件
func IPWhitelist(cfg *config.SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果没有配置IP白名单，直接通过
		if len(cfg.AllowedIPs) == 0 {
			c.Next()
			return
		}

		clientIP := c.ClientIP()

		// 检查客户端IP是否在白名单中
		if !isIPAllowed(clientIP, cfg.AllowedIPs) {
			logger.Warn("IP access denied",
				zap.String("client_ip", clientIP),
				zap.Strings("allowed_ips", cfg.AllowedIPs))
			c.JSON(http.StatusForbidden, types.ErrorResponse{
				Code:    http.StatusForbidden,
				Message: "访问被拒绝",
				Details: "您的IP地址不在允许列表中",
			})
			c.Abort()
			return
		}

		logger.Debug("IP access granted",
			zap.String("client_ip", clientIP))

		c.Next()
	}
}

// isIPAllowed 检查IP是否在允许列表中
func isIPAllowed(clientIP string, allowedIPs []string) bool {
	clientIPAddr := net.ParseIP(clientIP)
	if clientIPAddr == nil {
		logger.Error("invalid client IP", zap.String("client_ip", clientIP))
		return false
	}

	for _, allowedIP := range allowedIPs {
		if allowedIP == "" {
			continue
		}

		// 检查是否为CIDR格式
		if strings.Contains(allowedIP, "/") {
			_, network, err := net.ParseCIDR(allowedIP)
			if err != nil {
				logger.Error("invalid CIDR in allowed IPs",
					zap.String("cidr", allowedIP),
					zap.Error(err))
				continue
			}
			if network.Contains(clientIPAddr) {
				return true
			}
		} else {
			// 直接IP地址比较
			allowedIPAddr := net.ParseIP(allowedIP)
			if allowedIPAddr != nil && allowedIPAddr.Equal(clientIPAddr) {
				return true
			}
		}
	}

	return false
}

// SecurityHeaders 安全头中间件
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置安全相关的HTTP头
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// 对于非开发环境，设置更严格的CSP
		if gin.Mode() != gin.DebugMode {
			c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:")
		}

		c.Next()
	}
}
