package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/local-log-viewer/internal/types"
)

// JSONLogParser JSON格式日志解析器
type JSONLogParser struct {
	*BaseParser
}

// NewJSONLogParser 创建JSON日志解析器
func NewJSONLogParser() *JSONLogParser {
	return &JSONLogParser{
		BaseParser: NewBaseParser("JSON"),
	}
}

// Parse 解析JSON格式的日志行
func (p *JSONLogParser) Parse(line string) (*types.LogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	// 解析JSON
	data, err := ParseJSON(line)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	entry := &types.LogEntry{
		Raw:    line,
		Fields: make(map[string]interface{}),
	}

	// 提取时间戳
	if timestamp := p.extractTimestamp(data); !timestamp.IsZero() {
		entry.Timestamp = timestamp
	} else {
		entry.Timestamp = time.Now()
	}

	// 提取日志级别
	entry.Level = p.extractLevel(data)

	// 提取消息
	entry.Message = p.extractMessage(data)

	// 复制所有字段到Fields中
	for key, value := range data {
		entry.Fields[key] = value
	}

	// 设置日志类型
	entry.LogType = "JSON"

	return entry, nil
}

// CanParse 检查是否能解析指定内容
func (p *JSONLogParser) CanParse(content string) bool {
	return IsValidJSON(content)
}

// extractTimestamp 从JSON数据中提取时间戳
func (p *JSONLogParser) extractTimestamp(data map[string]interface{}) time.Time {
	// 常见的时间戳字段名
	timestampFields := []string{
		"timestamp", "time", "ts", "@timestamp", "datetime", "date",
		"created_at", "logged_at", "event_time",
	}

	for _, field := range timestampFields {
		if value, exists := data[field]; exists {
			switch v := value.(type) {
			case string:
				return p.ParseTimestamp(v)
			case float64:
				// Unix时间戳（秒）
				if v > 1000000000 && v < 10000000000 {
					return time.Unix(int64(v), 0)
				}
				// Unix时间戳（毫秒）
				if v > 1000000000000 {
					return time.Unix(int64(v/1000), int64(v)%1000*1000000)
				}
			case int64:
				// Unix时间戳（秒）
				if v > 1000000000 && v < 10000000000 {
					return time.Unix(v, 0)
				}
				// Unix时间戳（毫秒）
				if v > 1000000000000 {
					return time.Unix(v/1000, v%1000*1000000)
				}
			}
		}
	}

	return time.Time{}
}

// extractLevel 从JSON数据中提取日志级别
func (p *JSONLogParser) extractLevel(data map[string]interface{}) string {
	// 常见的级别字段名
	levelFields := []string{
		"level", "severity", "priority", "log_level", "loglevel",
		"type", "category",
	}

	for _, field := range levelFields {
		if value, exists := data[field]; exists {
			if levelStr, ok := value.(string); ok {
				level := strings.ToUpper(strings.TrimSpace(levelStr))
				// 标准化级别名称
				switch level {
				case "ERR", "ERROR":
					return "ERROR"
				case "WARN", "WARNING":
					return "WARN"
				case "INFO", "INFORMATION":
					return "INFO"
				case "DEBUG", "DBG":
					return "DEBUG"
				case "TRACE", "TRC":
					return "TRACE"
				case "FATAL", "CRIT", "CRITICAL":
					return "FATAL"
				default:
					if level != "" {
						return level
					}
				}
			}
		}
	}

	// 如果没有找到级别字段，尝试从消息中提取
	if message := p.extractMessage(data); message != "" {
		return p.ExtractLogLevel(message)
	}

	return "INFO"
}

// extractMessage 从JSON数据中提取消息
func (p *JSONLogParser) extractMessage(data map[string]interface{}) string {
	// 优先检查标准消息字段（不包括error）
	messageFields := []string{
		"message", "msg", "text", "content", "description", "detail",
	}

	for _, field := range messageFields {
		if value, exists := data[field]; exists {
			if messageStr, ok := value.(string); ok && messageStr != "" {
				return messageStr
			}
		}
	}

	// 如果没有找到标准消息字段，尝试构建消息
	var messageParts []string

	// 检查是否有错误信息
	if errorMsg, exists := data["error"]; exists {
		if errorStr, ok := errorMsg.(string); ok && errorStr != "" {
			messageParts = append(messageParts, "Error: "+errorStr)
		}
	}

	// 检查是否有事件类型
	if eventType, exists := data["event"]; exists {
		if eventStr, ok := eventType.(string); ok && eventStr != "" {
			messageParts = append(messageParts, "Event: "+eventStr)
		}
	}

	// 检查是否有操作信息
	if action, exists := data["action"]; exists {
		if actionStr, ok := action.(string); ok && actionStr != "" {
			messageParts = append(messageParts, "Action: "+actionStr)
		}
	}

	if len(messageParts) > 0 {
		return strings.Join(messageParts, " | ")
	}

	// 最后检查其他可能的消息字段
	otherFields := []string{"exception", "reason"}
	for _, field := range otherFields {
		if value, exists := data[field]; exists {
			if messageStr, ok := value.(string); ok && messageStr != "" {
				return messageStr
			}
		}
	}

	// 最后尝试使用原始JSON作为消息（截断长度）
	if len(data) > 0 {
		message := fmt.Sprintf("JSON log entry with %d fields", len(data))
		if len(message) > 100 {
			message = message[:97] + "..."
		}
		return message
	}

	return "JSON log entry"
}
