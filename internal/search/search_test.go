package search

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/local-log-viewer/internal/cache"
	"github.com/local-log-viewer/internal/interfaces"
	"github.com/local-log-viewer/internal/types"
)

// mockParser 模拟解析器
type mockParser struct {
	format   string
	canParse bool
}

func (mp *mockParser) Parse(line string) (*types.LogEntry, error) {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return &types.LogEntry{
			Raw:     line,
			Message: line,
		}, nil
	}

	timestamp, _ := time.Parse("2006-01-02T15:04:05", parts[0])
	level := parts[1]
	message := strings.Join(parts[2:], " ")

	return &types.LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
		Raw:       line,
	}, nil
}

func (mp *mockParser) CanParse(content string) bool {
	return mp.canParse
}

func (mp *mockParser) GetFormat() string {
	return mp.format
}

func createTestFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.log")

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	return filePath
}

func TestNewSearchEngine(t *testing.T) {
	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)

	se := NewSearchEngine(parsers, cache)

	if se == nil {
		t.Fatal("Expected non-nil SearchEngine")
	}
	if se.parsers == nil {
		t.Fatal("Expected parsers to be set")
	}
	if se.cache == nil {
		t.Fatal("Expected cache to be set")
	}
}

func TestSearch_BasicKeywordSearch(t *testing.T) {
	content := `2023-01-01T10:00:00 INFO This is a test message
2023-01-01T10:01:00 ERROR An error occurred
2023-01-01T10:02:00 INFO Another test message
2023-01-01T10:03:00 WARN Warning message`

	filePath := createTestFile(t, content)

	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)
	se := NewSearchEngine(parsers, cache)

	query := types.SearchQuery{
		Path:   filePath,
		Query:  "test",
		Offset: 0,
		Limit:  10,
	}

	result, err := se.Search(query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if result.TotalCount != 2 {
		t.Errorf("Expected 2 matches, got %d", result.TotalCount)
	}

	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(result.Entries))
	}

	// 检查高亮
	for _, entry := range result.Entries {
		if !strings.Contains(entry.Message, "<mark>test</mark>") {
			t.Errorf("Expected highlighted 'test' in message: %s", entry.Message)
		}
	}
}

func TestSearch_RegexSearch(t *testing.T) {
	content := `2023-01-01T10:00:00 INFO Message with number 123
2023-01-01T10:01:00 ERROR Error code 456
2023-01-01T10:02:00 INFO No numbers here
2023-01-01T10:03:00 WARN Code 789 found`

	filePath := createTestFile(t, content)

	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)
	se := NewSearchEngine(parsers, cache)

	query := types.SearchQuery{
		Path:    filePath,
		Query:   `\d{3}`,
		IsRegex: true,
		Offset:  0,
		Limit:   10,
	}

	result, err := se.Search(query)
	if err != nil {
		t.Fatalf("Regex search failed: %v", err)
	}

	// 所有行都包含3位数字序列（时间戳中的数字），所以应该是4个匹配
	if result.TotalCount != 4 {
		t.Errorf("Expected 4 matches, got %d", result.TotalCount)
	}

	// 检查正则表达式高亮
	for _, entry := range result.Entries {
		if !strings.Contains(entry.Raw, "<mark>") && !strings.Contains(entry.Message, "<mark>") {
			t.Errorf("Expected highlighted regex match in entry: %s", entry.Raw)
		}
	}
}

func TestSearch_InvalidRegex(t *testing.T) {
	content := `2023-01-01T10:00:00 INFO Test message`
	filePath := createTestFile(t, content)

	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)
	se := NewSearchEngine(parsers, cache)

	query := types.SearchQuery{
		Path:    filePath,
		Query:   `[invalid regex`,
		IsRegex: true,
		Offset:  0,
		Limit:   10,
	}

	_, err := se.Search(query)
	if err == nil {
		t.Fatal("Expected error for invalid regex")
	}

	if !strings.Contains(err.Error(), "invalid regex pattern") {
		t.Errorf("Expected 'invalid regex pattern' error, got: %v", err)
	}
}

