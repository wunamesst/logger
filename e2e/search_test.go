package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/server"
)

// TestSearchFunctionality 测试搜索功能
func TestSearchFunctionality(t *testing.T) {
	// 设置测试环境
	testDir := setupSearchTestEnvironment(t)
	defer os.RemoveAll(testDir)

	// 创建配置
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:     "localhost",
			Port:     8085,
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
	baseURL := fmt.Sprintf("http://localhost:%d", 8085)

	defer srv.Stop()

	t.Run("关键词搜索", func(t *testing.T) {
		// 搜索ERROR关键词
		searchURL := fmt.Sprintf("%s/api/search?path=search_test.log&query=%s",
			baseURL, url.QueryEscape("ERROR"))

		resp, err := http.Get(searchURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var searchResult map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResult)
		require.NoError(t, err)

		matches, ok := searchResult["matches"].([]interface{})
		require.True(t, ok)
		assert.Greater(t, len(matches), 0, "应该找到包含ERROR的日志条目")

		// 验证搜索结果包含关键词
		for _, match := range matches {
			if matchMap, ok := match.(map[string]interface{}); ok {
				if message, ok := matchMap["message"].(string); ok {
					assert.Contains(t, message, "ERROR")
				}
			}
		}
	})

	t.Run("正则表达式搜索", func(t *testing.T) {
		// 使用正则表达式搜索时间戳
		pattern := `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`
		searchURL := fmt.Sprintf("%s/api/search?path=search_test.log&query=%s&isRegex=true",
			baseURL, url.QueryEscape(pattern))

		resp, err := http.Get(searchURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var searchResult map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResult)
		require.NoError(t, err)

		matches, ok := searchResult["matches"].([]interface{})
		require.True(t, ok)
		assert.Greater(t, len(matches), 0, "应该找到匹配时间戳格式的日志条目")
	})

	t.Run("日志级别过滤", func(t *testing.T) {
		// 只搜索ERROR级别的日志
		searchURL := fmt.Sprintf("%s/api/search?path=search_test.log&levels=ERROR", baseURL)

		resp, err := http.Get(searchURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var searchResult map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResult)
		require.NoError(t, err)

		matches, ok := searchResult["matches"].([]interface{})
		require.True(t, ok)

		// 验证所有结果都是ERROR级别
		for _, match := range matches {
			if matchMap, ok := match.(map[string]interface{}); ok {
				if level, ok := matchMap["level"].(string); ok {
					assert.Equal(t, "ERROR", level)
				}
			}
		}
	})

	t.Run("时间范围过滤", func(t *testing.T) {
		// 搜索特定时间范围的日志
		startTime := "2024-01-01T10:00:00Z"
		endTime := "2024-01-01T11:00:00Z"

		searchURL := fmt.Sprintf("%s/api/search?path=search_test.log&startTime=%s&endTime=%s",
			baseURL, url.QueryEscape(startTime), url.QueryEscape(endTime))

		resp, err := http.Get(searchURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var searchResult map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResult)
		require.NoError(t, err)

		matches, ok := searchResult["matches"].([]interface{})
		require.True(t, ok)

		// 验证结果在指定时间范围内
		for _, match := range matches {
			if matchMap, ok := match.(map[string]interface{}); ok {
				if timestamp, ok := matchMap["timestamp"].(string); ok {
					assert.True(t, timestamp >= startTime && timestamp <= endTime,
						"时间戳应该在指定范围内: %s", timestamp)
				}
			}
		}
	})

	t.Run("分页搜索", func(t *testing.T) {
		// 第一页
		searchURL := fmt.Sprintf("%s/api/search?path=search_test.log&query=log&limit=5&offset=0", baseURL)

		resp, err := http.Get(searchURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var searchResult map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&searchResult)
		require.NoError(t, err)

		matches, ok := searchResult["matches"].([]interface{})
		require.True(t, ok)
		assert.LessOrEqual(t, len(matches), 5, "第一页应该最多返回5个结果")

		// 检查是否有更多结果
		if hasMore, ok := searchResult["hasMore"].(bool); ok && hasMore {
			// 第二页
			searchURL2 := fmt.Sprintf("%s/api/search?path=search_test.log&query=log&limit=5&offset=5", baseURL)

			resp2, err := http.Get(searchURL2)
			require.NoError(t, err)
			defer resp2.Body.Close()

			assert.Equal(t, http.StatusOK, resp2.StatusCode)
		}
	})

	t.Run("无效搜索参数", func(t *testing.T) {
		// 测试无效的正则表达式
		invalidPattern := `[invalid`
		searchURL := fmt.Sprintf("%s/api/search?path=search_test.log&query=%s&isRegex=true",
			baseURL, url.QueryEscape(invalidPattern))

		resp, err := http.Get(searchURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		// 应该返回错误状态码或空结果
		assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusOK)
	})
}

// setupSearchTestEnvironment 设置搜索测试环境
func setupSearchTestEnvironment(t *testing.T) string {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "e2e_search_test_*")
	require.NoError(t, err)

	// 创建日志目录
	logsDir := filepath.Join(tempDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	// 创建搜索测试日志文件
	searchLogPath := filepath.Join(logsDir, "search_test.log")
	file, err := os.Create(searchLogPath)
	require.NoError(t, err)
	defer file.Close()

	// 生成测试日志数据
	logEntries := []string{
		"2024-01-01T10:00:00.000Z [INFO] Application started successfully",
		"2024-01-01T10:05:00.000Z [INFO] Processing user request",
		"2024-01-01T10:10:00.000Z [WARN] High memory usage detected",
		"2024-01-01T10:15:00.000Z [ERROR] Database connection failed",
		"2024-01-01T10:20:00.000Z [INFO] Retrying database connection",
		"2024-01-01T10:25:00.000Z [ERROR] Authentication failed for user",
		"2024-01-01T10:30:00.000Z [INFO] User logged in successfully",
		"2024-01-01T11:00:00.000Z [INFO] Daily backup completed",
		"2024-01-01T11:05:00.000Z [WARN] Disk space running low",
		"2024-01-01T11:10:00.000Z [ERROR] Failed to send notification",
	}

	for _, entry := range logEntries {
		_, err := file.WriteString(entry + "\n")
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
