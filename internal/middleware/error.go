package middleware

import (
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/local-log-viewer/internal/errors"
	"github.com/local-log-viewer/internal/logger"
	"github.com/local-log-viewer/internal/types"
)

// ErrorHandler 统一错误处理中间件
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// 记录panic信息
				logger.Error("panic recovered",
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					zap.String("client_ip", c.ClientIP()),
				)

				// 返回内部服务器错误
				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, types.ErrorResponse{
						Code:    http.StatusInternalServerError,
						Message: "服务器内部错误",
						Details: "系统发生了意外错误，请稍后重试",
					})
				}
				c.Abort()
			}
		}()

		c.Next()

		// 处理错误
		if len(c.Errors) > 0 {
			handleErrors(c)
		}
	}
}

// handleErrors 处理错误列表
func handleErrors(c *gin.Context) {
	// 获取最后一个错误
	lastError := c.Errors.Last()
	err := lastError.Err

	// 记录错误信息
	logger.Error("request error",
		zap.Error(err),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.Request.UserAgent()),
	)

	// 如果已经写入响应，则不再处理
	if c.Writer.Written() {
		return
	}

	// 处理应用程序错误
	if appErr, ok := err.(*errors.AppError); ok {
		handleAppError(c, appErr)
		return
	}

	// 处理其他类型的错误
	handleGenericError(c, err)
}

// handleAppError 处理应用程序错误
func handleAppError(c *gin.Context, appErr *errors.AppError) {
	statusCode := appErr.ToHTTPStatus()
	userMessage := appErr.GetUserMessage()

	response := types.ErrorResponse{
		Code:    statusCode,
		Message: userMessage,
	}

	// 在开发模式下提供更多详细信息
	if gin.Mode() == gin.DebugMode {
		response.Details = appErr.Message
		if appErr.Details != "" {
			response.Details += " (" + appErr.Details + ")"
		}
	}

	c.JSON(statusCode, response)
}

// handleGenericError 处理通用错误
func handleGenericError(c *gin.Context, err error) {
	// 根据错误类型推断状态码
	statusCode := http.StatusInternalServerError
	message := "服务器内部错误"

	// 可以根据错误字符串内容进行更精确的分类
	errorStr := err.Error()
	switch {
	case contains(errorStr, "not found", "no such file"):
		statusCode = http.StatusNotFound
		message = "请求的资源不存在"
	case contains(errorStr, "permission denied", "access denied"):
		statusCode = http.StatusForbidden
		message = "没有权限访问该资源"
	case contains(errorStr, "timeout", "deadline exceeded"):
		statusCode = http.StatusRequestTimeout
		message = "请求超时，请稍后重试"
	case contains(errorStr, "invalid", "bad request"):
		statusCode = http.StatusBadRequest
		message = "请求参数错误"
	}

	response := types.ErrorResponse{
		Code:    statusCode,
		Message: message,
	}

	// 在开发模式下提供错误详情
	if gin.Mode() == gin.DebugMode {
		response.Details = err.Error()
	}

	c.JSON(statusCode, response)
}

// contains 检查字符串是否包含任意一个子字符串（忽略大小写）
func contains(s string, substrings ...string) bool {
	s = strings.ToLower(s)
	for _, substr := range substrings {
		if strings.Contains(s, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}

// RequestLogger 请求日志中间件
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 计算延迟
		latency := time.Since(start)

		// 获取状态码
		statusCode := c.Writer.Status()

		// 构建完整路径
		if raw != "" {
			path = path + "?" + raw
		}

		// 记录请求日志
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int64("body_size", int64(c.Writer.Size())),
		}

		// 根据状态码选择日志级别
		switch {
		case statusCode >= 500:
			logger.Error("server error", fields...)
		case statusCode >= 400:
			logger.Warn("client error", fields...)
		default:
			logger.Info("request completed", fields...)
		}
	}
}