func TestSearch_TimeRangeFilter(t *testing.T) {
	content := `2023-01-01T10:00:00 INFO Message 1
2023-01-01T11:00:00 INFO Message 2
2023-01-01T12:00:00 INFO Message 3
2023-01-01T13:00:00 INFO Message 4`

	filePath := createTestFile(t, content)

	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)
	se := NewSearchEngine(parsers, cache)

	startTime, _ := time.Parse("2006-01-02T15:04:05", "2023-01-01T10:30:00")
	endTime, _ := time.Parse("2006-01-02T15:04:05", "2023-01-01T12:30:00")

	query := types.SearchQuery{
		Path:      filePath,
		Query:     "Message",
		StartTime: startTime,
		EndTime:   endTime,
		Offset:    0,
		Limit:     10,
	}

	result, err := se.Search(query)
	if err != nil {
		t.Fatalf("Time range search failed: %v", err)
	}

	if result.TotalCount != 2 {
		t.Errorf("Expected 2 matches in time range, got %d", result.TotalCount)
	}

	// 验证返回的条目在时间范围内
	for _, entry := range result.Entries {
		if entry.Timestamp.Before(startTime) || entry.Timestamp.After(endTime) {
			t.Errorf("Entry timestamp %v is outside range [%v, %v]",
				entry.Timestamp, startTime, endTime)
		}
	}
}

func TestSearch_LevelFilter(t *testing.T) {
	content := `2023-01-01T10:00:00 INFO Info message
2023-01-01T10:01:00 ERROR Error message
2023-01-01T10:02:00 WARN Warning message
2023-01-01T10:03:00 INFO Another info message
2023-01-01T10:04:00 DEBUG Debug message`

	filePath := createTestFile(t, content)

	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)
	se := NewSearchEngine(parsers, cache)

	query := types.SearchQuery{
		Path:   filePath,
		Query:  "message",
		Levels: []string{"INFO", "ERROR"},
		Offset: 0,
		Limit:  10,
	}

	result, err := se.Search(query)
	if err != nil {
		t.Fatalf("Level filter search failed: %v", err)
	}

	if result.TotalCount != 3 {
		t.Errorf("Expected 3 matches with INFO/ERROR levels, got %d", result.TotalCount)
	}

	// 验证返回的条目级别正确
	for _, entry := range result.Entries {
		if entry.Level != "INFO" && entry.Level != "ERROR" {
			t.Errorf("Entry level %s is not in allowed levels [INFO, ERROR]", entry.Level)
		}
	}
}

func TestSearch_Pagination(t *testing.T) {
	content := `2023-01-01T10:00:00 INFO Message 1
2023-01-01T10:01:00 INFO Message 2
2023-01-01T10:02:00 INFO Message 3
2023-01-01T10:03:00 INFO Message 4
2023-01-01T10:04:00 INFO Message 5`

	filePath := createTestFile(t, content)

	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)
	se := NewSearchEngine(parsers, cache)

	// 第一页
	query := types.SearchQuery{
		Path:   filePath,
		Query:  "Message",
		Offset: 0,
		Limit:  2,
	}

	result, err := se.Search(query)
	if err != nil {
		t.Fatalf("Pagination search failed: %v", err)
	}

	if result.TotalCount != 5 {
		t.Errorf("Expected total count 5, got %d", result.TotalCount)
	}

	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries in first page, got %d", len(result.Entries))
	}

	if !result.HasMore {
		t.Error("Expected HasMore to be true")
	}

	// 第二页
	query.Offset = 2
	result, err = se.Search(query)
	if err != nil {
		t.Fatalf("Second page search failed: %v", err)
	}

	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries in second page, got %d", len(result.Entries))
	}

	// 第三页
	query.Offset = 4
	result, err = se.Search(query)
	if err != nil {
		t.Fatalf("Third page search failed: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Errorf("Expected 1 entry in third page, got %d", len(result.Entries))
	}

	if result.HasMore {
		t.Error("Expected HasMore to be false on last page")
	}
}

