package parser

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/local-log-viewer/internal/interfaces"
)

// BaseParser 基础解析器实现
type BaseParser struct {
	format string
}

// NewBaseParser 创建基础解析器
func NewBaseParser(format string) *BaseParser {
	return &BaseParser{
		format: format,
	}
}

// GetFormat 获取格式名称
func (p *BaseParser) GetFormat() string {
	return p.format
}

// ParseTimestamp 解析时间戳的通用方法
func (p *BaseParser) ParseTimestamp(timestampStr string) time.Time {
	// 常见的时间格式
	timeFormats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"Jan 02 15:04:05",
		"02/Jan/2006:15:04:05 -0700",
	}

	for _, format := range timeFormats {
		if t, err := time.Parse(format, timestampStr); err == nil {
			return t
		}
	}

	// 如果无法解析，返回当前时间
	return time.Now()
}

// ExtractLogLevel 提取日志级别的通用方法
func (p *BaseParser) ExtractLogLevel(content string) string {
	content = strings.ToUpper(content)
	// 按优先级顺序检查，WARNING在WARN之前
	levels := []string{"ERROR", "WARNING", "WARN", "INFO", "DEBUG", "TRACE", "FATAL"}

	for _, level := range levels {
		if strings.Contains(content, level) {
			return level
		}
	}

	return "INFO" // 默认级别
}

// AutoDetector 自动检测器
type AutoDetector struct {
	parsers []interfaces.LogParser
}

// NewAutoDetector 创建自动检测器
func NewAutoDetector() *AutoDetector {
	return &AutoDetector{
		parsers: []interfaces.LogParser{
			NewJSONLogParser(),
			NewCommonLogParser(),
		},
	}
}

// DetectFormat 检测日志格式
func (d *AutoDetector) DetectFormat(content string) interfaces.LogParser {
	// 取前几行进行检测
	lines := strings.Split(content, "\n")
	sampleLines := make([]string, 0, 5)

	for i, line := range lines {
		if i >= 5 {
			break
		}
		if strings.TrimSpace(line) != "" {
			sampleLines = append(sampleLines, line)
		}
	}

	if len(sampleLines) == 0 {
		return NewCommonLogParser() // 默认使用通用解析器
	}

	// 检测每个解析器
	var bestParser interfaces.LogParser = NewCommonLogParser()
	bestScore := 0

	for _, parser := range d.parsers {
		canParseCount := 0
		for _, line := range sampleLines {
			if parser.CanParse(line) {
				canParseCount++
			}
		}

		// 如果这个解析器的得分更高，选择它
		if canParseCount > bestScore {
			bestScore = canParseCount
			bestParser = parser
		}

		// 如果超过一半的行都能解析，且是JSON解析器，优先选择
		if canParseCount > len(sampleLines)/2 && parser.GetFormat() == "JSON" {
			return parser
		}
	}

	return bestParser
}

// ParseJSON 解析JSON的辅助函数
func ParseJSON(line string) (map[string]interface{}, error) {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(line), &data)
	return data, err
}

// IsValidJSON 检查是否为有效JSON
func IsValidJSON(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
		return false
	}

	var data map[string]interface{}
	return json.Unmarshal([]byte(line), &data) == nil
}

// ExtractIPAddress 提取IP地址
func ExtractIPAddress(content string) string {
	ipRegex := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	if match := ipRegex.FindString(content); match != "" {
		return match
	}
	return ""
}

// ExtractStatusCode 提取HTTP状态码
func ExtractStatusCode(content string) string {
	statusRegex := regexp.MustCompile(`\b[1-5]\d{2}\b`)
	if match := statusRegex.FindString(content); match != "" {
		return match
	}
	return ""
}

// ExtractSize 提取大小信息
func ExtractSize(content string) int64 {
	// 优先匹配带有bytes或B后缀的数字
	sizeRegex := regexp.MustCompile(`\b(\d+)\s*(?:bytes?|B)\b`)
	if matches := sizeRegex.FindStringSubmatch(content); len(matches) > 1 {
		if size, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			return size
		}
	}

	// 如果没有找到带后缀的，查找日志中最后一个数字（通常是大小）
	allNumbersRegex := regexp.MustCompile(`\b(\d+)\b`)
	matches := allNumbersRegex.FindAllStringSubmatch(content, -1)
	if len(matches) > 0 {
		// 取最后一个数字
		lastMatch := matches[len(matches)-1]
		if len(lastMatch) > 1 {
			if size, err := strconv.ParseInt(lastMatch[1], 10, 64); err == nil {
				return size
			}
		}
	}

	return 0
}
