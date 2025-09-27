package errors

import (
	"fmt"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appError *AppError
		expected string
	}{
		{
			name: "error without cause",
			appError: &AppError{
				Type:    ErrorTypeFileNotFound,
				Message: "file not found",
			},
			expected: "FILE_NOT_FOUND: file not found",
		},
		{
			name: "error with cause",
			appError: &AppError{
				Type:    ErrorTypeFileNotFound,
				Message: "file not found",
				Cause:   fmt.Errorf("no such file or directory"),
			},
			expected: "FILE_NOT_FOUND: file not found (caused by: no such file or directory)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.appError.Error(); got != tt.expected {
				t.Errorf("AppError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppError_ToHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		appError *AppError
		expected int
	}{
		{
			name: "file not found",
			appError: &AppError{
				Type: ErrorTypeFileNotFound,
			},
			expected: http.StatusNotFound,
		},
		{
			name: "file permission",
			appError: &AppError{
				Type: ErrorTypeFilePermission,
			},
			expected: http.StatusForbidden,
		},
		{
			name: "auth failed",
			appError: &AppError{
				Type: ErrorTypeAuthFailed,
			},
			expected: http.StatusUnauthorized,
		},
		{
			name: "invalid format",
			appError: &AppError{
				Type: ErrorTypeInvalidFormat,
			},
			expected: http.StatusBadRequest,
		},
		{
			name: "file too large",
			appError: &AppError{
				Type: ErrorTypeFileTooLarge,
			},
			expected: http.StatusRequestEntityTooLarge,
		},
		{
			name: "search timeout",
			appError: &AppError{
				Type: ErrorTypeSearchTimeout,
			},
			expected: http.StatusRequestTimeout,
		},
		{
			name: "service unavailable",
			appError: &AppError{
				Type: ErrorTypeServiceUnavailable,
			},
			expected: http.StatusServiceUnavailable,
		},
		{
			name: "internal error",
			appError: &AppError{
				Type: ErrorTypeInternalError,
			},
			expected: http.StatusInternalServerError,
		},
		{
			name: "custom code",
			appError: &AppError{
				Type: ErrorTypeInternalError,
				Code: http.StatusTeapot,
			},
			expected: http.StatusTeapot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.appError.ToHTTPStatus(); got != tt.expected {
				t.Errorf("AppError.ToHTTPStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}
func TestAppError_GetUserMessage(t *testing.T) {
	tests := []struct {
		name     string
		appError *AppError
		expected string
	}{
		{
			name: "custom user message",
			appError: &AppError{
				Type:         ErrorTypeFileNotFound,
				UserFriendly: "自定义用户消息",
			},
			expected: "自定义用户消息",
		},
		{
			name: "file not found default message",
			appError: &AppError{
				Type: ErrorTypeFileNotFound,
			},
			expected: "请求的文件不存在",
		},
		{
			name: "file permission default message",
			appError: &AppError{
				Type: ErrorTypeFilePermission,
			},
			expected: "没有权限访问该文件",
		},
		{
			name: "auth failed default message",
			appError: &AppError{
				Type: ErrorTypeAuthFailed,
			},
			expected: "用户名或密码错误",
		},
		{
			name: "unknown error type",
			appError: &AppError{
				Type: "UNKNOWN_ERROR",
			},
			expected: "系统内部错误，请联系管理员",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.appError.GetUserMessage(); got != tt.expected {
				t.Errorf("AppError.GetUserMessage() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewFileNotFoundError(t *testing.T) {
	path := "/test/path"
	cause := fmt.Errorf("no such file")

	err := NewFileNotFoundError(path, cause)

	if err.Type != ErrorTypeFileNotFound {
		t.Errorf("Expected type %v, got %v", ErrorTypeFileNotFound, err.Type)
	}

	if err.Details != path {
		t.Errorf("Expected details %v, got %v", path, err.Details)
	}

	if err.Cause != cause {
		t.Errorf("Expected cause %v, got %v", cause, err.Cause)
	}
}

func TestNewFileTooLargeError(t *testing.T) {
	path := "/test/large/file"
	size := int64(1000)
	maxSize := int64(500)

	err := NewFileTooLargeError(path, size, maxSize)

	if err.Type != ErrorTypeFileTooLarge {
		t.Errorf("Expected type %v, got %v", ErrorTypeFileTooLarge, err.Type)
	}

	expectedDetails := "size: 1000, max: 500"
	if err.Details != expectedDetails {
		t.Errorf("Expected details %v, got %v", expectedDetails, err.Details)
	}
}

func TestWrapError(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	errorType := ErrorTypeParseFailure
	message := "wrapped error message"

	wrappedErr := WrapError(originalErr, errorType, message)

	if wrappedErr.Type != errorType {
		t.Errorf("Expected type %v, got %v", errorType, wrappedErr.Type)
	}

	if wrappedErr.Message != message {
		t.Errorf("Expected message %v, got %v", message, wrappedErr.Message)
	}

	if wrappedErr.Cause != originalErr {
		t.Errorf("Expected cause %v, got %v", originalErr, wrappedErr.Cause)
	}

	// Test Unwrap
	if wrappedErr.Unwrap() != originalErr {
		t.Errorf("Expected unwrapped error %v, got %v", originalErr, wrappedErr.Unwrap())
	}
}
