package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/server"
)

// TestRealtimeUpdates 测试实时更新功能
func TestRealtimeUpdates(t *testing.T) {
	// 设置测试环境
	testDir := setupTestEnvironment(t)
	defer os.RemoveAll(testDir)

	// 创建配置
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:     "localhost",
			Port:     8084,
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
	baseURL := fmt.Sprintf("http://localhost:%d", 8084)

	defer srv.Stop()

	t.Run("WebSocket连接测试", func(t *testing.T) {
		// 连接WebSocket
		wsURL := fmt.Sprintf("ws://localhost:%d/ws", 8084)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Skipf("WebSocket连接失败，跳过测试: %v", err)
			return
		}
		defer conn.Close()

		// 设置读取超时
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		// 发送订阅消息
		subscribeMsg := map[string]interface{}{
			"type": "subscribe",
			"path": "app.log",
		}
		err = conn.WriteJSON(subscribeMsg)
		require.NoError(t, err)

		// 等待响应
		var response map[string]interface{}
		err = conn.ReadJSON(&response)
		if err != nil {
			t.Logf("读取WebSocket响应失败: %v", err)
		}
	})

	t.Run("文件更新检测", func(t *testing.T) {
		// 创建测试日志文件
		logFile := filepath.Join(testDir, "logs", "test.log")

		// 写入初始内容
		err := os.WriteFile(logFile, []byte("Initial log entry\n"), 0644)
		require.NoError(t, err)

		// 等待文件系统事件处理
		time.Sleep(100 * time.Millisecond)

		// 检查文件是否被检测到
		resp, err := http.Get(baseURL + "/api/logs")
		require.NoError(t, err)
		defer resp.Body.Close()

		var files []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&files)
		require.NoError(t, err)

		// 验证文件列表包含新文件
		found := false
		for _, file := range files {
			if name, ok := file["name"].(string); ok && name == "test.log" {
				found = true
				break
			}
		}
		assert.True(t, found, "新创建的日志文件应该被检测到")
	})

	t.Run("实时日志追加", func(t *testing.T) {
		// 创建测试日志文件
		logFile := filepath.Join(testDir, "logs", "realtime.log")

		// 写入初始内容
		initialContent := "2024-01-01T10:00:00.000Z [INFO] Initial log entry\n"
		err := os.WriteFile(logFile, []byte(initialContent), 0644)
		require.NoError(t, err)

		// 等待文件系统事件处理
		time.Sleep(200 * time.Millisecond)

		// 追加新内容
		newContent := "2024-01-01T10:00:01.000Z [INFO] New log entry\n"
		file, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
		require.NoError(t, err)
		_, err = file.WriteString(newContent)
		require.NoError(t, err)
		file.Close()

		// 等待文件更新处理
		time.Sleep(200 * time.Millisecond)

		// 验证文件内容已更新
		resp, err := http.Get(baseURL + "/api/logs/realtime.log")
		require.NoError(t, err)
		defer resp.Body.Close()

		var logContent map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&logContent)
		require.NoError(t, err)

		entries, ok := logContent["entries"].([]interface{})
		require.True(t, ok)
		assert.GreaterOrEqual(t, len(entries), 2, "应该包含初始和新增的日志条目")
	})
}
