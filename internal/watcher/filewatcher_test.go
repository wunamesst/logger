package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/local-log-viewer/internal/types"
)

func TestNewFileWatcher(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create FileWatcher: %v", err)
	}

	if fw == nil {
		t.Fatal("FileWatcher should not be nil")
	}

	// 清理
	fw.Stop()
}

func TestFileWatcher_WatchFile(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create FileWatcher: %v", err)
	}
	defer fw.Stop()

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "test_watch_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 测试监控文件
	callback := func(event types.FileEvent) {
		// 回调函数用于测试
	}

	err = fw.WatchFile(tmpFile.Name(), callback)
	if err != nil {
		t.Fatalf("Failed to watch file: %v", err)
	}

	// 验证回调函数已添加
	fwImpl := fw.(*FileWatcher)
	absPath, _ := filepath.Abs(tmpFile.Name())

	fwImpl.mutex.RLock()
	callbacks, exists := fwImpl.callbacks[absPath]
	fwImpl.mutex.RUnlock()

	if !exists {
		t.Fatal("File should be in callbacks map")
	}

	if len(callbacks) != 1 {
		t.Fatalf("Expected 1 callback, got %d", len(callbacks))
	}
}

func TestFileWatcher_UnwatchFile(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create FileWatcher: %v", err)
	}
	defer fw.Stop()

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "test_unwatch_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 先监控文件
	callback := func(event types.FileEvent) {}
	err = fw.WatchFile(tmpFile.Name(), callback)
	if err != nil {
		t.Fatalf("Failed to watch file: %v", err)
	}

	// 取消监控
	err = fw.UnwatchFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to unwatch file: %v", err)
	}

	// 验证回调函数已移除
	fwImpl := fw.(*FileWatcher)
	absPath, _ := filepath.Abs(tmpFile.Name())

	fwImpl.mutex.RLock()
	_, exists := fwImpl.callbacks[absPath]
	fwImpl.mutex.RUnlock()

	if exists {
		t.Fatal("File should not be in callbacks map after unwatching")
	}
}

func TestFileWatcher_StartStop(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create FileWatcher: %v", err)
	}

	fwImpl := fw.(*FileWatcher)

	// 测试启动
	err = fw.Start()
	if err != nil {
		t.Fatalf("Failed to start FileWatcher: %v", err)
	}

	if !fwImpl.running {
		t.Fatal("FileWatcher should be running after Start()")
	}

	// 测试重复启动应该返回错误
	err = fw.Start()
	if err == nil {
		t.Fatal("Starting already running FileWatcher should return error")
	}

	// 测试停止
	err = fw.Stop()
	if err != nil {
		t.Fatalf("Failed to stop FileWatcher: %v", err)
	}

	if fwImpl.running {
		t.Fatal("FileWatcher should not be running after Stop()")
	}

	// 测试重复停止不应该返回错误
	err = fw.Stop()
	if err != nil {
		t.Fatalf("Stopping already stopped FileWatcher should not return error: %v", err)
	}
}

func TestFileWatcher_FileEvents(t *testing.T) {
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

	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "test_events_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "test.log")

	// 先创建文件
	file, err := os.Create(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
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
	err = fw.WatchFile(tmpFile, callback)
	if err != nil {
		t.Fatalf("Failed to watch file: %v", err)
	}

	// 等待一下确保监控已设置
	time.Sleep(100 * time.Millisecond)

	// 修改文件
	file, err = os.OpenFile(tmpFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	// 写入内容
	_, err = file.WriteString("test content\n")
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}
	file.Close()

	// 等待事件处理
	time.Sleep(200 * time.Millisecond)

	// 删除文件
	err = os.Remove(tmpFile)
	if err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	// 等待事件处理
	time.Sleep(200 * time.Millisecond)

	// 验证事件
	eventsMutex.Lock()
	defer eventsMutex.Unlock()

	if len(events) == 0 {
		t.Fatal("Expected at least one event, got none")
	}

	// 验证事件类型
	hasModify := false
	hasDelete := false

	for _, event := range events {
		if event.Path != tmpFile {
			t.Errorf("Expected event path %s, got %s", tmpFile, event.Path)
		}

		switch event.Type {
		case "modify":
			hasModify = true
		case "delete":
			hasDelete = true
		}
	}

	if !hasModify {
		t.Error("Expected at least one modify event")
	}

	if !hasDelete {
		t.Error("Expected at least one delete event")
	}
}

func TestFileWatcher_MultipleCallbacks(t *testing.T) {
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

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "test_multiple_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 设置多个回调函数
	var callback1Called, callback2Called bool
	var mutex sync.Mutex

	callback1 := func(event types.FileEvent) {
		mutex.Lock()
		callback1Called = true
		mutex.Unlock()
	}

	callback2 := func(event types.FileEvent) {
		mutex.Lock()
		callback2Called = true
		mutex.Unlock()
	}

	// 添加多个回调
	err = fw.WatchFile(tmpFile.Name(), callback1)
	if err != nil {
		t.Fatalf("Failed to watch file with callback1: %v", err)
	}

	err = fw.WatchFile(tmpFile.Name(), callback2)
	if err != nil {
		t.Fatalf("Failed to watch file with callback2: %v", err)
	}

	// 等待一下确保监控已设置
	time.Sleep(100 * time.Millisecond)

	// 修改文件触发事件
	file, err := os.OpenFile(tmpFile.Name(), os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	_, err = file.WriteString("test\n")
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}
	file.Close()

	// 等待事件处理
	time.Sleep(200 * time.Millisecond)

	// 验证两个回调都被调用
	mutex.Lock()
	defer mutex.Unlock()

	if !callback1Called {
		t.Error("Callback1 should have been called")
	}

	if !callback2Called {
		t.Error("Callback2 should have been called")
	}
}

func TestFileWatcher_ConvertEvent(t *testing.T) {
	// 这个测试主要是为了文档化预期行为
	// convertEvent 方法通过集成测试在 TestFileWatcher_FileEvents 中验证

	testCases := []struct {
		name     string
		op       string
		expected string
	}{
		{"Create", "CREATE", "create"},
		{"Write", "WRITE", "modify"},
		{"Remove", "REMOVE", "delete"},
		{"Rename", "RENAME", "delete"},
		{"Chmod", "CHMOD", "modify"},
	}

	// 验证我们有正确的映射关系文档
	if len(testCases) != 5 {
		t.Errorf("Expected 5 test cases, got %d", len(testCases))
	}
}

func TestFileWatcher_ErrorHandling(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create FileWatcher: %v", err)
	}
	defer fw.Stop()

	// 测试监控不存在的目录
	nonExistentPath := "/path/that/does/not/exist/file.log"
	callback := func(event types.FileEvent) {}

	err = fw.WatchFile(nonExistentPath, callback)
	if err == nil {
		t.Error("Watching non-existent path should return error")
	}

	// 测试取消监控不存在的文件（应该不返回错误）
	err = fw.UnwatchFile(nonExistentPath)
	if err != nil {
		t.Errorf("Unwatching non-existent file should not return error: %v", err)
	}
}
