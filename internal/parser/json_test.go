package parser

import (
	"testing"
	"time"

	"github.com/local-log-viewer/internal/types"
)

func TestJSONLogParser_Parse(t *testing.T) {
	parser := NewJSONLogParser()

	tests := []struct {
		name        string
		input       string
		expectError bool
		checkFields func(t *testing.T, entry *types.LogEntry)
	}{
		{
			name:        "Valid JSON with standard fields",
			input:       `{"timestamp": "2023-12-07T10:30:45Z", "level": "info", "message": "Test message"}`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "INFO" {
					t.Errorf("Expected level INFO, got %s", entry.Level)
				}
				if entry.Message != "Test message" {
					t.Errorf("Expected message 'Test message', got %s", entry.Message)
				}
				if entry.Timestamp.IsZero() {
					t.Error("Expected valid timestamp")
				}
			},
		},
		{
			name:        "JSON with Unix timestamp",
			input:       `{"ts": 1701944445, "level": "error", "msg": "Error occurred"}`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "ERROR" {
					t.Errorf("Expected level ERROR, got %s", entry.Level)
				}
				if entry.Message != "Error occurred" {
					t.Errorf("Expected message 'Error occurred', got %s", entry.Message)
				}
				expectedTime := time.Unix(1701944445, 0)
				if !entry.Timestamp.Equal(expectedTime) {
					t.Errorf("Expected timestamp %v, got %v", expectedTime, entry.Timestamp)
				}
			},
		},
		{
			name:        "JSON with millisecond timestamp",
			input:       `{"time": 1701944445123, "severity": "warn", "text": "Warning message"}`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "WARN" {
					t.Errorf("Expected level WARN, got %s", entry.Level)
				}
				if entry.Message != "Warning message" {
					t.Errorf("Expected message 'Warning message', got %s", entry.Message)
				}
			},
		},
		{
			name:        "JSON without explicit message field",
			input:       `{"level": "debug", "event": "user_login", "user_id": 123}`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "DEBUG" {
					t.Errorf("Expected level DEBUG, got %s", entry.Level)
				}
				if entry.Message != "Event: user_login" {
					t.Errorf("Expected constructed message, got %s", entry.Message)
				}
				if entry.Fields["user_id"] != float64(123) {
					t.Errorf("Expected user_id field to be preserved")
				}
			},
		},
		{
			name:        "JSON with error field",
			input:       `{"level": "error", "error": "Database connection failed", "code": 500}`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "ERROR" {
					t.Errorf("Expected level ERROR, got %s", entry.Level)
				}
				if entry.Message != "Error: Database connection failed" {
					t.Errorf("Expected 'Error: Database connection failed', got %s", entry.Message)
				}
			},
		},
		{
			name:        "Invalid JSON",
			input:       `{invalid json}`,
			expectError: true,
		},
		{
			name:        "Empty line",
			input:       "",
			expectError: true,
		},
		{
			name:        "JSON with nested objects",
			input:       `{"level": "info", "message": "Request processed", "request": {"method": "GET", "path": "/api/users"}}`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "INFO" {
					t.Errorf("Expected level INFO, got %s", entry.Level)
				}
				if entry.Message != "Request processed" {
					t.Errorf("Expected message 'Request processed', got %s", entry.Message)
				}
				if _, exists := entry.Fields["request"]; !exists {
					t.Error("Expected request field to be preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if entry == nil {
				t.Error("Expected entry but got nil")
				return
			}

			// Check that raw field is set
			if entry.Raw != tt.input {
				t.Errorf("Expected raw field to be %s, got %s", tt.input, entry.Raw)
			}

			// Run custom field checks
			if tt.checkFields != nil {
				tt.checkFields(t, entry)
			}
		})
	}
}

