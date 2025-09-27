package parser

import (
	"testing"

	"github.com/local-log-viewer/internal/interfaces"
)

func TestParserFactory_NewParserFactory(t *testing.T) {
	factory := NewParserFactory()

	if factory == nil {
		t.Error("Expected factory to be created")
	}

	if factory.parsers == nil {
		t.Error("Expected parsers map to be initialized")
	}

	if factory.autoDetector == nil {
		t.Error("Expected autoDetector to be initialized")
	}

	// Check that default parsers are registered
	expectedParsers := []string{"json", "common", "apache", "nginx"}
	for _, name := range expectedParsers {
		if _, exists := factory.parsers[name]; !exists {
			t.Errorf("Expected parser %s to be registered", name)
		}
	}
}

func TestParserFactory_RegisterParser(t *testing.T) {
	factory := NewParserFactory()
	customParser := NewJSONLogParser()

	factory.RegisterParser("custom", customParser)

	if parser := factory.GetParser("custom"); parser != customParser {
		t.Error("Expected custom parser to be registered and retrievable")
	}

	// Test case insensitive registration
	factory.RegisterParser("CUSTOM2", customParser)
	if parser := factory.GetParser("custom2"); parser != customParser {
		t.Error("Expected case insensitive parser registration")
	}
}

func TestParserFactory_GetParser(t *testing.T) {
	factory := NewParserFactory()

	tests := []struct {
		name         string
		parserName   string
		expectFormat string
	}{
		{
			name:         "Get JSON parser",
			parserName:   "json",
			expectFormat: "JSON",
		},
		{
			name:         "Get common parser",
			parserName:   "common",
			expectFormat: "Common",
		},
		{
			name:         "Get apache parser (alias for common)",
			parserName:   "apache",
			expectFormat: "Common",
		},
		{
			name:         "Get nginx parser (alias for common)",
			parserName:   "nginx",
			expectFormat: "Common",
		},
		{
			name:         "Case insensitive",
			parserName:   "JSON",
			expectFormat: "JSON",
		},
		{
			name:         "Unknown parser (returns default)",
			parserName:   "unknown",
			expectFormat: "Common",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := factory.GetParser(tt.parserName)
			if parser == nil {
				t.Error("Expected parser to be returned")
				return
			}

			if parser.GetFormat() != tt.expectFormat {
				t.Errorf("Expected format %s, got %s", tt.expectFormat, parser.GetFormat())
			}
		})
	}
}

func TestParserFactory_GetParserByContent(t *testing.T) {
	factory := NewParserFactory()

	tests := []struct {
		name           string
		content        string
		expectedFormat string
	}{
		{
			name:           "JSON content",
			content:        `{"level": "info", "message": "test"}`,
			expectedFormat: "JSON",
		},
		{
			name:           "Apache log content",
			content:        `192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /index.html HTTP/1.1" 200 1234`,
			expectedFormat: "Common",
		},
		{
			name:           "Plain text content",
			content:        "This is just plain text",
			expectedFormat: "Common",
		},
		{
			name: "Mixed content (Common wins due to majority)",
			content: `{"level": "info", "message": "test"}
192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /index.html HTTP/1.1" 200 1234
Plain text line`,
			expectedFormat: "Common",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := factory.GetParserByContent(tt.content)
			if parser == nil {
				t.Error("Expected parser to be returned")
				return
			}

			if parser.GetFormat() != tt.expectedFormat {
				t.Errorf("Expected format %s, got %s", tt.expectedFormat, parser.GetFormat())
			}
		})
	}
}

func TestParserFactory_GetAvailableParsers(t *testing.T) {
	factory := NewParserFactory()
	parsers := factory.GetAvailableParsers()

	if len(parsers) == 0 {
		t.Error("Expected at least one parser to be available")
	}

	// Check that expected parsers are in the list
	expectedParsers := map[string]bool{
		"json":   false,
		"common": false,
		"apache": false,
		"nginx":  false,
	}

	for _, parser := range parsers {
		if _, exists := expectedParsers[parser]; exists {
			expectedParsers[parser] = true
		}
	}

	for name, found := range expectedParsers {
		if !found {
			t.Errorf("Expected parser %s to be in available parsers list", name)
		}
	}
}

func TestParserFactory_DetectFormat(t *testing.T) {
	factory := NewParserFactory()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "JSON format",
			content:  `{"level": "info", "message": "test"}`,
			expected: "JSON",
		},
		{
			name:     "Apache format",
			content:  `192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /index.html HTTP/1.1" 200 1234`,
			expected: "Common",
		},
		{
			name:     "Plain text",
			content:  "Just some plain text",
			expected: "Common",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format := factory.DetectFormat(tt.content)
			if format != tt.expected {
				t.Errorf("Expected format %s, got %s", tt.expected, format)
			}
		})
	}
}

func TestAutoDetector_DetectFormat(t *testing.T) {
	detector := NewAutoDetector()

	tests := []struct {
		name           string
		content        string
		expectedFormat string
	}{
		{
			name: "All JSON lines",
			content: `{"level": "info", "message": "test1"}
{"level": "error", "message": "test2"}
{"level": "warn", "message": "test3"}`,
			expectedFormat: "JSON",
		},
		{
			name: "All Apache lines",
			content: `192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /index.html HTTP/1.1" 200 1234
192.168.1.2 - - [07/Dec/2023:10:30:46 +0000] "POST /api/data HTTP/1.1" 201 567
192.168.1.3 - - [07/Dec/2023:10:30:47 +0000] "GET /style.css HTTP/1.1" 200 890`,
			expectedFormat: "Common",
		},
		{
			name: "Mixed content (no clear winner)",
			content: `{"level": "info", "message": "test1"}
192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /index.html HTTP/1.1" 200 1234
Plain text line
Another plain text line`,
			expectedFormat: "Common", // Default fallback
		},
		{
			name: "JSON majority",
			content: `{"level": "info", "message": "test1"}
{"level": "error", "message": "test2"}
{"level": "warn", "message": "test3"}
Plain text line`,
			expectedFormat: "JSON",
		},
		{
			name:           "Empty content",
			content:        "",
			expectedFormat: "Common", // Default fallback
		},
		{
			name: "Only empty lines",
			content: `


`,
			expectedFormat: "Common", // Default fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := detector.DetectFormat(tt.content)
			if parser == nil {
				t.Error("Expected parser to be returned")
				return
			}

			if parser.GetFormat() != tt.expectedFormat {
				t.Errorf("Expected format %s, got %s", tt.expectedFormat, parser.GetFormat())
			}
		})
	}
}

func TestAutoDetector_NewAutoDetector(t *testing.T) {
	detector := NewAutoDetector()

	if detector == nil {
		t.Error("Expected detector to be created")
	}

	if len(detector.parsers) == 0 {
		t.Error("Expected parsers to be initialized")
	}

	// Check that expected parsers are registered
	expectedFormats := map[string]bool{
		"JSON":   false,
		"Common": false,
	}

	for _, parser := range detector.parsers {
		format := parser.GetFormat()
		if _, exists := expectedFormats[format]; exists {
			expectedFormats[format] = true
		}
	}

	for format, found := range expectedFormats {
		if !found {
			t.Errorf("Expected parser with format %s to be registered", format)
		}
	}
}

// Test that parsers implement the interface correctly
func TestParsersImplementInterface(t *testing.T) {
	var _ interfaces.LogParser = NewJSONLogParser()
	var _ interfaces.LogParser = NewCommonLogParser()
}
