package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/local-log-viewer/internal/types"
)

// CommonLogParser 通用日志格式解析器（Apache、Nginx等）
type CommonLogParser struct {
	*BaseParser
	// 预编译的正则表达式
	apacheCommonRegex     *regexp.Regexp
	apacheCombinedRegex   *regexp.Regexp
	nginxAccessRegex      *regexp.Regexp
	nginxErrorRegex       *regexp.Regexp
	genericTimestampRegex *regexp.Regexp
}

// NewCommonLogParser 创建通用日志解析器
func NewCommonLogParser() *CommonLogParser {
	return &CommonLogParser{
		BaseParser: NewBaseParser("Common"),
		// Apache Common Log Format
		apacheCommonRegex: regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "([^"]*)" (\d+) (\S+)`),
		// Apache Combined Log Format
		apacheCombinedRegex: regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "([^"]*)" (\d+) (\S+) "([^"]*)" "([^"]*)"`),
		// Nginx Access Log
		nginxAccessRegex: regexp.MustCompile(`^(\S+) - \S+ \[([^\]]+)\] "([^"]*)" (\d+) (\d+) "([^"]*)" "([^"]*)"`),
		// Nginx Error Log
		nginxErrorRegex: regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[(\w+)\] \d+#\d+: (.+)`),
		// Generic timestamp pattern
		genericTimestampRegex: regexp.MustCompile(`(\d{4}[-/]\d{2}[-/]\d{2}[\sT]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)`),
	}
}

// Parse 解析通用格式的日志行
func (p *CommonLogParser) Parse(line string) (*types.LogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	entry := &types.LogEntry{
		Raw:    line,
		Fields: make(map[string]interface{}),
	}

	// 尝试不同的解析方法
	if p.parseApacheCombined(line, entry) {
		return entry, nil
	}
	if p.parseApacheCommon(line, entry) {
		return entry, nil
	}
	if p.parseNginxAccess(line, entry) {
		return entry, nil
	}
	if p.parseNginxError(line, entry) {
		return entry, nil
	}
	if p.parseGeneric(line, entry) {
		return entry, nil
	}

	// 如果所有格式都无法解析，使用基础解析
	p.parseBasic(line, entry)
	return entry, nil
}

// CanParse 检查是否能解析指定内容
func (p *CommonLogParser) CanParse(content string) bool {
	// 检查是否包含常见的日志模式
	patterns := []*regexp.Regexp{
		p.apacheCommonRegex,
		p.apacheCombinedRegex,
		p.nginxAccessRegex,
		p.nginxErrorRegex,
		p.genericTimestampRegex,
	}

	for _, pattern := range patterns {
		if pattern.MatchString(content) {
			return true
		}
	}

	// 检查是否包含常见的日志级别
	upperContent := strings.ToUpper(content)
	levels := []string{"ERROR", "WARN", "INFO", "DEBUG", "TRACE", "FATAL"}
	for _, level := range levels {
		if strings.Contains(upperContent, level) {
			return true
		}
	}

	return true // 通用解析器可以解析任何内容
}

// parseApacheCombined 解析Apache Combined Log Format
func (p *CommonLogParser) parseApacheCombined(line string, entry *types.LogEntry) bool {
	matches := p.apacheCombinedRegex.FindStringSubmatch(line)
	if len(matches) < 8 {
		return false
	}

	// 解析时间戳
	entry.Timestamp = p.ParseTimestamp(matches[2])

	// 提取字段
	entry.Fields["remote_addr"] = matches[1]
	entry.Fields["request"] = matches[3]
	entry.Fields["status"] = matches[4]
	entry.Fields["body_bytes_sent"] = matches[5]
	entry.Fields["http_referer"] = matches[6]
	entry.Fields["http_user_agent"] = matches[7]

	// 根据状态码确定日志级别
	if status, err := strconv.Atoi(matches[4]); err == nil {
		if status >= 500 {
			entry.Level = "ERROR"
		} else if status >= 400 {
			entry.Level = "WARN"
		} else {
			entry.Level = "INFO"
		}
	} else {
		entry.Level = "INFO"
	}

	// 构建消息
	entry.Message = fmt.Sprintf("%s %s - %s", matches[1], matches[3], matches[4])

	// 设置日志类型
	entry.LogType = "WebServer"

	return true
}

