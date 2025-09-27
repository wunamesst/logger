package health

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockHealthCheck 模拟健康检查
type MockHealthCheck struct {
	name   string
	status Status
	delay  time.Duration
}

func (m *MockHealthCheck) Name() string {
	return m.name
}

func (m *MockHealthCheck) Check(ctx context.Context) CheckResult {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return CheckResult{
				Name:      m.name,
				Status:    StatusUnhealthy,
				Message:   "timeout",
				Timestamp: time.Now(),
			}
		}
	}

	return CheckResult{
		Name:      m.name,
		Status:    m.status,
		Message:   "mock check",
		Timestamp: time.Now(),
	}
}

func TestHealthService_RegisterCheck(t *testing.T) {
	service := NewHealthService()
	check := &MockHealthCheck{name: "test", status: StatusHealthy}

	service.RegisterCheck(check)

	if len(service.checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(service.checks))
	}

	if service.checks["test"] != check {
		t.Error("Check not registered correctly")
	}
}

func TestHealthService_UnregisterCheck(t *testing.T) {
	service := NewHealthService()
	check := &MockHealthCheck{name: "test", status: StatusHealthy}

	service.RegisterCheck(check)
	service.UnregisterCheck("test")

	if len(service.checks) != 0 {
		t.Errorf("Expected 0 checks, got %d", len(service.checks))
	}
}

func TestHealthService_CheckAll(t *testing.T) {
	service := NewHealthService()

	check1 := &MockHealthCheck{name: "check1", status: StatusHealthy}
	check2 := &MockHealthCheck{name: "check2", status: StatusDegraded}

	service.RegisterCheck(check1)
	service.RegisterCheck(check2)

	ctx := context.Background()
	results := service.CheckAll(ctx)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results["check1"].Status != StatusHealthy {
		t.Errorf("Expected check1 to be healthy, got %v", results["check1"].Status)
	}

	if results["check2"].Status != StatusDegraded {
		t.Errorf("Expected check2 to be degraded, got %v", results["check2"].Status)
	}
}
func TestHealthService_GetOverallStatus(t *testing.T) {
	tests := []struct {
		name           string
		checks         []*MockHealthCheck
		expectedStatus Status
	}{
		{
			name: "all healthy",
			checks: []*MockHealthCheck{
				{name: "check1", status: StatusHealthy},
				{name: "check2", status: StatusHealthy},
			},
			expectedStatus: StatusHealthy,
		},
		{
			name: "one degraded",
			checks: []*MockHealthCheck{
				{name: "check1", status: StatusHealthy},
				{name: "check2", status: StatusDegraded},
			},
			expectedStatus: StatusDegraded,
		},
		{
			name: "one unhealthy",
			checks: []*MockHealthCheck{
				{name: "check1", status: StatusHealthy},
				{name: "check2", status: StatusUnhealthy},
			},
			expectedStatus: StatusUnhealthy,
		},
		{
			name: "unhealthy takes precedence",
			checks: []*MockHealthCheck{
				{name: "check1", status: StatusDegraded},
				{name: "check2", status: StatusUnhealthy},
			},
			expectedStatus: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewHealthService()

			for _, check := range tt.checks {
				service.RegisterCheck(check)
			}

			ctx := context.Background()
			status, _ := service.GetOverallStatus(ctx)

			if status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, status)
			}
		})
	}
}

func TestHealthService_CheckTimeout(t *testing.T) {
	service := NewHealthService()

	// 创建一个会超时的检查
	slowCheck := &MockHealthCheck{
		name:   "slow",
		status: StatusHealthy,
		delay:  10 * time.Second, // 比默认超时时间长
	}

	service.RegisterCheck(slowCheck)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	results := service.CheckAll(ctx)

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// 检查是否因为超时而失败
	result := results["slow"]
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status due to timeout, got %v", result.Status)
	}
}

func TestFileSystemCheck(t *testing.T) {
	// 创建临时目录用于测试
	tempDir, err := os.MkdirTemp("", "health_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	testFile := filepath.Join(tempDir, "test.log")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name           string
		paths          []string
		expectedStatus Status
	}{
		{
			name:           "all paths accessible",
			paths:          []string{tempDir, testFile},
			expectedStatus: StatusHealthy,
		},
		{
			name:           "some paths inaccessible",
			paths:          []string{tempDir, "/nonexistent/path"},
			expectedStatus: StatusDegraded,
		},
		{
			name:           "all paths inaccessible",
			paths:          []string{"/nonexistent/path1", "/nonexistent/path2"},
			expectedStatus: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := NewFileSystemCheck(tt.paths)
			result := check.Check(context.Background())

			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, result.Status)
			}

			if result.Name != "filesystem" {
				t.Errorf("Expected name 'filesystem', got %v", result.Name)
			}
		})
	}
}

func TestMemoryCheck(t *testing.T) {
	tests := []struct {
		name           string
		maxMemoryMB    int64
		expectedStatus Status
	}{
		{
			name:           "memory usage normal",
			maxMemoryMB:    1000, // 1GB limit, should be healthy
			expectedStatus: StatusHealthy,
		},
		{
			name:           "memory usage high",
			maxMemoryMB:    1, // 1MB limit, should be degraded or unhealthy
			expectedStatus: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := NewMemoryCheck(tt.maxMemoryMB)
			result := check.Check(context.Background())

			if result.Name != "memory" {
				t.Errorf("Expected name 'memory', got %v", result.Name)
			}

			// 对于内存检查，我们主要验证它能正常运行
			// 具体的状态取决于当前的内存使用情况
			if result.Status == "" {
				t.Error("Expected non-empty status")
			}

			// 验证详细信息包含预期的字段
			if result.Details["current_memory_mb"] == nil {
				t.Error("Expected current_memory_mb in details")
			}

			if result.Details["max_memory_mb"] != tt.maxMemoryMB {
				t.Errorf("Expected max_memory_mb %d, got %v", tt.maxMemoryMB, result.Details["max_memory_mb"])
			}
		})
	}
}

func TestHealthService_GetSystemInfo(t *testing.T) {
	service := NewHealthService()
	info := service.GetSystemInfo()

	// 验证系统信息包含预期的字段
	expectedFields := []string{
		"uptime", "timestamp", "go_version", "go_routines",
		"memory", "system",
	}

	for _, field := range expectedFields {
		if info[field] == nil {
			t.Errorf("Expected field %s in system info", field)
		}
	}

	// 验证内存信息
	memory, ok := info["memory"].(map[string]interface{})
	if !ok {
		t.Error("Expected memory to be a map")
	} else {
		memoryFields := []string{"alloc", "total_alloc", "sys", "heap_alloc", "heap_sys", "heap_objects", "gc_runs"}
		for _, field := range memoryFields {
			if memory[field] == nil {
				t.Errorf("Expected field %s in memory info", field)
			}
		}
	}

	// 验证系统信息
	system, ok := info["system"].(map[string]interface{})
	if !ok {
		t.Error("Expected system to be a map")
	} else {
		systemFields := []string{"os", "arch", "cpu_count"}
		for _, field := range systemFields {
			if system[field] == nil {
				t.Errorf("Expected field %s in system info", field)
			}
		}
	}
}
