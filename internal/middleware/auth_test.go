package middleware

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/local-log-viewer/internal/config"
)

func TestBasicAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		config         *config.SecurityConfig
		authHeader     string
		expectedStatus int
		expectedAuth   bool
	}{
		{
			name: "auth disabled - should pass",
			config: &config.SecurityConfig{
				EnableAuth: false,
			},
			authHeader:     "",
			expectedStatus: http.StatusOK,
			expectedAuth:   true,
		},
		{
			name: "auth enabled - no header",
			config: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "password123",
			},
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedAuth:   false,
		},
		{
			name: "auth enabled - invalid format",
			config: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "password123",
			},
			authHeader:     "Bearer token123",
			expectedStatus: http.StatusUnauthorized,
			expectedAuth:   false,
		},
		{
			name: "auth enabled - invalid base64",
			config: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "password123",
			},
			authHeader:     "Basic invalid-base64!",
			expectedStatus: http.StatusUnauthorized,
			expectedAuth:   false,
		},
		{
			name: "auth enabled - invalid credentials format",
			config: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "password123",
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("admin")),
			expectedStatus: http.StatusUnauthorized,
			expectedAuth:   false,
		},
		{
			name: "auth enabled - wrong username",
			config: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "password123",
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("wrong:password123")),
			expectedStatus: http.StatusUnauthorized,
			expectedAuth:   false,
		},
		{
			name: "auth enabled - wrong password",
			config: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "password123",
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:wrong")),
			expectedStatus: http.StatusUnauthorized,
			expectedAuth:   false,
		},
		{
			name: "auth enabled - correct credentials",
			config: &config.SecurityConfig{
				EnableAuth: true,
				Username:   "admin",
				Password:   "password123",
			},
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:password123")),
			expectedStatus: http.StatusOK,
			expectedAuth:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(BasicAuth(tt.config))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedAuth {
				assert.Contains(t, w.Body.String(), "success")
			} else {
				assert.Contains(t, w.Body.String(), "code")
			}
		})
	}
}

func TestIPWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		config         *config.SecurityConfig
		clientIP       string
		expectedStatus int
		expectedPass   bool
	}{
		{
			name: "no whitelist - should pass",
			config: &config.SecurityConfig{
				AllowedIPs: []string{},
			},
			clientIP:       "192.168.1.100",
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
		{
			name: "IP in whitelist - should pass",
			config: &config.SecurityConfig{
				AllowedIPs: []string{"192.168.1.100", "10.0.0.1"},
			},
			clientIP:       "192.168.1.100",
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
		{
			name: "IP not in whitelist - should block",
			config: &config.SecurityConfig{
				AllowedIPs: []string{"192.168.1.100", "10.0.0.1"},
			},
			clientIP:       "192.168.1.200",
			expectedStatus: http.StatusForbidden,
			expectedPass:   false,
		},
		{
			name: "CIDR range - IP in range",
			config: &config.SecurityConfig{
				AllowedIPs: []string{"192.168.1.0/24"},
			},
			clientIP:       "192.168.1.150",
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
		{
			name: "CIDR range - IP not in range",
			config: &config.SecurityConfig{
				AllowedIPs: []string{"192.168.1.0/24"},
			},
			clientIP:       "192.168.2.150",
			expectedStatus: http.StatusForbidden,
			expectedPass:   false,
		},
		{
			name: "localhost - should pass",
			config: &config.SecurityConfig{
				AllowedIPs: []string{"127.0.0.1"},
			},
			clientIP:       "127.0.0.1",
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(IPWhitelist(tt.config))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.clientIP)
			req.RemoteAddr = tt.clientIP + ":12345"

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedPass {
				assert.Contains(t, w.Body.String(), "success")
			} else {
				assert.Contains(t, w.Body.String(), "访问被拒绝")
			}
		})
	}
}

func TestIsIPAllowed(t *testing.T) {
	tests := []struct {
		name       string
		clientIP   string
		allowedIPs []string
		expected   bool
	}{
		{
			name:       "exact IP match",
			clientIP:   "192.168.1.100",
			allowedIPs: []string{"192.168.1.100", "10.0.0.1"},
			expected:   true,
		},
		{
			name:       "IP not in list",
			clientIP:   "192.168.1.200",
			allowedIPs: []string{"192.168.1.100", "10.0.0.1"},
			expected:   false,
		},
		{
			name:       "CIDR match",
			clientIP:   "192.168.1.150",
			allowedIPs: []string{"192.168.1.0/24"},
			expected:   true,
		},
		{
			name:       "CIDR no match",
			clientIP:   "192.168.2.150",
			allowedIPs: []string{"192.168.1.0/24"},
			expected:   false,
		},
		{
			name:       "mixed IP and CIDR",
			clientIP:   "10.0.0.5",
			allowedIPs: []string{"192.168.1.0/24", "10.0.0.5"},
			expected:   true,
		},
		{
			name:       "empty allowed list",
			clientIP:   "192.168.1.100",
			allowedIPs: []string{},
			expected:   false,
		},
		{
			name:       "invalid client IP",
			clientIP:   "invalid-ip",
			allowedIPs: []string{"192.168.1.100"},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIPAllowed(tt.clientIP, tt.allowedIPs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 检查安全头是否设置
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
}

func TestCombinedSecurityMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &config.SecurityConfig{
		EnableAuth: true,
		Username:   "admin",
		Password:   "secret123",
		AllowedIPs: []string{"192.168.1.0/24"},
	}

	router := gin.New()
	router.Use(SecurityHeaders())
	router.Use(IPWhitelist(config))
	router.Use(BasicAuth(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	t.Run("valid IP and auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret123")))
		req.Header.Set("X-Forwarded-For", "192.168.1.100")
		req.RemoteAddr = "192.168.1.100:12345"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("invalid IP", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret123")))
		req.Header.Set("X-Forwarded-For", "10.0.0.100")
		req.RemoteAddr = "10.0.0.100:12345"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("invalid auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:wrong")))
		req.Header.Set("X-Forwarded-For", "192.168.1.100")
		req.RemoteAddr = "192.168.1.100:12345"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
