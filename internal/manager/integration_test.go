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

func TestLogManager_Integration(t *testing.T) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "logmanager_integration_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建测试日志文件
	logDir := filepath.Join(tempDir, "logs")
	err = os.MkdirAll(logDir, 0755)
	if err != nil {
		t.Fatalf("创建日志目录失败: %v", err)
	}

	// 创建应用日志文件
	appLogPath := filepath.Join(logDir, "application.log")
	initialContent := `2023-01-01 10:00:00 INFO Application started
2023-01-01 10:00:01 DEBUG Loading configuration
2023-01-01 10:00:02 ERROR Failed to connect to database
2023-01-01 10:00:03 WARN Retrying connection
2023-01-01 10:00:04 INFO Connection established
`
	err = os.WriteFile(appLogPath, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("创建应用日志文件失败: %v", err)
	}

	// 创建访问日志文件
	accessLogPath := filepath.Join(logDir, "access.log")
	accessContent := `127.0.0.1 - - [01/Jan/2023:10:00:00 +0000] "GET / HTTP/1.1" 200 1234
127.0.0.1 - - [01/Jan/2023:10:00:01 +0000] "GET /api HTTP/1.1" 404 567
127.0.0.1 - - [01/Jan/2023:10:00:02 +0000] "POST /login HTTP/1.1" 200 89
`
	err = os.WriteFile(accessLogPath, []byte(accessContent), 0644)
	if err != nil {
		t.Fatalf("创建访问日志文件失败: %v", err)
	}

	// 创建配置
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:        "localhost",
			Port:        8080,
			LogPaths:    []string{logDir},
			MaxFileSize: 10 * 1024 * 1024, // 10MB
			CacheSize:   20,
		},
	}

	// 创建文件监控器
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		t.Fatalf("创建文件监控器失败: %v", err)
	}
	defer fileWatcher.Stop()

	// 创建缓存
	cache := cache.NewMemoryCache(20, 5*time.Minute)

	// 创建日志管理器
	manager := NewLogManager(cfg, fileWatcher, cache)

	// 启动管理器
	err = manager.Start()
	if err != nil {
		t.Fatalf("启动日志管理器失败: %v", err)
	}
	defer manager.Stop()

	// 测试获取日志文件列表
	t.Run("GetLogFiles", func(t *testing.T) {
		files, err := manager.GetLogFiles()
		if err != nil {
			t.Fatalf("获取日志文件失败: %v", err)
		}

		if len(files) == 0 {
			t.Fatal("应该检测到日志文件")
		}

		// 验证检测到的文件
		foundFiles := make(map[string]bool)
		collectFileNamesIntegration(files, foundFiles)

		expectedFiles := []string{"application.log", "access.log"}
		for _, expectedFile := range expectedFiles {
			if !foundFiles[expectedFile] {
				t.Errorf("未找到期望的日志文件: %s", expectedFile)
			}
		}
	})

	// 测试读取日志文件内容
	t.Run("ReadLogFile", func(t *testing.T) {
		content, err := manager.ReadLogFile(appLogPath, 0, 3)
		if err != nil {
			t.Fatalf("读取日志文件失败: %v", err)
		}

		if len(content.Entries) != 3 {
			t.Errorf("期望读取3条记录，实际读取 %d 条", len(content.Entries))
		}

		if content.TotalLines != 5 {
			t.Errorf("期望总行数为5，实际为 %d", content.TotalLines)
		}

		if !content.HasMore {
			t.Error("应该还有更多内容")
		}

		// 验证第一条记录
		if len(content.Entries) > 0 {
			firstEntry := content.Entries[0]
			if firstEntry.LineNum != 0 {
				t.Errorf("第一条记录的行号应该是0，实际是 %d", firstEntry.LineNum)
			}
			if firstEntry.Raw == "" {
				t.Error("原始内容不应该为空")
			}
		}
	})

	// 测试分页读取
	t.Run("ReadLogFile_Pagination", func(t *testing.T) {
		// 读取第一页
		page1, err := manager.ReadLogFile(appLogPath, 0, 2)
		if err != nil {
			t.Fatalf("读取第一页失败: %v", err)
		}

		// 读取第二页
		page2, err := manager.ReadLogFile(appLogPath, 2, 2)
		if err != nil {
			t.Fatalf("读取第二页失败: %v", err)
		}

		// 验证分页结果
		if len(page1.Entries) != 2 {
			t.Errorf("第一页应该包含2条记录，实际包含 %d 条", len(page1.Entries))
		}

		// 第二页应该包含剩余的记录（总共5行，第一页2行，第二页应该有2行）
		expectedPage2Count := 2
		if len(page2.Entries) != expectedPage2Count {
			t.Errorf("第二页应该包含%d条记录，实际包含 %d 条", expectedPage2Count, len(page2.Entries))
		}

		// 验证记录不重复
		if len(page1.Entries) > 0 && len(page2.Entries) > 0 {
			if page1.Entries[0].Raw == page2.Entries[0].Raw {
				t.Error("分页结果不应该重复")
			}
		}
	})

	// 测试文件监控
	t.Run("WatchFile", func(t *testing.T) {
		// 监控应用日志文件
		updateCh, err := manager.WatchFile(appLogPath)
		if err != nil {
			t.Fatalf("监控文件失败: %v", err)
		}

		// 在另一个协程中修改文件
		go func() {
			time.Sleep(100 * time.Millisecond)
			file, err := os.OpenFile(appLogPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				t.Logf("打开文件失败: %v", err)
				return
			}
			defer file.Close()

			_, err = file.WriteString("2023-01-01 10:00:05 INFO New log entry\n")
			if err != nil {
				t.Logf("写入文件失败: %v", err)
			}
		}()

		// 等待更新通知
		select {
		case update := <-updateCh:
			if update.Path != appLogPath {
				t.Errorf("更新路径不匹配，期望 %s，实际 %s", appLogPath, update.Path)
			}
			if update.Type == "" {
				t.Error("更新类型不应该为空")
			}
		case <-time.After(2 * time.Second):
			t.Error("超时等待文件更新通知")
		}
	})

	// 测试缓存功能
	t.Run("Cache", func(t *testing.T) {
		// 第一次读取
		start := time.Now()
		content1, err := manager.ReadLogFile(accessLogPath, 0, 5)
		if err != nil {
			t.Fatalf("第一次读取失败: %v", err)
		}
		firstReadTime := time.Since(start)

		// 第二次读取（应该从缓存获取）
		start = time.Now()
		content2, err := manager.ReadLogFile(accessLogPath, 0, 5)
		if err != nil {
			t.Fatalf("第二次读取失败: %v", err)
		}
		secondReadTime := time.Since(start)

		// 验证内容一致
		if len(content1.Entries) != len(content2.Entries) {
			t.Error("缓存的内容与原始内容不一致")
		}

		// 验证缓存效果（第二次读取应该更快）
		if secondReadTime > firstReadTime {
			t.Log("注意：第二次读取时间可能没有明显改善，这在测试环境中是正常的")
		}

		// 验证具体内容
		if len(content1.Entries) > 0 && len(content2.Entries) > 0 {
			if content1.Entries[0].Raw != content2.Entries[0].Raw {
				t.Error("缓存内容与原始内容不匹配")
			}
		}
	})

	// 测试错误处理
	t.Run("ErrorHandling", func(t *testing.T) {
		// 测试读取不存在的文件
		_, err := manager.ReadLogFile("/nonexistent/file.log", 0, 10)
		if err == nil {
			t.Error("读取不存在的文件应该返回错误")
		}

		// 测试监控不存在的文件
		_, err = manager.WatchFile("/nonexistent/file.log")
		if err == nil {
			t.Error("监控不存在的文件应该返回错误")
		}
	})
}

// 辅助函数：递归收集文件名（集成测试版本）
func collectFileNamesIntegration(files []types.LogFile, found map[string]bool) {
	for _, file := range files {
		if file.IsDirectory {
			collectFileNamesIntegration(file.Children, found)
		} else {
			found[file.Name] = true
		}
	}
}
