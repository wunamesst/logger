package benchmark

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/interfaces"
	"github.com/local-log-viewer/internal/server"
	"github.com/local-log-viewer/internal/types"
)

// MockLogManager 模拟日志管理器
type MockLogManager struct{}

// GetLogPaths implements interfaces.LogManager.
func (m *MockLogManager) GetLogPaths() []string {
	panic("unimplemented")
}

// ReadLogFileFromTail implements interfaces.LogManager.
func (m *MockLogManager) ReadLogFileFromTail(path string, lines int) (*types.LogContent, error) {
	panic("unimplemented")
}

func (m *MockLogManager) GetLogFiles() ([]types.LogFile, error) {
	return []types.LogFile{
		{Name: "small.log", Path: "small.log", Size: 1024, ModTime: time.Now()},
		{Name: "large.log", Path: "large.log", Size: 1024000, ModTime: time.Now()},
	}, nil
}

func (m *MockLogManager) ReadLogFile(path string, offset int64, limit int) (*types.LogContent, error) {
	entries := make([]types.LogEntry, limit)
	for i := 0; i < limit; i++ {
		entries[i] = types.LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   fmt.Sprintf("Mock log entry %d", i),
			Raw:       fmt.Sprintf("2024-01-01 10:00:00 INFO Mock log entry %d", i),
		}
	}
	return &types.LogContent{
		Entries:    entries,
		TotalLines: int64(limit * 10),
		HasMore:    true,
	}, nil
}

func (m *MockLogManager) SearchLogs(query types.SearchQuery) (*types.SearchResult, error) {
	entries := make([]types.LogEntry, 10)
	for i := 0; i < 10; i++ {
		entries[i] = types.LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Mock search result %d", i),
			Raw:       fmt.Sprintf("2024-01-01 10:00:00 ERROR Mock search result %d", i),
		}
	}
	return &types.SearchResult{
		Entries:    entries,
		TotalCount: 10,
		HasMore:    false,
		Offset:     0,
	}, nil
}

func (m *MockLogManager) WatchFile(path string) (<-chan types.LogUpdate, error) {
	ch := make(chan types.LogUpdate)
	close(ch)
	return ch, nil
}

func (m *MockLogManager) Start() error { return nil }
func (m *MockLogManager) Stop() error  { return nil }

// MockWebSocketHub 模拟WebSocket中心
type MockWebSocketHub struct{}

func (m *MockWebSocketHub) Run()                                               {}
func (m *MockWebSocketHub) BroadcastLogUpdate(update types.LogUpdate)          {}
func (m *MockWebSocketHub) RegisterClient(client interfaces.WebSocketClient)   {}
func (m *MockWebSocketHub) UnregisterClient(client interfaces.WebSocketClient) {}
func (m *MockWebSocketHub) Start() error                                       { return nil }
func (m *MockWebSocketHub) Stop() error                                        { return nil }

// BenchmarkAPIEndpoints 测试各个 API 端点的性能
func BenchmarkAPIEndpoints(b *testing.B) {
	// 设置测试环境
	testDir := setupBenchmarkEnvironment(b)
	defer os.RemoveAll(testDir)

	// 启动服务器
	srv, port := startTestServer(b, testDir)
	defer srv.Stop()

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	b.Run("HealthCheck", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := http.Get(baseURL + "/api/health")
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})

	b.Run("GetLogFiles", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := http.Get(baseURL + "/api/logs")
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})

	b.Run("ReadSmallFile", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := http.Get(baseURL + "/api/logs/small.log?limit=50")
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})

	b.Run("ReadLargeFile", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := http.Get(baseURL + "/api/logs/large.log?limit=100")
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})

	b.Run("SearchLogs", func(b *testing.B) {
		params := url.Values{}
		params.Set("path", "large.log")
		params.Set("query", "ERROR")
		params.Set("limit", "20")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := http.Get(baseURL + "/api/search?" + params.Encode())
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkConcurrentRequests 测试并发请求处理能力
func BenchmarkConcurrentRequests(b *testing.B) {
	testDir := setupBenchmarkEnvironment(b)
	defer os.RemoveAll(testDir)

	srv, port := startTestServer(b, testDir)
	defer srv.Stop()

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	concurrencyLevels := []int{1, 10, 50, 100, 200}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			b.SetParallelism(concurrency)
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					resp, err := http.Get(baseURL + "/api/health")
					if err != nil {
						b.Error(err)
						continue
					}
					resp.Body.Close()
				}
			})
		})
	}
}

