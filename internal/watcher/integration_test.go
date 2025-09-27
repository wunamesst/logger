package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/local-log-viewer/internal/types"
)

// TestFileWatcher_Integration 集成测试，模拟真实使用场景
func TestFileWatcher_Integration(t *testing.T) {
	// 创建文件监控器
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create FileWatcher: %v", err)
	}
	defer fw.Stop()

	// 启动监控器
	err = fw.Start()
	if err != nil {
		t.Fatalf("Failed to start FileWatcher: %v", err)
	}

	// 创建临时目录模拟日志目录
	logDir, err := os.MkdirTemp("", "integration_test_logs_*")
	if err != nil {
		t.Fatalf("Failed to create temp log dir: %v", err)
	}
	defer os.RemoveAll(logDir)

	// 创建多个日志文件
	logFiles := []string{
		filepath.Join(logDir, "app.log"),
		filepath.Join(logDir, "error.log"),
		filepath.Join(logDir, "access.log"),
	}

	for _, logFile := range logFiles {
		file, err := os.Create(logFile)
		if err != nil {
			t.Fatalf("Failed to create log file %s: %v", logFile, err)
		}
		file.Close()
	}

	// 设置事件收集器
	var allEvents []types.FileEvent
	var eventsMutex sync.Mutex

	// 为每个日志文件设置监控
	for _, logFile := range logFiles {
		err = fw.WatchFile(logFile, func(event types.FileEvent) {
			eventsMutex.Lock()
			allEvents = append(allEvents, event)
			eventsMutex.Unlock()
		})
		if err != nil {
			t.Fatalf("Failed to watch file %s: %v", logFile, err)
		}
	}

	// 等待监控设置完成
	time.Sleep(100 * time.Millisecond)

	// 模拟应用程序写入日志
	testScenarios := []struct {
		file    string
		content string
	}{
		{logFiles[0], "INFO: Application started\n"},
		{logFiles[1], "ERROR: Database connection failed\n"},
		{logFiles[2], "GET /api/users 200\n"},
		{logFiles[0], "INFO: Processing request\n"},
		{logFiles[1], "ERROR: Timeout occurred\n"},
	}

	for _, scenario := range testScenarios {
		file, err := os.OpenFile(scenario.file, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			t.Fatalf("Failed to open file %s: %v", scenario.file, err)
		}

		_, err = file.WriteString(scenario.content)
		if err != nil {
			t.Fatalf("Failed to write to file %s: %v", scenario.file, err)
		}
		file.Close()

		// 短暂等待确保事件被处理
		time.Sleep(50 * time.Millisecond)
	}

	// 等待所有事件处理完成
	time.Sleep(200 * time.Millisecond)

	// 验证事件
	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	if len(allEvents) == 0 {
		t.Fatal("Expected events to be generated, got none")
	}

	// 验证每个文件都有事件
	fileEventCount := make(map[string]int)
	for _, event := range allEvents {
		fileEventCount[event.Path]++

		// 验证事件类型
		if event.Type != "modify" {
			t.Errorf("Expected modify event, got %s for file %s", event.Type, event.Path)
		}
	}

	// 验证所有文件都有事件
	for _, logFile := range logFiles {
		if count, exists := fileEventCount[logFile]; !exists || count == 0 {
			t.Errorf("Expected events for file %s, got %d", logFile, count)
		}
	}

	t.Logf("Successfully processed %d events across %d files", len(allEvents), len(logFiles))
}

// TestFileWatcher_LogRotation 测试日志轮转场景
func TestFileWatcher_LogRotation(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create FileWatcher: %v", err)
	}
	defer fw.Stop()

	err = fw.Start()
	if err != nil {
		t.Fatalf("Failed to start FileWatcher: %v", err)
	}

	// 创建临时目录
	logDir, err := os.MkdirTemp("", "rotation_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(logDir)

	logFile := filepath.Join(logDir, "app.log")

	// 创建初始日志文件
	file, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	file.Close()

	// 设置事件收集
	var events []types.FileEvent
	var eventsMutex sync.Mutex

	callback := func(event types.FileEvent) {
		eventsMutex.Lock()
		events = append(events, event)
		eventsMutex.Unlock()
	}

	// 监控文件
	err = fw.WatchFile(logFile, callback)
	if err != nil {
		t.Fatalf("Failed to watch file: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 模拟日志轮转：删除原文件，创建新文件
	err = os.Remove(logFile)
	if err != nil {
		t.Fatalf("Failed to remove log file: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 创建新的日志文件
	newFile, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Failed to create new log file: %v", err)
	}
	_, err = newFile.WriteString("New log after rotation\n")
	if err != nil {
		t.Fatalf("Failed to write to new log file: %v", err)
	}
	newFile.Close()

	time.Sleep(200 * time.Millisecond)

	// 验证事件
	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	if len(events) == 0 {
		t.Fatal("Expected events for log rotation, got none")
	}

	// 应该至少有一个删除事件
	hasDelete := false
	for _, event := range events {
		if event.Type == "delete" {
			hasDelete = true
			break
		}
	}

	if !hasDelete {
		t.Error("Expected delete event during log rotation")
	}

	t.Logf("Log rotation test completed with %d events", len(events))
}

// TestFileWatcher_ConcurrentAccess 测试并发访问
func TestFileWatcher_ConcurrentAccess(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create FileWatcher: %v", err)
	}
	defer fw.Stop()

	err = fw.Start()
	if err != nil {
		t.Fatalf("Failed to start FileWatcher: %v", err)
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "concurrent_test_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 并发添加和移除监控
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			callback := func(event types.FileEvent) {
				// 简单的回调函数
			}

			// 添加监控
			err := fw.WatchFile(tmpFile.Name(), callback)
			if err != nil {
				t.Errorf("Goroutine %d failed to watch file: %v", id, err)
				return
			}

			// 短暂等待
			time.Sleep(10 * time.Millisecond)

			// 移除监控
			err = fw.UnwatchFile(tmpFile.Name())
			if err != nil {
				t.Errorf("Goroutine %d failed to unwatch file: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	t.Log("Concurrent access test completed successfully")
}
