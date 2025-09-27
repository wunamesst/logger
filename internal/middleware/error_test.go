package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/errors"
	"github.com/local-log-viewer/internal/logger"
	"github.com/local-log-viewer/internal/types"
)

func init() {
	// 初始化测试用的logger
	gin.SetMode(gin.TestMode)
	logger.Initialize(config.LogConfig{
		Level:      "debug",
		Format:     "json",
		OutputPath: "stdout",
	})
}

func TestErrorHandler_AppError(t *testing.T) {
	router := gin.New()
	router.Use(ErrorHandler())

	router.GET("/test", func(c *gin.Context) {
		appErr := errors.NewFileNotFoundError("/test/path", nil)
		c.Error(appErr)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response types.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Code != http.StatusNotFound {
		t.Errorf("Expected code %d, got %d", http.StatusNotFound, response.Code)
	}

	expectedMessage := "请求的文件不存在"
	if response.Message != expectedMessage {
		t.Errorf("Expected message %s, got %s", expectedMessage, response.Message)
	}
}

func TestErrorHandler_GenericError(t *testing.T) {
	router := gin.New()
	router.Use(ErrorHandler())

	router.GET("/test", func(c *gin.Context) {
		c.Error(errors.NewInternalError("generic error", nil))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var response types.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Code != http.StatusInternalServerError {
		t.Errorf("Expected code %d, got %d", http.StatusInternalServerError, response.Code)
	}
}

func TestErrorHandler_Panic(t *testing.T) {
	router := gin.New()
	router.Use(ErrorHandler())

	router.GET("/test", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var response types.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Message != "服务器内部错误" {
		t.Errorf("Expected panic error message, got %s", response.Message)
	}
}
func TestErrorHandler_NoError(t *testing.T) {
	router := gin.New()
	router.Use(ErrorHandler())

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["message"] != "success" {
		t.Errorf("Expected success message, got %v", response["message"])
	}
}

func TestRequestLogger(t *testing.T) {
	router := gin.New()
	router.Use(RequestLogger())

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test?param=value", nil)
	req.Header.Set("User-Agent", "test-agent")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// 请求日志中间件主要是记录日志，这里主要验证它不会影响正常的请求处理
}

func TestContains(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		substrings []string
		expected   bool
	}{
		{
			name:       "contains single substring",
			s:          "file not found",
			substrings: []string{"not found"},
			expected:   true,
		},
		{
			name:       "contains multiple substrings",
			s:          "permission denied error",
			substrings: []string{"not found", "permission denied"},
			expected:   true,
		},
		{
			name:       "case insensitive",
			s:          "FILE NOT FOUND",
			substrings: []string{"not found"},
			expected:   true,
		},
		{
			name:       "does not contain",
			s:          "some other error",
			substrings: []string{"not found", "permission denied"},
			expected:   false,
		},
		{
			name:       "empty substrings",
			s:          "any string",
			substrings: []string{},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.s, tt.substrings...); got != tt.expected {
				t.Errorf("contains() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorHandler_DebugMode(t *testing.T) {
	// 保存原始模式
	originalMode := gin.Mode()
	defer gin.SetMode(originalMode)

	// 设置为调试模式
	gin.SetMode(gin.DebugMode)

	router := gin.New()
	router.Use(ErrorHandler())

	router.GET("/test", func(c *gin.Context) {
		appErr := &errors.AppError{
			Type:    errors.ErrorTypeFileNotFound,
			Message: "detailed error message",
			Details: "additional details",
		}
		c.Error(appErr)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	var response types.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 在调试模式下应该包含详细信息
	if response.Details == "" {
		t.Error("Expected details in debug mode")
	}

	expectedDetails := "detailed error message (additional details)"
	if response.Details != expectedDetails {
		t.Errorf("Expected details %s, got %s", expectedDetails, response.Details)
	}
}