// BenchmarkSearchPerformance 测试搜索性能
func BenchmarkSearchPerformance(b *testing.B) {
	testDir := setupBenchmarkEnvironment(b)
	defer os.RemoveAll(testDir)

	srv, port := startTestServer(b, testDir)
	defer srv.Stop()

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	searchQueries := []struct {
		name  string
		query string
		regex bool
	}{
		{"SimpleKeyword", "ERROR", false},
		{"MultipleKeywords", "ERROR|WARN", false},
		{"RegexPattern", `\d{4}-\d{2}-\d{2}`, true},
		{"ComplexRegex", `\b(error|fail|exception)\b`, true},
	}

	for _, sq := range searchQueries {
		b.Run(sq.name, func(b *testing.B) {
			params := url.Values{}
			params.Set("path", "large.log")
			params.Set("query", sq.query)
			params.Set("isRegex", fmt.Sprintf("%t", sq.regex))
			params.Set("limit", "50")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := http.Get(baseURL + "/api/search?" + params.Encode())
				if err != nil {
					b.Fatal(err)
				}

				var result types.SearchResult
				err = json.NewDecoder(resp.Body).Decode(&result)
				if err != nil {
					b.Fatal(err)
				}
				resp.Body.Close()
			}
		})
	}
}

// BenchmarkMemoryUsage 测试内存使用情况
func BenchmarkMemoryUsage(b *testing.B) {
	testDir := setupBenchmarkEnvironment(b)
	defer os.RemoveAll(testDir)

	srv, port := startTestServer(b, testDir)
	defer srv.Stop()

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	b.Run("MemoryStability", func(b *testing.B) {
		b.ResetTimer()

		// 连续读取大文件多次，测试内存是否稳定
		for i := 0; i < b.N; i++ {
			params := url.Values{}
			params.Set("offset", fmt.Sprintf("%d", (i%10)*100))
			params.Set("limit", "100")

			resp, err := http.Get(baseURL + "/api/logs/large.log?" + params.Encode())
			if err != nil {
				b.Fatal(err)
			}

			var content types.LogContent
			err = json.NewDecoder(resp.Body).Decode(&content)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()

			// 强制垃圾回收以测试内存泄漏
			if i%100 == 0 {
				runtime.GC()
			}
		}
	})
}

// BenchmarkResponseTime 测试响应时间分布
func BenchmarkResponseTime(b *testing.B) {
	testDir := setupBenchmarkEnvironment(b)
	defer os.RemoveAll(testDir)

	srv, port := startTestServer(b, testDir)
	defer srv.Stop()

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	var responseTimes []time.Duration
	var mu sync.Mutex

	b.Run("ResponseTimeDistribution", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			start := time.Now()

			resp, err := http.Get(baseURL + "/api/logs/large.log?limit=50")
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()

			duration := time.Since(start)

			mu.Lock()
			responseTimes = append(responseTimes, duration)
			mu.Unlock()
		}

		// 计算响应时间统计
		if len(responseTimes) > 0 {
			sort.Slice(responseTimes, func(i, j int) bool {
				return responseTimes[i] < responseTimes[j]
			})

			p50 := responseTimes[len(responseTimes)*50/100]
			p95 := responseTimes[len(responseTimes)*95/100]
			p99 := responseTimes[len(responseTimes)*99/100]

			b.Logf("Response time P50: %v, P95: %v, P99: %v", p50, p95, p99)
		}
	})
}

