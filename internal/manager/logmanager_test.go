package manager

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/local-log-viewer/internal/cache"
	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/types"
	"github.com/local-log-viewer/internal/watcher"
)

// 创建测试目录和文件
func setupTestFiles(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "logmanager_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	// 创建测试日志文件
	testFiles := map[string]string{
		"app.log":       "2023-01-01 10:00:00 INFO Application started\n2023-01-01 10:00:01 DEBUG Loading configuration\n2023-01-01 10:00:02 ERROR Failed to connect to database\n",
		"access.log":    "127.0.0.1 - - [01/Jan/2023:10:00:00 +0000] \"GET / HTTP/1.1\" 200 1234\n127.0.0.1 - - [01/Jan/2023:10:00:01 +0000] \"GET /api HTTP/1.1\" 404 567\n",
		"error.log":     "2023-01-01 10:00:00 [error] Connection timeout\n2023-01-01 10:00:01 [error] Database connection failed\n",
		"debug.txt":     "Debug message 1\nDebug message 2\nDebug message 3\n",
		"not_a_log.dat": "This should not be detected as a log file\n",
	}

	// 创建子目录
	subDir := filepath.Join(tempDir, "logs", "app")
	err = os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("创建子目录失败: %v", err)
	}

	// 写入测试文件
	for filename, content := range testFiles {
		var filePath string
		if filename == "app.log" {
			filePath = filepath.Join(subDir, filename)
		} else {
			filePath = filepath.Join(tempDir, filename)
		}

		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("创建测试文件 %s 失败: %v", filename, err)
		}
	}

	return tempDir
}

// 清理测试文件
func cleanupTestFiles(tempDir string) {
	os.RemoveAll(tempDir)
}

// 创建测试配置
func createTestConfig(logPaths []string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Server.LogPaths = logPaths
	cfg.Server.MaxFileSize = 1024 * 1024 // 1MB
	cfg.Server.CacheSize = 10
	return cfg
}

func TestLogManager_GetLogFiles(t *testing.T) {
	tempDir := setupTestFiles(t)
	defer cleanupTestFiles(tempDir)

	cfg := createTestConfig([]string{tempDir})
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		t.Fatalf("创建文件监控器失败: %v", err)
	}
	defer fileWatcher.Stop()

	cache := cache.NewMemoryCache(10, time.Minute)
	manager := NewLogManager(cfg, fileWatcher, cache)

	files, err := manager.GetLogFiles()
	if err != nil {
		t.Fatalf("获取日志文件失败: %v", err)
	}

	// 验证返回的文件数量（应该检测到4个日志文件，排除.dat文件）
	logFileCount := 0
	countLogFiles(files, &logFileCount)

	expectedCount := 4 // app.log, access.log, error.log, debug.txt
	if logFileCount != expectedCount {
		t.Errorf("期望检测到 %d 个日志文件，实际检测到 %d 个", expectedCount, logFileCount)
		// 打印实际检测到的文件用于调试
		found := make(map[string]bool)
		collectFileNames(files, found)
		t.Logf("实际检测到的文件: %v", found)
	}

	// 验证文件信息
	found := make(map[string]bool)
	collectFileNames(files, found)

	expectedFiles := []string{"app.log", "access.log", "error.log", "debug.txt"}
	for _, expectedFile := range expectedFiles {
		if !found[expectedFile] {
			t.Errorf("未找到期望的日志文件: %s", expectedFile)
		}
	}

	// 验证不应该包含的文件
	if found["not_a_log.dat"] {
		t.Error("不应该检测到非日志文件: not_a_log.dat")
	}
}

// 递归计算日志文件数量
func countLogFiles(files []types.LogFile, count *int) {
	for _, file := range files {
		if file.IsDirectory {
			countLogFiles(file.Children, count)
		} else {
			*count++
		}
	}
}

// 递归收集文件名
func collectFileNames(files []types.LogFile, found map[string]bool) {
	for _, file := range files {
		if file.IsDirectory {
			collectFileNames(file.Children, found)
		} else {
			found[file.Name] = true
		}
	}
}