func TestSearch_FileNotFound(t *testing.T) {
	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)
	se := NewSearchEngine(parsers, cache)

	query := types.SearchQuery{
		Path:   "/nonexistent/file.log",
		Query:  "test",
		Offset: 0,
		Limit:  10,
	}

	_, err := se.Search(query)
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}

	if !strings.Contains(err.Error(), "failed to open file") {
		t.Errorf("Expected 'failed to open file' error, got: %v", err)
	}
}
func TestSearch_EmptyQuery(t *testing.T) {
	content := `2023-01-01T10:00:00 INFO Message 1
2023-01-01T10:01:00 ERROR Message 2`

	filePath := createTestFile(t, content)

	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)
	se := NewSearchEngine(parsers, cache)

	query := types.SearchQuery{
		Path:   filePath,
		Query:  "", // 空查询应该返回所有条目
		Offset: 0,
		Limit:  10,
	}

	result, err := se.Search(query)
	if err != nil {
		t.Fatalf("Empty query search failed: %v", err)
	}

	if result.TotalCount != 2 {
		t.Errorf("Expected 2 matches for empty query, got %d", result.TotalCount)
	}
}

func TestHighlightText(t *testing.T) {
	se := &SearchEngine{}

	tests := []struct {
		text          string
		originalQuery string
		queryLower    string
		expected      string
	}{
		{
			text:          "This is a test message",
			originalQuery: "test",
			queryLower:    "test",
			expected:      "This is a <mark>test</mark> message",
		},
		{
			text:          "Test case with Test repeated",
			originalQuery: "test",
			queryLower:    "test",
			expected:      "<mark>Test</mark> case with <mark>Test</mark> repeated",
		},
		{
			text:          "No matches here",
			originalQuery: "xyz",
			queryLower:    "xyz",
			expected:      "No matches here",
		},
		{
			text:          "",
			originalQuery: "test",
			queryLower:    "test",
			expected:      "",
		},
	}

	for i, test := range tests {
		result := se.highlightText(test.text, test.originalQuery, test.queryLower)
		if result != test.expected {
			t.Errorf("Test %d: expected %q, got %q", i, test.expected, result)
		}
	}
}

func TestMatchesQuery(t *testing.T) {
	se := &SearchEngine{}

	entry := &types.LogEntry{
		Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Level:     "INFO",
		Message:   "This is a test message",
		Raw:       "2023-01-01T12:00:00 INFO This is a test message",
	}

	tests := []struct {
		name     string
		query    types.SearchQuery
		expected bool
	}{
		{
			name: "keyword match",
			query: types.SearchQuery{
				Query: "test",
			},
			expected: true,
		},
		{
			name: "keyword no match",
			query: types.SearchQuery{
				Query: "xyz",
			},
			expected: false,
		},
		{
			name: "regex match",
			query: types.SearchQuery{
				Query:   `\btest\b`,
				IsRegex: true,
			},
			expected: true,
		},
		{
			name: "time range match",
			query: types.SearchQuery{
				StartTime: time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2023, 1, 1, 13, 0, 0, 0, time.UTC),
			},
			expected: true,
		},
		{
			name: "time range no match",
			query: types.SearchQuery{
				StartTime: time.Date(2023, 1, 1, 13, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2023, 1, 1, 14, 0, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name: "level match",
			query: types.SearchQuery{
				Levels: []string{"INFO", "ERROR"},
			},
			expected: true,
		},
		{
			name: "level no match",
			query: types.SearchQuery{
				Levels: []string{"ERROR", "WARN"},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var regex *regexp.Regexp
			if test.query.IsRegex {
				regex, _ = regexp.Compile(test.query.Query)
			}

			result := se.matchesQuery(entry, test.query, regex)
			if result != test.expected {
				t.Errorf("Expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestIndexFile(t *testing.T) {
	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)
	se := NewSearchEngine(parsers, cache)

	// 当前实现应该总是返回 nil
	err := se.IndexFile("/some/path")
	if err != nil {
		t.Errorf("IndexFile should return nil, got: %v", err)
	}
}

func TestRemoveIndex(t *testing.T) {
	parsers := map[string]interfaces.LogParser{
		"test": &mockParser{format: "test", canParse: true},
	}
	cache := cache.NewMemoryCache(100, time.Hour)
	se := NewSearchEngine(parsers, cache)

	// 当前实现应该总是返回 nil
	err := se.RemoveIndex("/some/path")
	if err != nil {
		t.Errorf("RemoveIndex should return nil, got: %v", err)
	}
}