// setupBenchmarkEnvironment 设置基准测试环境
func setupBenchmarkEnvironment(b *testing.B) string {
	tempDir, err := os.MkdirTemp("", "benchmark_test_*")
	require.NoError(b, err)

	logsDir := filepath.Join(tempDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(b, err)

	// 创建小文件
	createSmallLogFile(b, filepath.Join(logsDir, "small.log"))

	// 创建大文件
	createLargeLogFile(b, filepath.Join(logsDir, "large.log"))

	// 创建 JSON 格式文件
	createJSONLogFile(b, filepath.Join(logsDir, "json.log"))

	return tempDir
}

// startTestServer 启动测试服务器
func startTestServer(b *testing.B, testDir string) (*server.HTTPServer, int) {
	// Use a fixed port for testing to avoid port conflicts
	port := 18080

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:        "localhost",
			Port:        port,
			LogPaths:    []string{filepath.Join(testDir, "logs")},
			MaxFileSize: 100 * 1024 * 1024,
			CacheSize:   50,
		},
		Logging: config.LogConfig{
			Level:      "info",
			Format:     "json",
			OutputPath: "stdout",
		},
		Security: config.SecurityConfig{
			EnableAuth: false,
		},
	}

	// Create mock dependencies
	logManager := &MockLogManager{}
	wsHub := &MockWebSocketHub{}

	srv := server.New(cfg, logManager, wsHub)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			b.Errorf("服务器启动失败: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(200 * time.Millisecond)

	b.Cleanup(func() {
		srv.Stop()
	})

	return srv, port
}

// createSmallLogFile 创建小型日志文件
func createSmallLogFile(b *testing.B, path string) {
	file, err := os.Create(path)
	require.NoError(b, err)
	defer file.Close()

	// 生成 100 行日志
	for i := 0; i < 100; i++ {
		timestamp := time.Now().Add(time.Duration(i) * time.Second)
		level := []string{"INFO", "WARN", "ERROR", "DEBUG"}[i%4]
		message := fmt.Sprintf("Log message %d", i)

		logLine := fmt.Sprintf("%s %s %s\n",
			timestamp.Format("2006-01-02 15:04:05"),
			level,
			message)

		_, err := file.WriteString(logLine)
		require.NoError(b, err)
	}
}

// createLargeLogFile 创建大型日志文件
func createLargeLogFile(b *testing.B, path string) {
	file, err := os.Create(path)
	require.NoError(b, err)
	defer file.Close()

	// 生成 50000 行日志
	logLevels := []string{"INFO", "WARN", "ERROR", "DEBUG"}
	messages := []string{
		"Application started successfully",
		"Processing user request",
		"Database query executed",
		"Cache miss for key",
		"External API call completed",
		"Background job finished",
		"Memory usage check",
		"Configuration reloaded",
		"User authentication successful",
		"File operation completed",
	}

	for i := 0; i < 50000; i++ {
		timestamp := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Second)
		level := logLevels[i%len(logLevels)]
		message := messages[i%len(messages)]

		logLine := fmt.Sprintf("%s %s %s - entry %d\n",
			timestamp.Format("2006-01-02 15:04:05"),
			level,
			message,
			i+1)

		_, err := file.WriteString(logLine)
		require.NoError(b, err)
	}

	b.Logf("创建大型日志文件: %s (50000 行)", path)
}

// createJSONLogFile 创建 JSON 格式日志文件
func createJSONLogFile(b *testing.B, path string) {
	file, err := os.Create(path)
	require.NoError(b, err)
	defer file.Close()

	// 生成 1000 行 JSON 日志
	for i := 0; i < 1000; i++ {
		timestamp := time.Now().Add(time.Duration(i) * time.Second)
		level := []string{"info", "warn", "error", "debug"}[i%4]

		logEntry := map[string]interface{}{
			"timestamp":  timestamp.Format(time.RFC3339),
			"level":      level,
			"message":    fmt.Sprintf("JSON log message %d", i),
			"service":    "logviewer",
			"request_id": fmt.Sprintf("req-%d", i),
			"user_id":    fmt.Sprintf("user-%d", i%100),
		}

		jsonData, err := json.Marshal(logEntry)
		require.NoError(b, err)

		_, err = file.WriteString(string(jsonData) + "\n")
		require.NoError(b, err)
	}
}