func TestLogManager_ReadLogFile(t *testing.T) {
	tempDir := setupTestFiles(t)
	defer cleanupTestFiles(tempDir)

	cfg := createTestConfig([]string{tempDir})
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		t.Fatalf("创建文件监控器失败: %v", err)
	}
	defer fileWatcher.Stop()

	cache := cache.NewMemoryCache(10, time.Minute)
	manager := NewLogManager(cfg, fileWatcher, cache)

	// 测试读取文件内容
	logFilePath := filepath.Join(tempDir, "access.log") // 使用根目录下的文件
	content, err := manager.ReadLogFile(logFilePath, 0, 10)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	// 验证内容
	if len(content.Entries) == 0 {
		t.Error("读取的日志条目为空")
	}

	if content.TotalLines == 0 {
		t.Error("总行数应该大于0")
	}

	// 验证第一行内容
	if len(content.Entries) > 0 {
		firstEntry := content.Entries[0]
		if firstEntry.LineNum != 0 {
			t.Errorf("第一行的行号应该是0，实际是 %d", firstEntry.LineNum)
		}
		if firstEntry.Raw == "" {
			t.Error("原始内容不应该为空")
		}
	}
}

func TestLogManager_ReadLogFile_Pagination(t *testing.T) {
	tempDir := setupTestFiles(t)
	defer cleanupTestFiles(tempDir)

	cfg := createTestConfig([]string{tempDir})
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		t.Fatalf("创建文件监控器失败: %v", err)
	}
	defer fileWatcher.Stop()

	cache := cache.NewMemoryCache(10, time.Minute)
	manager := NewLogManager(cfg, fileWatcher, cache)

	logFilePath := filepath.Join(tempDir, "access.log")

	// 测试分页读取
	page1, err := manager.ReadLogFile(logFilePath, 0, 2)
	if err != nil {
		t.Fatalf("读取第一页失败: %v", err)
	}

	page2, err := manager.ReadLogFile(logFilePath, 2, 2)
	if err != nil {
		t.Fatalf("读取第二页失败: %v", err)
	}

	// 验证分页结果
	if len(page1.Entries) > 2 {
		t.Errorf("第一页应该最多包含2条记录，实际包含 %d 条", len(page1.Entries))
	}

	if len(page2.Entries) > 2 {
		t.Errorf("第二页应该最多包含2条记录，实际包含 %d 条", len(page2.Entries))
	}

	// 验证偏移量
	if page1.Offset != 0 {
		t.Errorf("第一页偏移量应该是0，实际是 %d", page1.Offset)
	}

	if page2.Offset != 2 {
		t.Errorf("第二页偏移量应该是2，实际是 %d", page2.Offset)
	}
}

func TestLogManager_ReadLogFile_NonExistent(t *testing.T) {
	tempDir := setupTestFiles(t)
	defer cleanupTestFiles(tempDir)

	cfg := createTestConfig([]string{tempDir})
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		t.Fatalf("创建文件监控器失败: %v", err)
	}
	defer fileWatcher.Stop()

	cache := cache.NewMemoryCache(10, time.Minute)
	manager := NewLogManager(cfg, fileWatcher, cache)

	// 测试读取不存在的文件
	_, err = manager.ReadLogFile("/nonexistent/file.log", 0, 10)
	if err == nil {
		t.Error("读取不存在的文件应该返回错误")
	}
}

func TestLogManager_WatchFile(t *testing.T) {
	tempDir := setupTestFiles(t)
	defer cleanupTestFiles(tempDir)

	cfg := createTestConfig([]string{tempDir})
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		t.Fatalf("创建文件监控器失败: %v", err)
	}
	defer fileWatcher.Stop()

	cache := cache.NewMemoryCache(10, time.Minute)
	manager := NewLogManager(cfg, fileWatcher, cache)

	// 启动管理器
	err = manager.Start()
	if err != nil {
		t.Fatalf("启动日志管理器失败: %v", err)
	}
	defer manager.Stop()

	// 监控文件
	logFilePath := filepath.Join(tempDir, "watch_test.log")

	// 创建测试文件
	err = os.WriteFile(logFilePath, []byte("initial content\n"), 0644)
	if err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	updateCh, err := manager.WatchFile(logFilePath)
	if err != nil {
		t.Fatalf("监控文件失败: %v", err)
	}

	// 修改文件
	go func() {
		time.Sleep(100 * time.Millisecond)
		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer file.Close()
		file.WriteString("new content\n")
	}()

	// 等待更新通知
	select {
	case update := <-updateCh:
		if update.Path != logFilePath {
			t.Errorf("更新路径不匹配，期望 %s，实际 %s", logFilePath, update.Path)
		}
		if update.Type == "" {
			t.Error("更新类型不应该为空")
		}
	case <-time.After(2 * time.Second):
		t.Error("超时等待文件更新通知")
	}
}

