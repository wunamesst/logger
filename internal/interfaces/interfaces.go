package interfaces

import (
	"github.com/local-log-viewer/internal/types"
)

// LogManager 日志管理器接口
type LogManager interface {
	// GetLogFiles 获取日志文件列表
	GetLogFiles() ([]types.LogFile, error)

	// GetDirectoryFiles 获取指定目录的直接子节点(用于懒加载)
	GetDirectoryFiles(dirPath string) ([]types.LogFile, error)

	// ReadLogFile 读取日志文件内容
	ReadLogFile(path string, offset int64, limit int) (*types.LogContent, error)

	// ReadLogFileFromTail 从文件尾部读取日志内容
	ReadLogFileFromTail(path string, lines int) (*types.LogContent, error)

	// SearchLogs 搜索日志内容
	SearchLogs(query types.SearchQuery) (*types.SearchResult, error)

	// WatchFile 监控文件变化
	WatchFile(path string) (<-chan types.LogUpdate, error)

	// GetLogPaths 获取配置的日志目录列表
	GetLogPaths() []string

	// Start 启动日志管理器
	Start() error

	// Stop 停止日志管理器
	Stop() error
}

// FileWatcher 文件监控器接口
type FileWatcher interface {
	// WatchFile 监控文件
	WatchFile(path string, callback func(types.FileEvent)) error

	// UnwatchFile 取消监控文件
	UnwatchFile(path string) error

	// Start 启动文件监控器
	Start() error

	// Stop 停止文件监控器
	Stop() error
}

// LogParser 日志解析器接口
type LogParser interface {
	// Parse 解析日志行
	Parse(line string) (*types.LogEntry, error)

	// CanParse 检查是否能解析指定内容
	CanParse(content string) bool

	// GetFormat 获取格式名称
	GetFormat() string
}

// WebSocketHub WebSocket中心接口
type WebSocketHub interface {
	// Run 运行WebSocket中心
	Run()

	// BroadcastLogUpdate 广播日志更新
	BroadcastLogUpdate(update types.LogUpdate)

	// RegisterClient 注册客户端
	RegisterClient(client WebSocketClient)

	// UnregisterClient 注销客户端
	UnregisterClient(client WebSocketClient)

	// Start 启动WebSocket中心
	Start() error

	// Stop 停止WebSocket中心
	Stop() error
}

// WebSocketClient WebSocket客户端接口
type WebSocketClient interface {
	// Send 发送消息
	Send(message types.WSMessage) error

	// Close 关闭连接
	Close() error

	// GetID 获取客户端ID
	GetID() string
}

// SearchEngine 搜索引擎接口
type SearchEngine interface {
	// Search 搜索日志
	Search(query types.SearchQuery) (*types.SearchResult, error)

	// IndexFile 索引文件
	IndexFile(path string) error

	// RemoveIndex 移除索引
	RemoveIndex(path string) error
}

// LogCache 日志缓存接口
type LogCache interface {
	// Get 获取缓存内容
	Get(key string) (interface{}, bool)

	// Set 设置缓存内容
	Set(key string, value interface{})

	// Delete 删除缓存内容
	Delete(key string)

	// Clear 清空缓存
	Clear()
}

// AdvancedLogCache 高级日志缓存接口
type AdvancedLogCache interface {
	LogCache

	// GetStats 获取缓存统计信息
	GetStats() interface{}

	// Stop 停止缓存
	Stop()
}
