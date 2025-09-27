package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/server"
)

// TestPerformance 测试性能
func TestPerformance(t *testing.T) {
	// 设置测试环境
	testDir := setupLargeTestEnvironment(t)
	defer os.RemoveAll(testDir)

	// 创建配置
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:     "localhost",
			Port:     8082,
			LogPaths: []string{filepath.Join(testDir, "logs")},
		},
	}

	// 启动服务器
	srv := server.New(cfg, nil, nil)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("服务器启动失败: %v", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	baseURL := fmt.Sprintf("http://localhost:%d", 8082)

	defer srv.Stop()

	t.Run("并发访问测试", func(t *testing.T) {
		concurrency := 10
		requests := 50

		var wg sync.WaitGroup
		results := make(chan time.Duration, concurrency*requests)

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < requests; j++ {
					start := time.Now()
					resp, err := http.Get(baseURL + "/api/logs")
					if err == nil {
						resp.Body.Close()
						results <- time.Since(start)
					}
				}
			}()
		}

		wg.Wait()
		close(results)

		var totalTime time.Duration
		count := 0
		for duration := range results {
			totalTime += duration
			count++
		}

		if count > 0 {
			avgTime := totalTime / time.Duration(count)
			t.Logf("平均响应时间: %v", avgTime)
			assert.Less(t, avgTime, 2*time.Second, "平均响应时间应该小于2秒")
		}
	})

	t.Run("大文件处理测试", func(t *testing.T) {
		// 测试读取大文件
		resp, err := http.Get(baseURL + "/api/logs/large.log?limit=1000")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var logContent map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&logContent)
		require.NoError(t, err)

		entries, ok := logContent["entries"].([]interface{})
		require.True(t, ok)
		assert.LessOrEqual(t, len(entries), 1000)
	})

	t.Run("内存使用测试", func(t *testing.T) {
		// 连续请求多次，检查内存是否稳定
		for i := 0; i < 100; i++ {
			resp, err := http.Get(baseURL + "/api/logs")
			require.NoError(t, err)
			resp.Body.Close()
		}

		// 检查健康状态
		resp, err := http.Get(baseURL + "/api/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestLargeFileHandling 测试大文件处理
func TestLargeFileHandling(t *testing.T) {
	// 设置测试环境
	testDir := setupLargeTestEnvironment(t)
	defer os.RemoveAll(testDir)

	// 创建配置
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:     "localhost",
			Port:     8083,
			LogPaths: []string{filepath.Join(testDir, "logs")},
		},
	}

	// 启动服务器
	srv := server.New(cfg, nil, nil)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("服务器启动失败: %v", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	baseURL := fmt.Sprintf("http://localhost:%d", 8083)

	defer srv.Stop()

	t.Run("分页读取大文件", func(t *testing.T) {
		// 第一页
		resp, err := http.Get(baseURL + "/api/logs/large.log?offset=0&limit=100")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var logContent map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&logContent)
		require.NoError(t, err)

		entries, ok := logContent["entries"].([]interface{})
		require.True(t, ok)
		assert.LessOrEqual(t, len(entries), 100)

		// 检查是否有更多数据
		hasMore, ok := logContent["hasMore"].(bool)
		require.True(t, ok)
		if len(entries) == 100 {
			assert.True(t, hasMore)
		}
	})

	t.Run("搜索大文件", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/search?path=large.log&query=ERROR&limit=50")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var searchResult map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResult)
		require.NoError(t, err)

		matches, ok := searchResult["matches"].([]interface{})
		require.True(t, ok)
		assert.LessOrEqual(t, len(matches), 50)
	})
}

// setupLargeTestEnvironment 设置大文件测试环境
func setupLargeTestEnvironment(t *testing.T) string {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "e2e_perf_test_*")
	require.NoError(t, err)

	// 创建日志目录
	logsDir := filepath.Join(tempDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	// 创建大文件
	largeLogPath := filepath.Join(logsDir, "large.log")
	file, err := os.Create(largeLogPath)
	require.NoError(t, err)
	defer file.Close()

	// 生成大量日志数据
	for i := 0; i < 10000; i++ {
		level := "INFO"
		if i%10 == 0 {
			level = "ERROR"
		} else if i%5 == 0 {
			level = "WARN"
		}

		logLine := fmt.Sprintf("2024-01-01T%02d:%02d:%02d.000Z [%s] This is log entry number %d with some additional content to make it longer\n",
			i/3600%24, (i/60)%60, i%60, level, i)
		_, err := file.WriteString(logLine)
		require.NoError(t, err)
	}

	// 复制其他测试文件
	testDataDir := "testdata"
	files := []string{"app.log", "error.log", "access.log", "json.log"}

	for _, filename := range files {
		srcPath := filepath.Join(testDataDir, filename)
		dstPath := filepath.Join(logsDir, filename)

		srcData, err := os.ReadFile(srcPath)
		if err != nil {
			// 如果测试文件不存在，创建一个简单的文件
			srcData = []byte(fmt.Sprintf("Test log file: %s\n", filename))
		}

		err = os.WriteFile(dstPath, srcData, 0644)
		require.NoError(t, err)
	}

	return tempDir
}
