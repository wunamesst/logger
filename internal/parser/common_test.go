package parser

import (
	"testing"

	"github.com/local-log-viewer/internal/types"
)

func TestCommonLogParser_Parse(t *testing.T) {
	parser := NewCommonLogParser()

	tests := []struct {
		name        string
		input       string
		expectError bool
		checkFields func(t *testing.T, entry *types.LogEntry)
	}{
		{
			name:        "Apache Common Log Format",
			input:       `192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /index.html HTTP/1.1" 200 1234`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "INFO" {
					t.Errorf("Expected level INFO, got %s", entry.Level)
				}
				if entry.Fields["remote_addr"] != "192.168.1.1" {
					t.Errorf("Expected remote_addr 192.168.1.1, got %v", entry.Fields["remote_addr"])
				}
				if entry.Fields["status"] != "200" {
					t.Errorf("Expected status 200, got %v", entry.Fields["status"])
				}
				if entry.Fields["request"] != "GET /index.html HTTP/1.1" {
					t.Errorf("Expected request field, got %v", entry.Fields["request"])
				}
			},
		},
		{
			name:        "Apache Combined Log Format",
			input:       `192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /index.html HTTP/1.1" 200 1234 "http://example.com" "Mozilla/5.0"`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "INFO" {
					t.Errorf("Expected level INFO, got %s", entry.Level)
				}
				if entry.Fields["http_referer"] != "http://example.com" {
					t.Errorf("Expected referer, got %v", entry.Fields["http_referer"])
				}
				if entry.Fields["http_user_agent"] != "Mozilla/5.0" {
					t.Errorf("Expected user agent, got %v", entry.Fields["http_user_agent"])
				}
			},
		},
		{
			name:        "Apache log with 404 error",
			input:       `192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /missing.html HTTP/1.1" 404 0`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "WARN" {
					t.Errorf("Expected level WARN for 404, got %s", entry.Level)
				}
				if entry.Fields["status"] != "404" {
					t.Errorf("Expected status 404, got %v", entry.Fields["status"])
				}
			},
		},
		{
			name:        "Apache log with 500 error",
			input:       `192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "POST /api/data HTTP/1.1" 500 0`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "ERROR" {
					t.Errorf("Expected level ERROR for 500, got %s", entry.Level)
				}
				if entry.Fields["status"] != "500" {
					t.Errorf("Expected status 500, got %v", entry.Fields["status"])
				}
			},
		},
		{
			name:        "Nginx Access Log",
			input:       `192.168.1.1 - user [07/Dec/2023:10:30:45 +0000] "GET /api/users HTTP/1.1" 200 567 "http://example.com" "curl/7.68.0"`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "INFO" {
					t.Errorf("Expected level INFO, got %s", entry.Level)
				}
				if entry.Fields["remote_addr"] != "192.168.1.1" {
					t.Errorf("Expected remote_addr, got %v", entry.Fields["remote_addr"])
				}
				if entry.Fields["body_bytes_sent"] != "567" {
					t.Errorf("Expected body_bytes_sent, got %v", entry.Fields["body_bytes_sent"])
				}
			},
		},
		{
			name:        "Nginx Error Log",
			input:       `2023/12/07 10:30:45 [error] 1234#0: *1 connect() failed (111: Connection refused)`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "ERROR" {
					t.Errorf("Expected level ERROR, got %s", entry.Level)
				}
				if entry.Fields["error_level"] != "error" {
					t.Errorf("Expected error_level field, got %v", entry.Fields["error_level"])
				}
				if entry.Message != "*1 connect() failed (111: Connection refused)" {
					t.Errorf("Expected error message, got %s", entry.Message)
				}
			},
		},
		{
			name:        "Generic log with timestamp",
			input:       `2023-12-07 10:30:45 INFO Application started successfully`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "INFO" {
					t.Errorf("Expected level INFO, got %s", entry.Level)
				}
				if entry.Message != "INFO Application started successfully" {
					t.Errorf("Expected message, got %s", entry.Message)
				}
			},
		},
		{
			name:        "Generic log with ERROR level",
			input:       `2023-12-07T10:30:45Z ERROR Database connection failed`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "ERROR" {
					t.Errorf("Expected level ERROR, got %s", entry.Level)
				}
			},
		},
		{
			name:        "Plain text without timestamp",
			input:       `This is a simple log message with ERROR level`,
			expectError: false,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "ERROR" {
					t.Errorf("Expected level ERROR, got %s", entry.Level)
				}
				if entry.Message != "This is a simple log message with ERROR level" {
					t.Errorf("Expected full message, got %s", entry.Message)
				}
			},
		},
		{
			name:        "Empty line",
			input:       "",
			expectError: true,
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

			// Check that timestamp is set
			if entry.Timestamp.IsZero() {
				t.Error("Expected timestamp to be set")
			}

			// Run custom field checks
			if tt.checkFields != nil {
				tt.checkFields(t, entry)
			}
		})
	}
}

