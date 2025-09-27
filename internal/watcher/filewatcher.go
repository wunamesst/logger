package watcher

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/local-log-viewer/internal/interfaces"
	"github.com/local-log-viewer/internal/types"
)

// FileWatcher 文件监控器实现
type FileWatcher struct {
	watcher   *fsnotify.Watcher
	callbacks map[string][]func(types.FileEvent)
	mutex     sync.RWMutex
	running   bool
	stopCh    chan struct{}
}

// NewFileWatcher 创建新的文件监控器
func NewFileWatcher() (interfaces.FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	return &FileWatcher{
		watcher:   watcher,
		callbacks: make(map[string][]func(types.FileEvent)),
		stopCh:    make(chan struct{}),
	}, nil
}

// WatchFile 监控文件
func (fw *FileWatcher) WatchFile(path string, callback func(types.FileEvent)) error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	// 规范化路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	// 添加回调函数
	fw.callbacks[absPath] = append(fw.callbacks[absPath], callback)

	// 如果是第一次监控这个文件，添加到 fsnotify watcher
	if len(fw.callbacks[absPath]) == 1 {
		if err := fw.watcher.Add(absPath); err != nil {
			// 如果添加失败，移除回调
			delete(fw.callbacks, absPath)
			return fmt.Errorf("failed to watch file %s: %w", absPath, err)
		}
	}

	return nil
}

// UnwatchFile 取消监控文件
func (fw *FileWatcher) UnwatchFile(path string) error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	// 规范化路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	// 移除所有回调函数
	delete(fw.callbacks, absPath)

	// 从 fsnotify watcher 中移除
	if err := fw.watcher.Remove(absPath); err != nil {
		// 即使移除失败也不返回错误，因为文件可能已经不存在
		return nil
	}

	return nil
}

// Start 启动文件监控器
func (fw *FileWatcher) Start() error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	if fw.running {
		return fmt.Errorf("file watcher is already running")
	}

	fw.running = true
	go fw.eventLoop()

	return nil
}

// Stop 停止文件监控器
func (fw *FileWatcher) Stop() error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	if !fw.running {
		return nil
	}

	fw.running = false
	close(fw.stopCh)

	// 关闭 fsnotify watcher
	if err := fw.watcher.Close(); err != nil {
		return fmt.Errorf("failed to close fsnotify watcher: %w", err)
	}

	// 清空回调函数
	fw.callbacks = make(map[string][]func(types.FileEvent))

	return nil
}

// eventLoop 事件循环
func (fw *FileWatcher) eventLoop() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			// 记录错误但不停止监控
			fmt.Printf("File watcher error: %v\n", err)

		case <-fw.stopCh:
			return
		}
	}
}

// handleEvent 处理文件系统事件(立即串行执行)
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	fw.mutex.RLock()
	callbacks, exists := fw.callbacks[event.Name]
	fw.mutex.RUnlock()

	if !exists {
		return
	}

	// 将 fsnotify 事件转换为我们的 FileEvent
	fileEvent := fw.convertEvent(event)

	// 立即串行执行所有回调,不启动新的 goroutine
	// 这保证了事件处理的顺序性和原子性,类似 tail -f 的行为
	for _, callback := range callbacks {
		callback(fileEvent)
	}
}

// convertEvent 将 fsnotify.Event 转换为 types.FileEvent
func (fw *FileWatcher) convertEvent(event fsnotify.Event) types.FileEvent {
	var eventType string

	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		eventType = "create"
	case event.Op&fsnotify.Write == fsnotify.Write:
		eventType = "modify"
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = "delete"
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		eventType = "delete" // 重命名视为删除
	case event.Op&fsnotify.Chmod == fsnotify.Chmod:
		eventType = "modify" // 权限变化视为修改
	default:
		eventType = "modify" // 默认为修改
	}

	return types.FileEvent{
		Path: event.Name,
		Type: eventType,
	}
}