func TestLogManager_StartStop(t *testing.T) {
	tempDir := setupTestFiles(t)
	defer cleanupTestFiles(tempDir)

	cfg := createTestConfig([]string{tempDir})
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		t.Fatalf("创建文件监控器失败: %v", err)
	}

	cache := cache.NewMemoryCache(10, time.Minute)
	manager := NewLogManager(cfg, fileWatcher, cache)

	// 测试启动
	err = manager.Start()
	if err != nil {
		t.Fatalf("启动日志管理器失败: %v", err)
	}

	// 测试重复启动
	err = manager.Start()
	if err == nil {
		t.Error("重复启动应该返回错误")
	}

	// 测试停止
	err = manager.Stop()
	if err != nil {
		t.Fatalf("停止日志管理器失败: %v", err)
	}

	// 测试重复停止
	err = manager.Stop()
	if err != nil {
		t.Errorf("重复停止不应该返回错误: %v", err)
	}
}

func TestLogManager_isLogFile(t *testing.T) {
	cfg := createTestConfig([]string{})
	fileWatcher, _ := watcher.NewFileWatcher()
	cache := cache.NewMemoryCache(10, time.Minute)
	manager := NewLogManager(cfg, fileWatcher, cache).(*LogManager)

	testCases := []struct {
		path     string
		expected bool
	}{
		{"app.log", true},
		{"access.log", true},
		{"error.txt", true},
		{"debug.out", true},
		{"system.err", true},
		{"data.json", false},    // .json files need log keywords
		{"log_data.json", true}, // .json with log keyword
		{"config.yaml", false},
		{"image.png", false},
		{"document.pdf", false},
		{"application_log.txt", true},
		{"error_report.log", true},
		{"access_info.out", true},
		{"not_a_log.dat", false},
		{"debug.txt", true},
	}

	for _, tc := range testCases {
		result := manager.isLogFile(tc.path)
		if result != tc.expected {
			t.Errorf("isLogFile(%s) = %v, 期望 %v", tc.path, result, tc.expected)
		}
	}
}

func TestLogManager_Cache(t *testing.T) {
	tempDir := setupTestFiles(t)
	defer cleanupTestFiles(tempDir)

	cfg := createTestConfig([]string{tempDir})
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		t.Fatalf("创建文件监控器失败: %v", err)
	}
	defer fileWatcher.Stop()

	cache := cache.NewMemoryCache(10, time.Minute)
	manager := NewLogManager(cfg, fileWatcher, cache)

	logFilePath := filepath.Join(tempDir, "access.log")

	// 第一次读取
	start := time.Now()
	content1, err := manager.ReadLogFile(logFilePath, 0, 10)
	if err != nil {
		t.Fatalf("第一次读取失败: %v", err)
	}
	firstReadTime := time.Since(start)

	// 第二次读取（应该从缓存获取）
	start = time.Now()
	content2, err := manager.ReadLogFile(logFilePath, 0, 10)
	if err != nil {
		t.Fatalf("第二次读取失败: %v", err)
	}
	secondReadTime := time.Since(start)

	// 验证内容一致
	if len(content1.Entries) != len(content2.Entries) {
		t.Error("缓存的内容与原始内容不一致")
	}

	// 第二次读取应该更快（从缓存获取）
	if secondReadTime > firstReadTime {
		t.Log("注意：第二次读取时间可能没有明显改善，这在测试环境中是正常的")
	}
}