func TestCommonLogParser_CanParse(t *testing.T) {
	parser := NewCommonLogParser()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Apache Common Log",
			input:    `192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /index.html HTTP/1.1" 200 1234`,
			expected: true,
		},
		{
			name:     "Apache Combined Log",
			input:    `192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /index.html HTTP/1.1" 200 1234 "ref" "ua"`,
			expected: true,
		},
		{
			name:     "Nginx Access Log",
			input:    `192.168.1.1 - user [07/Dec/2023:10:30:45 +0000] "GET /api HTTP/1.1" 200 567 "ref" "ua"`,
			expected: true,
		},
		{
			name:     "Nginx Error Log",
			input:    `2023/12/07 10:30:45 [error] 1234#0: connection failed`,
			expected: true,
		},
		{
			name:     "Generic timestamp",
			input:    `2023-12-07 10:30:45 Some log message`,
			expected: true,
		},
		{
			name:     "Log with ERROR level",
			input:    `This is an ERROR message`,
			expected: true,
		},
		{
			name:     "Log with INFO level",
			input:    `This is an INFO message`,
			expected: true,
		},
		{
			name:     "Plain text (always parseable)",
			input:    `Just some plain text`,
			expected: true,
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

func TestCommonLogParser_GetFormat(t *testing.T) {
	parser := NewCommonLogParser()
	if parser.GetFormat() != "Common" {
		t.Errorf("Expected format 'Common', got %s", parser.GetFormat())
	}
}

func TestCommonLogParser_parseApacheCombined(t *testing.T) {
	parser := NewCommonLogParser()
	entry := &types.LogEntry{
		Fields: make(map[string]interface{}),
	}

	// Test valid Apache Combined format
	line := `192.168.1.1 - - [07/Dec/2023:10:30:45 +0000] "GET /index.html HTTP/1.1" 200 1234 "http://example.com" "Mozilla/5.0"`
	result := parser.parseApacheCombined(line, entry)

	if !result {
		t.Error("Expected parseApacheCombined to return true")
	}

	expectedFields := map[string]string{
		"remote_addr":     "192.168.1.1",
		"request":         "GET /index.html HTTP/1.1",
		"status":          "200",
		"body_bytes_sent": "1234",
		"http_referer":    "http://example.com",
		"http_user_agent": "Mozilla/5.0",
	}

	for key, expected := range expectedFields {
		if entry.Fields[key] != expected {
			t.Errorf("Expected %s to be %s, got %v", key, expected, entry.Fields[key])
		}
	}

	if entry.Level != "INFO" {
		t.Errorf("Expected level INFO, got %s", entry.Level)
	}
}

func TestCommonLogParser_parseNginxError(t *testing.T) {
	parser := NewCommonLogParser()
	entry := &types.LogEntry{
		Fields: make(map[string]interface{}),
	}

	tests := []struct {
		name          string
		input         string
		expectedLevel string
		shouldMatch   bool
	}{
		{
			name:          "Error level",
			input:         `2023/12/07 10:30:45 [error] 1234#0: connection failed`,
			expectedLevel: "ERROR",
			shouldMatch:   true,
		},
		{
			name:          "Warning level",
			input:         `2023/12/07 10:30:45 [warn] 1234#0: deprecated feature used`,
			expectedLevel: "WARN",
			shouldMatch:   true,
		},
		{
			name:          "Critical level",
			input:         `2023/12/07 10:30:45 [crit] 1234#0: system critical error`,
			expectedLevel: "FATAL",
			shouldMatch:   true,
		},
		{
			name:          "Info level",
			input:         `2023/12/07 10:30:45 [info] 1234#0: server started`,
			expectedLevel: "INFO",
			shouldMatch:   true,
		},
		{
			name:          "Debug level",
			input:         `2023/12/07 10:30:45 [debug] 1234#0: debug information`,
			expectedLevel: "DEBUG",
			shouldMatch:   true,
		},
		{
			name:          "Invalid format",
			input:         `Not a nginx error log`,
			expectedLevel: "",
			shouldMatch:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset entry
			entry.Level = ""
			entry.Message = ""
			entry.Fields = make(map[string]interface{})

			result := parser.parseNginxError(tt.input, entry)

			if result != tt.shouldMatch {
				t.Errorf("Expected parseNginxError to return %v, got %v", tt.shouldMatch, result)
			}

			if tt.shouldMatch && entry.Level != tt.expectedLevel {
				t.Errorf("Expected level %s, got %s", tt.expectedLevel, entry.Level)
			}
		})
	}
}

func TestCommonLogParser_parseGeneric(t *testing.T) {
	parser := NewCommonLogParser()

	tests := []struct {
		name        string
		input       string
		shouldMatch bool
		checkFields func(t *testing.T, entry *types.LogEntry)
	}{
		{
			name:        "ISO timestamp",
			input:       `2023-12-07T10:30:45Z Application started`,
			shouldMatch: true,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Message != "Application started" {
					t.Errorf("Expected message 'Application started', got %s", entry.Message)
				}
			},
		},
		{
			name:        "Standard datetime",
			input:       `2023-12-07 10:30:45 ERROR Database connection failed`,
			shouldMatch: true,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Level != "ERROR" {
					t.Errorf("Expected level ERROR, got %s", entry.Level)
				}
			},
		},
		{
			name:        "With IP address",
			input:       `2023-12-07 10:30:45 Connection from 192.168.1.1`,
			shouldMatch: true,
			checkFields: func(t *testing.T, entry *types.LogEntry) {
				if entry.Fields["ip_address"] != "192.168.1.1" {
					t.Errorf("Expected IP address field, got %v", entry.Fields["ip_address"])
				}
			},
		},
		{
			name:        "No timestamp",
			input:       `Just a regular message`,
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &types.LogEntry{
				Fields: make(map[string]interface{}),
			}

			result := parser.parseGeneric(tt.input, entry)

			if result != tt.shouldMatch {
				t.Errorf("Expected parseGeneric to return %v, got %v", tt.shouldMatch, result)
			}

			if tt.shouldMatch && tt.checkFields != nil {
				tt.checkFields(t, entry)
			}
		})
	}
}
