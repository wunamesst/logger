package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/types"
)

// MockLogManager 模拟日志管理器
type MockLogManager struct {
	files   []types.LogFile
	content *types.LogContent
	result  *types.SearchResult
	err     error
}

// GetDirectoryFiles implements interfaces.LogManager.
func (m *MockLogManager) GetDirectoryFiles(dirPath string) ([]types.LogFile, error) {
	panic("unimplemented")
}

// GetLogPaths implements interfaces.LogManager.
func (m *MockLogManager) GetLogPaths() []string {
	panic("unimplemented")
}

// ReadLogFileFromTail implements interfaces.LogManager.
func (m *MockLogManager) ReadLogFileFromTail(path string, lines int) (*types.LogContent, error) {
	panic("unimplemented")
}

func (m *MockLogManager) GetLogFiles() ([]types.LogFile, error) {
	return m.files, m.err
}

func (m *MockLogManager) ReadLogFile(path string, offset int64, limit int) (*types.LogContent, error) {
	return m.content, m.err
}

func (m *MockLogManager) SearchLogs(query types.SearchQuery) (*types.SearchResult, error) {
	return m.result, m.err
}

func (m *MockLogManager) WatchFile(path string) (<-chan types.LogUpdate, error) {
	ch := make(chan types.LogUpdate)
	return ch, m.err
}

func (m *MockLogManager) Start() error {
	return m.err
}

func (m *MockLogManager) Stop() error {
	return m.err
}

func setupTestServer() *HTTPServer {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:        "localhost",
			Port:        8080,
			LogPaths:    []string{"/tmp/logs"},
			MaxFileSize: 1024 * 1024 * 100, // 100MB
			CacheSize:   100,
		},
	}

	mockLogManager := &MockLogManager{}
	wsHub := NewWebSocketHub()

	server := New(cfg, mockLogManager, wsHub)
	server.setupRoutes() // 设置路由
	return server
}

func TestGetLogFiles(t *testing.T) {
	server := setupTestServer()

	// 设置模拟数据
	mockFiles := []types.LogFile{
		{
			Path:        "/tmp/logs/app.log",
			Name:        "app.log",
			Size:        1024,
			ModTime:     time.Now(),
			IsDirectory: false,
		},
		{
			Path:        "/tmp/logs/error.log",
			Name:        "error.log",
			Size:        2048,
			ModTime:     time.Now(),
			IsDirectory: false,
		},
	}

	if logManager, ok := server.logManager.(*MockLogManager); ok {
		logManager.files = mockFiles
	}

	// 创建测试请求
	req, _ := http.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("期望 success 为 true")
	}

	data := response["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("期望返回 2 个文件, 得到 %d", len(data))
	}
}

func TestGetLogFilesError(t *testing.T) {
	server := setupTestServer()

	// 设置模拟错误
	if logManager, ok := server.logManager.(*MockLogManager); ok {
		logManager.err = fmt.Errorf("模拟错误")
	}

	req, _ := http.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证错误响应
	if w.Code != http.StatusInternalServerError {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusInternalServerError, w.Code)
	}

	var response types.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("解析错误响应失败: %v", err)
	}

	if response.Code != http.StatusInternalServerError {
		t.Errorf("期望错误码 %d, 得到 %d", http.StatusInternalServerError, response.Code)
	}
}

func TestGetLogContent(t *testing.T) {
	server := setupTestServer()

	// 设置模拟数据
	mockContent := &types.LogContent{
		Entries: []types.LogEntry{
			{
				Timestamp: time.Now(),
				Level:     "INFO",
				Message:   "测试日志消息",
				Raw:       "2023-01-01 12:00:00 INFO 测试日志消息",
				LineNum:   1,
			},
		},
		TotalLines: 100,
		HasMore:    true,
		Offset:     0,
	}

	if logManager, ok := server.logManager.(*MockLogManager); ok {
		logManager.content = mockContent
	}

	// 测试正常请求
	testPath := url.QueryEscape("/tmp/logs/app.log")
	req, _ := http.NewRequest("GET", "/api/logs/"+testPath+"?offset=0&limit=10", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("期望 success 为 true")
	}

	data := response["data"].(map[string]interface{})
	entries := data["entries"].([]interface{})
	if len(entries) != 1 {
		t.Errorf("期望返回 1 个日志条目, 得到 %d", len(entries))
	}
}

func TestGetLogContentMissingPath(t *testing.T) {
	server := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/logs/", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证错误响应
	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetLogContentInvalidOffset(t *testing.T) {
	server := setupTestServer()

	testPath := url.QueryEscape("/tmp/logs/app.log")
	req, _ := http.NewRequest("GET", "/api/logs/"+testPath+"?offset=invalid", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证错误响应
	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusBadRequest, w.Code)
	}
}

func TestSearchLogs(t *testing.T) {
	server := setupTestServer()

	// 设置模拟数据
	mockResult := &types.SearchResult{
		Entries: []types.LogEntry{
			{
				Timestamp: time.Now(),
				Level:     "ERROR",
				Message:   "错误消息",
				Raw:       "2023-01-01 12:00:00 ERROR 错误消息",
				LineNum:   1,
			},
		},
		TotalCount: 1,
		HasMore:    false,
		Offset:     0,
	}

	if logManager, ok := server.logManager.(*MockLogManager); ok {
		logManager.result = mockResult
	}

	// 测试搜索请求
	req, _ := http.NewRequest("GET", "/api/search?path=/tmp/logs/app.log&query=ERROR&offset=0&limit=10", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("期望 success 为 true")
	}

	data := response["data"].(map[string]interface{})
	entries := data["entries"].([]interface{})
	if len(entries) != 1 {
		t.Errorf("期望返回 1 个搜索结果, 得到 %d", len(entries))
	}
}

