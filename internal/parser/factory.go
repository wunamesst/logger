package parser

import (
	"strings"

	"github.com/local-log-viewer/internal/interfaces"
)

// ParserFactory 解析器工厂
type ParserFactory struct {
	parsers      map[string]interfaces.LogParser
	autoDetector *AutoDetector
}

// NewParserFactory 创建解析器工厂
func NewParserFactory() *ParserFactory {
	factory := &ParserFactory{
		parsers:      make(map[string]interfaces.LogParser),
		autoDetector: NewAutoDetector(),
	}

	// 注册默认解析器
	factory.RegisterParser("json", NewJSONLogParser())
	factory.RegisterParser("common", NewCommonLogParser())
	factory.RegisterParser("apache", NewCommonLogParser())
	factory.RegisterParser("nginx", NewCommonLogParser())

	return factory
}

// RegisterParser 注册解析器
func (f *ParserFactory) RegisterParser(name string, parser interfaces.LogParser) {
	f.parsers[strings.ToLower(name)] = parser
}

// GetParser 根据名称获取解析器
func (f *ParserFactory) GetParser(name string) interfaces.LogParser {
	if parser, exists := f.parsers[strings.ToLower(name)]; exists {
		return parser
	}
	return f.parsers["common"] // 默认返回通用解析器
}

// GetParserByContent 根据内容自动检测并返回合适的解析器
func (f *ParserFactory) GetParserByContent(content string) interfaces.LogParser {
	return f.autoDetector.DetectFormat(content)
}

// GetAvailableParsers 获取所有可用的解析器名称
func (f *ParserFactory) GetAvailableParsers() []string {
	var names []string
	for name := range f.parsers {
		names = append(names, name)
	}
	return names
}

// DetectFormat 检测日志格式并返回格式名称
func (f *ParserFactory) DetectFormat(content string) string {
	parser := f.autoDetector.DetectFormat(content)
	return parser.GetFormat()
}