func TestJSONLogParser_CanParse(t *testing.T) {
	parser := NewJSONLogParser()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid JSON",
			input:    `{"level": "info", "message": "test"}`,
			expected: true,
		},
		{
			name:     "Invalid JSON",
			input:    `{level: info}`,
			expected: false,
		},
		{
			name:     "Plain text",
			input:    "This is not JSON",
			expected: false,
		},
		{
			name:     "Empty JSON object",
			input:    "{}",
			expected: true,
		},
		{
			name:     "JSON array",
			input:    `["not", "an", "object"]`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.CanParse(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestJSONLogParser_GetFormat(t *testing.T) {
	parser := NewJSONLogParser()
	if parser.GetFormat() != "JSON" {
		t.Errorf("Expected format 'JSON', got %s", parser.GetFormat())
	}
}

func TestJSONLogParser_extractTimestamp(t *testing.T) {
	parser := NewJSONLogParser()

	tests := []struct {
		name     string
		data     map[string]interface{}
		expected bool // whether a valid timestamp should be extracted
	}{
		{
			name: "String timestamp",
			data: map[string]interface{}{
				"timestamp": "2023-12-07T10:30:45Z",
			},
			expected: true,
		},
		{
			name: "Unix timestamp (seconds)",
			data: map[string]interface{}{
				"ts": float64(1701944445),
			},
			expected: true,
		},
		{
			name: "Unix timestamp (milliseconds)",
			data: map[string]interface{}{
				"time": float64(1701944445123),
			},
			expected: true,
		},
		{
			name: "No timestamp field",
			data: map[string]interface{}{
				"level":   "info",
				"message": "test",
			},
			expected: false,
		},
		{
			name: "Invalid timestamp format",
			data: map[string]interface{}{
				"timestamp": "invalid-time",
			},
			expected: true, // ParseTimestamp returns current time as fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractTimestamp(tt.data)

			if tt.expected && result.IsZero() {
				t.Error("Expected valid timestamp but got zero time")
			}
			if !tt.expected && !result.IsZero() {
				t.Error("Expected zero time but got valid timestamp")
			}
		})
	}
}

func TestJSONLogParser_extractLevel(t *testing.T) {
	parser := NewJSONLogParser()

	tests := []struct {
		name     string
		data     map[string]interface{}
		expected string
	}{
		{
			name: "Standard level field",
			data: map[string]interface{}{
				"level": "error",
			},
			expected: "ERROR",
		},
		{
			name: "Severity field",
			data: map[string]interface{}{
				"severity": "warn",
			},
			expected: "WARN",
		},
		{
			name: "Priority field",
			data: map[string]interface{}{
				"priority": "debug",
			},
			expected: "DEBUG",
		},
		{
			name: "No level field, extract from message",
			data: map[string]interface{}{
				"message": "This is an ERROR message",
			},
			expected: "ERROR",
		},
		{
			name: "No level information",
			data: map[string]interface{}{
				"data": "some data",
			},
			expected: "INFO",
		},
		{
			name: "Level normalization",
			data: map[string]interface{}{
				"level": "err",
			},
			expected: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractLevel(tt.data)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestJSONLogParser_extractMessage(t *testing.T) {
	parser := NewJSONLogParser()

	tests := []struct {
		name     string
		data     map[string]interface{}
		expected string
	}{
		{
			name: "Standard message field",
			data: map[string]interface{}{
				"message": "Test message",
			},
			expected: "Test message",
		},
		{
			name: "Msg field",
			data: map[string]interface{}{
				"msg": "Short message",
			},
			expected: "Short message",
		},
		{
			name: "Error field",
			data: map[string]interface{}{
				"error": "Something went wrong",
			},
			expected: "Error: Something went wrong",
		},
		{
			name: "Multiple message sources",
			data: map[string]interface{}{
				"error":  "Database error",
				"event":  "user_login",
				"action": "authenticate",
				"level":  "error",
			},
			expected: "Error: Database error | Event: user_login | Action: authenticate",
		},
		{
			name: "No message fields",
			data: map[string]interface{}{
				"level":     "info",
				"timestamp": "2023-12-07T10:30:45Z",
			},
			expected: "JSON log entry with 2 fields",
		},
		{
			name:     "Empty data",
			data:     map[string]interface{}{},
			expected: "JSON log entry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractMessage(tt.data)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
