package types

import "time"

// LogFile 日志文件信息
type LogFile struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"modTime"`
	IsDirectory bool      `json:"isDirectory"`
	Children    []LogFile `json:"children,omitempty"`
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields"`
	Raw       string                 `json:"raw"`
	LineNum   int64                  `json:"lineNum"`
	LogType   string                 `json:"logType"` // JSON, WebServer, Generic
}

// LogContent 日志内容响应
type LogContent struct {
	Entries    []LogEntry `json:"entries"`
	TotalLines int64      `json:"totalLines"`
	HasMore    bool       `json:"hasMore"`
	Offset     int64      `json:"offset"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Path      string    `json:"path"`
	Query     string    `json:"query"`
	IsRegex   bool      `json:"isRegex"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Levels    []string  `json:"levels"`
	Offset    int       `json:"offset"`
	Limit     int       `json:"limit"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Entries    []LogEntry `json:"entries"`
	TotalCount int64      `json:"totalCount"`
	HasMore    bool       `json:"hasMore"`
	Offset     int        `json:"offset"`
}

// LogUpdate 日志更新事件
type LogUpdate struct {
	Path    string     `json:"path"`
	Entries []LogEntry `json:"entries"`
	Type    string     `json:"type"` // "append", "truncate", "delete"
}

// FileEvent 文件事件
type FileEvent struct {
	Path string `json:"path"`
	Type string `json:"type"` // "create", "modify", "delete"
}

// WSMessage WebSocket消息
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// ErrorResponse 错误响应格式
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}