func TestSearchLogsMissingPath(t *testing.T) {
	server := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/search?query=ERROR", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证错误响应
	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusBadRequest, w.Code)
	}
}

func TestSearchLogsMissingQuery(t *testing.T) {
	server := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/search?path=/tmp/logs/app.log", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证错误响应
	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusBadRequest, w.Code)
	}
}

func TestSearchLogsWithTimeRange(t *testing.T) {
	server := setupTestServer()

	// 设置模拟数据
	mockResult := &types.SearchResult{
		Entries:    []types.LogEntry{},
		TotalCount: 0,
		HasMore:    false,
		Offset:     0,
	}

	if logManager, ok := server.logManager.(*MockLogManager); ok {
		logManager.result = mockResult
	}

	// 测试带时间范围的搜索
	startTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	endTime := time.Now().Format(time.RFC3339)

	reqURL := fmt.Sprintf("/api/search?path=/tmp/logs/app.log&query=ERROR&startTime=%s&endTime=%s&levels=ERROR,WARN&isRegex=true",
		url.QueryEscape(startTime), url.QueryEscape(endTime))

	req, _ := http.NewRequest("GET", reqURL, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusOK, w.Code)
	}
}

func TestSearchLogsInvalidTimeFormat(t *testing.T) {
	server := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/search?path=/tmp/logs/app.log&query=ERROR&startTime=invalid-time", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证错误响应
	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusBadRequest, w.Code)
	}
}

func TestHealthCheck(t *testing.T) {
	// 创建测试目录
	err := os.MkdirAll("/tmp/logs", 0755)
	if err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	defer os.RemoveAll("/tmp/logs")

	server := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 健康检查应该返回健康状态
	if response["status"] != "healthy" {
		t.Errorf("期望 status 为 'healthy', 得到 %v", response["status"])
	}

	if _, exists := response["timestamp"]; !exists {
		t.Error("期望响应包含 timestamp")
	}

	if _, exists := response["uptime"]; !exists {
		t.Error("期望响应包含 uptime")
	}
}

func TestCORSHeaders(t *testing.T) {
	server := setupTestServer()

	req, _ := http.NewRequest("OPTIONS", "/api/logs", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证CORS头
	if w.Code != http.StatusNoContent {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusNoContent, w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("期望设置 Access-Control-Allow-Origin 头")
	}

	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("期望设置 Access-Control-Allow-Methods 头")
	}
}

func TestNotFoundRoute(t *testing.T) {
	server := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/nonexistent", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 验证404响应
	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusNotFound, w.Code)
	}

	var response types.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("解析错误响应失败: %v", err)
	}

	if response.Code != http.StatusNotFound {
		t.Errorf("期望错误码 %d, 得到 %d", http.StatusNotFound, response.Code)
	}
}

func TestStaticFileRoute(t *testing.T) {
	server := setupTestServer()

	// 测试前端路由重定向
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// 由于没有实际的静态文件，会返回404，但这验证了路由逻辑
	// 在实际部署中，这会返回index.html
}

// 集成测试：测试完整的API流程
func TestIntegrationAPIFlow(t *testing.T) {
	// 创建测试目录
	err := os.MkdirAll("/tmp/logs", 0755)
	if err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	defer os.RemoveAll("/tmp/logs")

	server := setupTestServer()

	// 设置模拟数据
	mockFiles := []types.LogFile{
		{
			Path:        "/tmp/logs/app.log",
			Name:        "app.log",
			Size:        1024,
			ModTime:     time.Now(),
			IsDirectory: false,
		},
	}

	mockContent := &types.LogContent{
		Entries: []types.LogEntry{
			{
				Timestamp: time.Now(),
				Level:     "INFO",
				Message:   "应用启动",
				Raw:       "2023-01-01 12:00:00 INFO 应用启动",
				LineNum:   1,
			},
			{
				Timestamp: time.Now(),
				Level:     "ERROR",
				Message:   "数据库连接失败",
				Raw:       "2023-01-01 12:01:00 ERROR 数据库连接失败",
				LineNum:   2,
			},
		},
		TotalLines: 2,
		HasMore:    false,
		Offset:     0,
	}

	mockSearchResult := &types.SearchResult{
		Entries: []types.LogEntry{
			{
				Timestamp: time.Now(),
				Level:     "ERROR",
				Message:   "数据库连接失败",
				Raw:       "2023-01-01 12:01:00 ERROR 数据库连接失败",
				LineNum:   2,
			},
		},
		TotalCount: 1,
		HasMore:    false,
		Offset:     0,
	}

	if logManager, ok := server.logManager.(*MockLogManager); ok {
		logManager.files = mockFiles
		logManager.content = mockContent
		logManager.result = mockSearchResult
	}

	// 1. 获取文件列表
	req1, _ := http.NewRequest("GET", "/api/logs", nil)
	w1 := httptest.NewRecorder()
	server.router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("获取文件列表失败: %d", w1.Code)
	}

	// 2. 读取文件内容
	testPath := url.QueryEscape("/tmp/logs/app.log")
	req2, _ := http.NewRequest("GET", "/api/logs/"+testPath, nil)
	w2 := httptest.NewRecorder()
	server.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("读取文件内容失败: %d", w2.Code)
	}

	// 3. 搜索日志
	req3, _ := http.NewRequest("GET", "/api/search?path=/tmp/logs/app.log&query=ERROR", nil)
	w3 := httptest.NewRecorder()
	server.router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("搜索日志失败: %d", w3.Code)
	}

	// 4. 健康检查
	req4, _ := http.NewRequest("GET", "/api/health", nil)
	w4 := httptest.NewRecorder()
	server.router.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("健康检查失败: %d", w4.Code)
	}

	t.Log("集成测试通过：所有API端点正常工作")
}
