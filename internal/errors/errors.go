package errors

import (
	"fmt"
	"net/http"
)

// ErrorType 错误类型枚举
type ErrorType string

const (
	// 文件系统错误
	ErrorTypeFileNotFound   ErrorType = "FILE_NOT_FOUND"
	ErrorTypeFilePermission ErrorType = "FILE_PERMISSION"
	ErrorTypeFileCorrupted  ErrorType = "FILE_CORRUPTED"
	ErrorTypeFileTooLarge   ErrorType = "FILE_TOO_LARGE"
	ErrorTypeDiskFull       ErrorType = "DISK_FULL"

	// 网络错误
	ErrorTypeNetworkTimeout ErrorType = "NETWORK_TIMEOUT"
	ErrorTypeConnectionLost ErrorType = "CONNECTION_LOST"
	ErrorTypePortInUse      ErrorType = "PORT_IN_USE"

	// 解析错误
	ErrorTypeParseFailure  ErrorType = "PARSE_FAILURE"
	ErrorTypeInvalidFormat ErrorType = "INVALID_FORMAT"
	ErrorTypeEncodingError ErrorType = "ENCODING_ERROR"

	// 配置错误
	ErrorTypeConfigInvalid ErrorType = "CONFIG_INVALID"
	ErrorTypeConfigMissing ErrorType = "CONFIG_MISSING"

	// 认证错误
	ErrorTypeAuthFailed   ErrorType = "AUTH_FAILED"
	ErrorTypeAccessDenied ErrorType = "ACCESS_DENIED"

	// 搜索错误
	ErrorTypeSearchTimeout ErrorType = "SEARCH_TIMEOUT"
	ErrorTypeInvalidQuery  ErrorType = "INVALID_QUERY"

	// 系统错误
	ErrorTypeInternalError      ErrorType = "INTERNAL_ERROR"
	ErrorTypeServiceUnavailable ErrorType = "SERVICE_UNAVAILABLE"
)

// AppError 应用程序错误
type AppError struct {
	Type         ErrorType `json:"type"`
	Code         int       `json:"code"`
	Message      string    `json:"message"`
	Details      string    `json:"details,omitempty"`
	Cause        error     `json:"-"`
	UserFriendly string    `json:"userMessage,omitempty"`
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap 支持errors.Unwrap
func (e *AppError) Unwrap() error {
	return e.Cause
}

// ToHTTPStatus 转换为HTTP状态码
func (e *AppError) ToHTTPStatus() int {
	if e.Code > 0 {
		return e.Code
	}

	// 根据错误类型返回默认状态码
	switch e.Type {
	case ErrorTypeFileNotFound:
		return http.StatusNotFound
	case ErrorTypeFilePermission, ErrorTypeAccessDenied:
		return http.StatusForbidden
	case ErrorTypeAuthFailed:
		return http.StatusUnauthorized
	case ErrorTypeConfigInvalid, ErrorTypeInvalidQuery, ErrorTypeInvalidFormat:
		return http.StatusBadRequest
	case ErrorTypeFileTooLarge:
		return http.StatusRequestEntityTooLarge
	case ErrorTypeSearchTimeout, ErrorTypeNetworkTimeout:
		return http.StatusRequestTimeout
	case ErrorTypeServiceUnavailable, ErrorTypePortInUse:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// GetUserMessage 获取用户友好的错误消息
func (e *AppError) GetUserMessage() string {
	if e.UserFriendly != "" {
		return e.UserFriendly
	}

	// 根据错误类型返回默认用户消息
	switch e.Type {
	case ErrorTypeFileNotFound:
		return "请求的文件不存在"
	case ErrorTypeFilePermission:
		return "没有权限访问该文件"
	case ErrorTypeFileCorrupted:
		return "文件已损坏，无法读取"
	case ErrorTypeFileTooLarge:
		return "文件太大，无法处理"
	case ErrorTypeDiskFull:
		return "磁盘空间不足"
	case ErrorTypeNetworkTimeout:
		return "网络请求超时，请稍后重试"
	case ErrorTypeConnectionLost:
		return "网络连接已断开"
	case ErrorTypePortInUse:
		return "端口已被占用，请更换端口"
	case ErrorTypeParseFailure:
		return "日志格式解析失败"
	case ErrorTypeInvalidFormat:
		return "不支持的文件格式"
	case ErrorTypeEncodingError:
		return "文件编码错误"
	case ErrorTypeConfigInvalid:
		return "配置文件格式错误"
	case ErrorTypeConfigMissing:
		return "缺少必要的配置项"
	case ErrorTypeAuthFailed:
		return "用户名或密码错误"
	case ErrorTypeAccessDenied:
		return "访问被拒绝"
	case ErrorTypeSearchTimeout:
		return "搜索超时，请简化搜索条件"
	case ErrorTypeInvalidQuery:
		return "搜索条件格式错误"
	case ErrorTypeServiceUnavailable:
		return "服务暂时不可用，请稍后重试"
	default:
		return "系统内部错误，请联系管理员"
	}
}

// 错误构造函数

// NewFileNotFoundError 文件不存在错误
func NewFileNotFoundError(path string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeFileNotFound,
		Message: fmt.Sprintf("file not found: %s", path),
		Details: path,
		Cause:   cause,
	}
}

// NewFilePermissionError 文件权限错误
func NewFilePermissionError(path string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeFilePermission,
		Message: fmt.Sprintf("permission denied: %s", path),
		Details: path,
		Cause:   cause,
	}
}

// NewFileTooLargeError 文件过大错误
func NewFileTooLargeError(path string, size int64, maxSize int64) *AppError {
	return &AppError{
		Type:    ErrorTypeFileTooLarge,
		Message: fmt.Sprintf("file too large: %s (%d bytes, max: %d bytes)", path, size, maxSize),
		Details: fmt.Sprintf("size: %d, max: %d", size, maxSize),
	}
}

// NewParseError 解析错误
func NewParseError(format string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeParseFailure,
		Message: fmt.Sprintf("failed to parse %s format", format),
		Details: format,
		Cause:   cause,
	}
}

// NewConfigError 配置错误
func NewConfigError(field string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeConfigInvalid,
		Message: fmt.Sprintf("invalid configuration: %s", field),
		Details: field,
		Cause:   cause,
	}
}

// NewAuthError 认证错误
func NewAuthError(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeAuthFailed,
		Message: message,
	}
}

// NewSearchError 搜索错误
func NewSearchError(query string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeInvalidQuery,
		Message: fmt.Sprintf("invalid search query: %s", query),
		Details: query,
		Cause:   cause,
	}
}

// NewInternalError 内部错误
func NewInternalError(message string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeInternalError,
		Message: message,
		Cause:   cause,
	}
}

// WrapError 包装现有错误
func WrapError(err error, errorType ErrorType, message string) *AppError {
	return &AppError{
		Type:    errorType,
		Message: message,
		Cause:   err,
	}
}
