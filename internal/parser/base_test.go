package parser

import (
	"testing"
	"time"
)

func TestBaseParser_ParseTimestamp(t *testing.T) {
	parser := NewBaseParser("test")

	tests := []struct {
		name     string
		input    string
		expected bool // whether parsing should succeed
	}{
		{
			name:     "RFC3339 format",
			input:    "2023-12-07T10:30:45Z",
			expected: true,
		},
		{
			name:     "RFC3339 with nanoseconds",
			input:    "2023-12-07T10:30:45.123456789Z",
			expected: true,
		},
		{
			name:     "Standard datetime format",
			input:    "2023-12-07 10:30:45",
			expected: true,
		},
		{
			name:     "Apache log format",
			input:    "07/Dec/2023:10:30:45 +0000",
			expected: true,
		},
		{
			name:     "Syslog format",
			input:    "Dec 07 10:30:45",
			expected: true,
		},
		{
			name:     "Invalid format",
			input:    "invalid-timestamp",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseTimestamp(tt.input)

			if tt.expected {
				// Should not be zero time
				if result.IsZero() {
					t.Errorf("Expected valid timestamp, got zero time")
				}
			} else {
				// Should be current time (fallback)
				now := time.Now()
				if result.Sub(now) > time.Second {
					t.Errorf("Expected current time as fallback, got %v", result)
				}
			}
		})
	}
}

func TestBaseParser_ExtractLogLevel(t *testing.T) {
	parser := NewBaseParser("test")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Error level",
			input:    "This is an ERROR message",
			expected: "ERROR",
		},
		{
			name:     "Warning level",
			input:    "This is a WARN message",
			expected: "WARN",
		},
		{
			name:     "Warning level (full word)",
			input:    "This is a WARNING message",
			expected: "WARNING",
		},
		{
			name:     "Info level",
			input:    "This is an INFO message",
			expected: "INFO",
		},
		{
			name:     "Debug level",
			input:    "This is a DEBUG message",
			expected: "DEBUG",
		},
		{
			name:     "Trace level",
			input:    "This is a TRACE message",
			expected: "TRACE",
		},
		{
			name:     "Fatal level",
			input:    "This is a FATAL message",
			expected: "FATAL",
		},
		{
			name:     "No level (default to INFO)",
			input:    "This is a regular message",
			expected: "INFO",
		},
		{
			name:     "Case insensitive",
			input:    "this is an error message",
			expected: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ExtractLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIsValidJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid JSON object",
			input:    `{"level": "info", "message": "test"}`,
			expected: true,
		},
		{
			name:     "Valid JSON with nested object",
			input:    `{"level": "info", "data": {"key": "value"}}`,
			expected: true,
		},
		{
			name:     "Invalid JSON - missing quotes",
			input:    `{level: info, message: test}`,
			expected: false,
		},
		{
			name:     "Invalid JSON - not an object",
			input:    `["array", "not", "object"]`,
			expected: false,
		},
		{
			name:     "Plain text",
			input:    "This is not JSON",
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Only braces",
			input:    "{}",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidJSON(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractIPAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Valid IP in log line",
			input:    "192.168.1.1 - - [07/Dec/2023:10:30:45 +0000]",
			expected: "192.168.1.1",
		},
		{
			name:     "IP in middle of text",
			input:    "Connection from 10.0.0.1 established",
			expected: "10.0.0.1",
		},
		{
			name:     "No IP address",
			input:    "This line has no IP address",
			expected: "",
		},
		{
			name:     "Invalid IP format",
			input:    "999.999.999.999 is not valid",
			expected: "999.999.999.999", // Regex matches, validation is elsewhere
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractIPAddress(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestExtractStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTP 200 status",
			input:    `"GET /index.html HTTP/1.1" 200 1234`,
			expected: "200",
		},
		{
			name:     "HTTP 404 status",
			input:    `"GET /missing.html HTTP/1.1" 404 0`,
			expected: "404",
		},
		{
			name:     "HTTP 500 status",
			input:    `"POST /api/data HTTP/1.1" 500 567`,
			expected: "500",
		},
		{
			name:     "No status code",
			input:    "This line has no status code",
			expected: "",
		},
		{
			name:     "Invalid status code",
			input:    "Status: 999",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractStatusCode(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestExtractSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			name:     "Size in bytes",
			input:    `"GET /file.txt HTTP/1.1" 200 1234`,
			expected: 1234,
		},
		{
			name:     "Size with bytes suffix",
			input:    "File size: 5678 bytes",
			expected: 5678,
		},
		{
			name:     "Size with B suffix",
			input:    "Downloaded 9876 B",
			expected: 9876,
		},
		{
			name:     "No size information",
			input:    "This line has no size",
			expected: 0,
		},
		{
			name:     "Multiple numbers (last match)",
			input:    "Status 200, size 1024, time 500ms",
			expected: 1024, // Last standalone number (500ms is not matched as it has 'ms')
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSize(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}