// parseApacheCommon 解析Apache Common Log Format
func (p *CommonLogParser) parseApacheCommon(line string, entry *types.LogEntry) bool {
	matches := p.apacheCommonRegex.FindStringSubmatch(line)
	if len(matches) < 6 {
		return false
	}

	// 解析时间戳
	entry.Timestamp = p.ParseTimestamp(matches[2])

	// 提取字段
	entry.Fields["remote_addr"] = matches[1]
	entry.Fields["request"] = matches[3]
	entry.Fields["status"] = matches[4]
	entry.Fields["body_bytes_sent"] = matches[5]

	// 根据状态码确定日志级别
	if status, err := strconv.Atoi(matches[4]); err == nil {
		if status >= 500 {
			entry.Level = "ERROR"
		} else if status >= 400 {
			entry.Level = "WARN"
		} else {
			entry.Level = "INFO"
		}
	} else {
		entry.Level = "INFO"
	}

	// 构建消息
	entry.Message = fmt.Sprintf("%s %s - %s", matches[1], matches[3], matches[4])

	// 设置日志类型
	entry.LogType = "WebServer"

	return true
}

// parseNginxAccess 解析Nginx访问日志
func (p *CommonLogParser) parseNginxAccess(line string, entry *types.LogEntry) bool {
	matches := p.nginxAccessRegex.FindStringSubmatch(line)
	if len(matches) < 8 {
		return false
	}

	// 解析时间戳
	entry.Timestamp = p.ParseTimestamp(matches[2])

	// 提取字段
	entry.Fields["remote_addr"] = matches[1]
	entry.Fields["request"] = matches[3]
	entry.Fields["status"] = matches[4]
	entry.Fields["body_bytes_sent"] = matches[5]
	entry.Fields["http_referer"] = matches[6]
	entry.Fields["http_user_agent"] = matches[7]

	// 根据状态码确定日志级别
	if status, err := strconv.Atoi(matches[4]); err == nil {
		if status >= 500 {
			entry.Level = "ERROR"
		} else if status >= 400 {
			entry.Level = "WARN"
		} else {
			entry.Level = "INFO"
		}
	} else {
		entry.Level = "INFO"
	}

	// 构建消息
	entry.Message = fmt.Sprintf("%s %s - %s", matches[1], matches[3], matches[4])

	// 设置日志类型
	entry.LogType = "WebServer"

	return true
}

// parseNginxError 解析Nginx错误日志
func (p *CommonLogParser) parseNginxError(line string, entry *types.LogEntry) bool {
	matches := p.nginxErrorRegex.FindStringSubmatch(line)
	if len(matches) < 4 {
		return false
	}

	// 解析时间戳
	entry.Timestamp = p.ParseTimestamp(matches[1])

	// 提取字段
	entry.Fields["error_level"] = matches[2]
	entry.Message = matches[3]

	// 设置日志级别
	switch strings.ToUpper(matches[2]) {
	case "EMERG", "ALERT", "CRIT":
		entry.Level = "FATAL"
	case "ERR", "ERROR":
		entry.Level = "ERROR"
	case "WARN", "WARNING":
		entry.Level = "WARN"
	case "NOTICE", "INFO":
		entry.Level = "INFO"
	case "DEBUG":
		entry.Level = "DEBUG"
	default:
		entry.Level = "INFO"
	}

	// 设置日志类型
	entry.LogType = "WebServer"

	return true
}

// parseGeneric 解析通用格式（包含时间戳的日志）
func (p *CommonLogParser) parseGeneric(line string, entry *types.LogEntry) bool {
	// 查找时间戳
	matches := p.genericTimestampRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return false
	}

	// 解析时间戳
	entry.Timestamp = p.ParseTimestamp(matches[1])

	// 提取日志级别
	entry.Level = p.ExtractLogLevel(line)

	// 消息是去掉时间戳后的内容
	message := strings.TrimSpace(strings.Replace(line, matches[1], "", 1))
	if message == "" {
		message = line
	}
	entry.Message = message

	// 尝试提取更多字段
	if ip := ExtractIPAddress(line); ip != "" {
		entry.Fields["ip_address"] = ip
	}
	if status := ExtractStatusCode(line); status != "" {
		entry.Fields["status_code"] = status
	}
	if size := ExtractSize(line); size > 0 {
		entry.Fields["size"] = size
	}

	// 设置日志类型
	entry.LogType = "Generic"

	return true
}

// parseBasic 基础解析（最后的备选方案）
func (p *CommonLogParser) parseBasic(line string, entry *types.LogEntry) {
	// 使用当前时间作为时间戳
	entry.Timestamp = time.Now()

	// 提取日志级别
	entry.Level = p.ExtractLogLevel(line)

	// 整行作为消息
	entry.Message = line

	// 尝试提取一些基本字段
	if ip := ExtractIPAddress(line); ip != "" {
		entry.Fields["ip_address"] = ip
	}
	if status := ExtractStatusCode(line); status != "" {
		entry.Fields["status_code"] = status
	}

	// 设置日志类型
	entry.LogType = "Generic"
}
