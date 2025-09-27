package search

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/local-log-viewer/internal/cache"
	"github.com/local-log-viewer/internal/interfaces"
	"github.com/local-log-viewer/internal/pool"
	"github.com/local-log-viewer/internal/types"
)

// SearchEngine 搜索引擎实现
type SearchEngine struct {
	parsers     map[string]interfaces.LogParser
	cache       interfaces.LogCache
	searchCache *cache.SearchCache
	filePool    *pool.FilePool
}

// NewSearchEngine 创建新的搜索引擎
func NewSearchEngine(parsers map[string]interfaces.LogParser, logCache interfaces.LogCache) *SearchEngine {
	// 创建搜索缓存
	searchCache := cache.NewSearchCache(logCache, 10*time.Minute, 1000)

	// 创建文件池
	poolConfig := pool.PoolConfig{
		InitialSize:         2,
		MaxSize:             5,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         30 * time.Minute,
		HealthCheckInterval: 5 * time.Minute,
	}
	filePool := pool.NewFilePool(poolConfig)

	return &SearchEngine{
		parsers:     parsers,
		cache:       logCache,
		searchCache: searchCache,
		filePool:    filePool,
	}
}

// Search 搜索日志
func (se *SearchEngine) Search(query types.SearchQuery) (*types.SearchResult, error) {
	// 尝试从缓存获取结果
	if result, found := se.searchCache.Get(query); found {
		return result, nil
	}

	// 使用文件池获取文件资源
	fileResource, err := se.filePool.GetFileResource(query.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file resource %s: %w", query.Path, err)
	}
	defer se.filePool.PutFileResource(query.Path, fileResource)

	// 重置文件位置
	if err := fileResource.Reset(); err != nil {
		return nil, fmt.Errorf("failed to reset file position: %w", err)
	}

	var results []types.LogEntry
	var totalCount int64
	var lineNum int64
	var processedCount int

	// 编译正则表达式（如果需要）
	var regex *regexp.Regexp
	if query.IsRegex {
		regex, err = regexp.Compile(query.Query)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	// 获取适合的解析器
	parser := se.getParserForFile(query.Path)

	// 使用优化的扫描器
	reader := fileResource.GetReader()
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024) // 64KB 缓冲区
	scanner.Buffer(buf, 1024*1024)  // 最大1MB行长度

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// 解析日志条目
		entry, err := se.parseLogEntry(line, lineNum, parser)
		if err != nil {
			// 如果解析失败，创建基本条目
			entry = &types.LogEntry{
				Raw:     line,
				LineNum: lineNum,
				Message: line,
			}
		}

		// 应用过滤条件
		if se.matchesQuery(entry, query, regex) {
			totalCount++

			// 应用分页
			if processedCount >= query.Offset && len(results) < query.Limit {
				// 高亮搜索结果
				highlightedEntry := se.highlightEntry(entry, query, regex)
				results = append(results, *highlightedEntry)
			}
			processedCount++

			// 如果已经收集足够的结果，可以继续计数但不添加到结果中
			if len(results) >= query.Limit {
				// 继续扫描以获取总数，但不添加到结果中
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	hasMore := totalCount > int64(query.Offset+query.Limit)

	result := &types.SearchResult{
		Entries:    results,
		TotalCount: totalCount,
		HasMore:    hasMore,
		Offset:     query.Offset,
	}

	// 缓存搜索结果
	se.searchCache.Set(query, result)

	return result, nil
}

// IndexFile 索引文件（当前实现为空，可以后续扩展）
func (se *SearchEngine) IndexFile(path string) error {
	// 当前实现不需要预索引，直接搜索文件
	// 可以在后续版本中添加索引功能以提高性能
	return nil
}

// RemoveIndex 移除索引
func (se *SearchEngine) RemoveIndex(path string) error {
	// 当前实现不需要索引，直接返回成功
	return nil
}

// getParserForFile 获取文件对应的解析器
func (se *SearchEngine) getParserForFile(path string) interfaces.LogParser {
	// 尝试从缓存获取文件内容片段来确定格式
	cacheKey := fmt.Sprintf("parser_%s", path)
	if cachedParser, exists := se.cache.Get(cacheKey); exists {
		if parser, ok := cachedParser.(interfaces.LogParser); ok {
			return parser
		}
	}

	// 读取文件前几行来确定格式
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var sampleLines []string
	for i := 0; i < 10 && scanner.Scan(); i++ {
		sampleLines = append(sampleLines, scanner.Text())
	}

	sampleContent := strings.Join(sampleLines, "\n")

	// 尝试各个解析器
	for _, parser := range se.parsers {
		if parser.CanParse(sampleContent) {
			se.cache.Set(cacheKey, parser)
			return parser
		}
	}

	return nil
}

// parseLogEntry 解析日志条目
func (se *SearchEngine) parseLogEntry(line string, lineNum int64, parser interfaces.LogParser) (*types.LogEntry, error) {
	if parser != nil {
		entry, err := parser.Parse(line)
		if err == nil {
			entry.LineNum = lineNum
			return entry, nil
		}
	}

	// 如果没有解析器或解析失败，返回基本条目
	return &types.LogEntry{
		Raw:     line,
		LineNum: lineNum,
		Message: line,
	}, nil
}

// matchesQuery 检查日志条目是否匹配查询条件
func (se *SearchEngine) matchesQuery(entry *types.LogEntry, query types.SearchQuery, regex *regexp.Regexp) bool {
	// 检查关键词匹配
	if query.Query != "" {
		var matched bool
		if query.IsRegex && regex != nil {
			matched = regex.MatchString(entry.Raw) || regex.MatchString(entry.Message)
		} else {
			queryLower := strings.ToLower(query.Query)
			matched = strings.Contains(strings.ToLower(entry.Raw), queryLower) ||
				strings.Contains(strings.ToLower(entry.Message), queryLower)
		}
		if !matched {
			return false
		}
	}

	// 检查时间范围过滤
	if !query.StartTime.IsZero() && entry.Timestamp.Before(query.StartTime) {
		return false
	}
	if !query.EndTime.IsZero() && entry.Timestamp.After(query.EndTime) {
		return false
	}

	// 检查日志级别过滤
	if len(query.Levels) > 0 && entry.Level != "" {
		levelMatched := false
		for _, level := range query.Levels {
			if strings.EqualFold(entry.Level, level) {
				levelMatched = true
				break
			}
		}
		if !levelMatched {
			return false
		}
	}

	return true
}

// highlightEntry 高亮搜索结果
func (se *SearchEngine) highlightEntry(entry *types.LogEntry, query types.SearchQuery, regex *regexp.Regexp) *types.LogEntry {
	if query.Query == "" {
		return entry
	}

	highlightedEntry := *entry

	if query.IsRegex && regex != nil {
		// 正则表达式高亮
		highlightedEntry.Message = regex.ReplaceAllStringFunc(entry.Message, func(match string) string {
			return fmt.Sprintf("<mark>%s</mark>", match)
		})
		highlightedEntry.Raw = regex.ReplaceAllStringFunc(entry.Raw, func(match string) string {
			return fmt.Sprintf("<mark>%s</mark>", match)
		})
	} else {
		// 关键词高亮
		queryLower := strings.ToLower(query.Query)

		// 高亮消息
		highlightedEntry.Message = se.highlightText(entry.Message, query.Query, queryLower)

		// 高亮原始内容
		highlightedEntry.Raw = se.highlightText(entry.Raw, query.Query, queryLower)
	}

	return &highlightedEntry
}

// highlightText 高亮文本中的关键词
func (se *SearchEngine) highlightText(text, originalQuery, queryLower string) string {
	if text == "" || queryLower == "" {
		return text
	}

	textLower := strings.ToLower(text)
	result := text

	// 找到所有匹配位置
	var matches [][]int
	start := 0
	for {
		index := strings.Index(textLower[start:], queryLower)
		if index == -1 {
			break
		}
		actualIndex := start + index
		matches = append(matches, []int{actualIndex, actualIndex + len(queryLower)})
		start = actualIndex + len(queryLower)
	}

	// 从后往前替换，避免位置偏移
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		matchedText := result[match[0]:match[1]]
		highlighted := fmt.Sprintf("<mark>%s</mark>", matchedText)
		result = result[:match[0]] + highlighted + result[match[1]:]
	}

	return result
}
