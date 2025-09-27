package health

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/local-log-viewer/internal/logger"
)

// Status 健康状态
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// CheckResult 健康检查结果
type CheckResult struct {
	Name      string                 `json:"name"`
	Status    Status                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
}

// HealthCheck 健康检查接口
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) CheckResult
}

// HealthService 健康检查服务
type HealthService struct {
	checks    map[string]HealthCheck
	mu        sync.RWMutex
	startTime time.Time
}

// NewHealthService 创建健康检查服务
func NewHealthService() *HealthService {
	return &HealthService{
		checks:    make(map[string]HealthCheck),
		startTime: time.Now(),
	}
}

// RegisterCheck 注册健康检查
func (h *HealthService) RegisterCheck(check HealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[check.Name()] = check
}

// UnregisterCheck 注销健康检查
func (h *HealthService) UnregisterCheck(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.checks, name)
}

// CheckAll 执行所有健康检查
func (h *HealthService) CheckAll(ctx context.Context) map[string]CheckResult {
	h.mu.RLock()
	checks := make(map[string]HealthCheck, len(h.checks))
	for name, check := range h.checks {
		checks[name] = check
	}
	h.mu.RUnlock()

	results := make(map[string]CheckResult)
	var wg sync.WaitGroup

	for name, check := range checks {
		wg.Add(1)
		go func(name string, check HealthCheck) {
			defer wg.Done()
			start := time.Now()

			// 设置超时
			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			result := check.Check(checkCtx)
			result.Duration = time.Since(start)
			result.Timestamp = time.Now()

			results[name] = result
		}(name, check)
	}

	wg.Wait()
	return results
}

// GetOverallStatus 获取整体健康状态
func (h *HealthService) GetOverallStatus(ctx context.Context) (Status, map[string]CheckResult) {
	results := h.CheckAll(ctx)

	overallStatus := StatusHealthy
	healthyCount := 0
	unhealthyCount := 0
	degradedCount := 0

	for _, result := range results {
		switch result.Status {
		case StatusHealthy:
			healthyCount++
		case StatusUnhealthy:
			unhealthyCount++
		case StatusDegraded:
			degradedCount++
		}
	}

	// 确定整体状态
	if unhealthyCount > 0 {
		overallStatus = StatusUnhealthy
	} else if degradedCount > 0 {
		overallStatus = StatusDegraded
	}

	return overallStatus, results
}

// GetSystemInfo 获取系统信息
func (h *HealthService) GetSystemInfo() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"uptime":      time.Since(h.startTime).Seconds(),
		"timestamp":   time.Now().Unix(),
		"go_version":  runtime.Version(),
		"go_routines": runtime.NumGoroutine(),
		"memory": map[string]interface{}{
			"alloc":        m.Alloc,
			"total_alloc":  m.TotalAlloc,
			"sys":          m.Sys,
			"heap_alloc":   m.HeapAlloc,
			"heap_sys":     m.HeapSys,
			"heap_objects": m.HeapObjects,
			"gc_runs":      m.NumGC,
		},
		"system": map[string]interface{}{
			"os":        runtime.GOOS,
			"arch":      runtime.GOARCH,
			"cpu_count": runtime.NumCPU(),
		},
	}
}

// 内置健康检查实现

// FileSystemCheck 文件系统健康检查
type FileSystemCheck struct {
	paths []string
}

// NewFileSystemCheck 创建文件系统检查
func NewFileSystemCheck(paths []string) *FileSystemCheck {
	return &FileSystemCheck{paths: paths}
}

// Name 返回检查名称
func (f *FileSystemCheck) Name() string {
	return "filesystem"
}

// Check 执行文件系统检查
func (f *FileSystemCheck) Check(ctx context.Context) CheckResult {
	result := CheckResult{
		Name:      f.Name(),
		Status:    StatusHealthy,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	accessiblePaths := 0
	inaccessiblePaths := []string{}

	for _, path := range f.paths {
		if _, err := os.Stat(path); err != nil {
			inaccessiblePaths = append(inaccessiblePaths, path)
			logger.Warn("filesystem check failed for path",
				zap.String("path", path),
				zap.Error(err),
			)
		} else {
			accessiblePaths++
		}
	}

	result.Details["total_paths"] = len(f.paths)
	result.Details["accessible_paths"] = accessiblePaths
	result.Details["inaccessible_paths"] = inaccessiblePaths

	if len(inaccessiblePaths) == len(f.paths) {
		result.Status = StatusUnhealthy
		result.Message = "所有日志路径都不可访问"
	} else if len(inaccessiblePaths) > 0 {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("部分日志路径不可访问 (%d/%d)", len(inaccessiblePaths), len(f.paths))
	} else {
		result.Message = "所有日志路径都可访问"
	}

	return result
}

// MemoryCheck 内存使用检查
type MemoryCheck struct {
	maxMemoryMB int64
}

// NewMemoryCheck 创建内存检查
func NewMemoryCheck(maxMemoryMB int64) *MemoryCheck {
	return &MemoryCheck{maxMemoryMB: maxMemoryMB}
}

// Name 返回检查名称
func (m *MemoryCheck) Name() string {
	return "memory"
}

// Check 执行内存检查
func (m *MemoryCheck) Check(ctx context.Context) CheckResult {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	result := CheckResult{
		Name:      m.Name(),
		Status:    StatusHealthy,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	currentMemoryMB := int64(memStats.Alloc) / 1024 / 1024
	result.Details["current_memory_mb"] = currentMemoryMB
	result.Details["max_memory_mb"] = m.maxMemoryMB
	result.Details["memory_usage_percent"] = float64(currentMemoryMB) / float64(m.maxMemoryMB) * 100

	if currentMemoryMB > m.maxMemoryMB {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("内存使用超出限制: %dMB > %dMB", currentMemoryMB, m.maxMemoryMB)
	} else if float64(currentMemoryMB)/float64(m.maxMemoryMB) > 0.8 {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("内存使用较高: %dMB (%.1f%%)", currentMemoryMB, float64(currentMemoryMB)/float64(m.maxMemoryMB)*100)
	} else {
		result.Message = fmt.Sprintf("内存使用正常: %dMB (%.1f%%)", currentMemoryMB, float64(currentMemoryMB)/float64(m.maxMemoryMB)*100)
	}

	return result
}
