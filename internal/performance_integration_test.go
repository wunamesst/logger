package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/local-log-viewer/internal/cache"
	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/interfaces"
	"github.com/local-log-viewer/internal/manager"
	"github.com/local-log-viewer/internal/monitor"
	"github.com/local-log-viewer/internal/parser"
	"github.com/local-log-viewer/internal/pool"
	"github.com/local-log-viewer/internal/search"
	"github.com/local-log-viewer/internal/types"
	"github.com/local-log-viewer/internal/watcher"
)

// TestPerformanceIntegration 测试性能优化的集成效果
func TestPerformanceIntegration(t *testing.T) {
	// 创建临时目录和测试文件
	tmpDir, err := os.MkdirTemp("", "perf_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建大型测试日志文件
	logFile := filepath.Join(tmpDir, "large.log")
	if err := createLargeLogFile(logFile, 10000); err != nil {
		t.Fatal(err)
	}

	// 配置
	cfg := &config.Config{
		Server: config.ServerConfig{
			LogPaths:    []string{tmpDir},
			MaxFileSize: 100 * 1024 * 1024, // 100MB
			CacheSize:   1000,
		},
	}

	// 创建组件
	logCache := cache.NewMemoryCacheWithOptions(1000, time.Hour, 50, 0.8) // 50MB限制
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		t.Fatal(err)
	}

	// 创建日志管理器（包含所有性能优化）
	logManager := manager.NewLogManager(cfg, fileWatcher, logCache)

	// 启动管理器
	if err := logManager.Start(); err != nil {
		t.Fatal(err)
	}
	defer logManager.Stop()

	// 测试1: 大文件读取性能
	t.Run("LargeFileReading", func(t *testing.T) {
		start := time.Now()

		// 读取多个页面
		for i := 0; i < 10; i++ {
			content, err := logManager.ReadLogFile(logFile, int64(i*100), 100)
			if err != nil {
				t.Errorf("读取文件失败: %v", err)
				continue
			}

			if len(content.Entries) == 0 {
				break
			}
		}

		duration := time.Since(start)
		t.Logf("大文件读取耗时: %v", duration)

		// 性能要求：10次读取应该在1秒内完成
		if duration > time.Second {
			t.Errorf("大文件读取性能不达标: %v", duration)
		}
	})

	// 测试2: 缓存效果
	t.Run("CacheEffectiveness", func(t *testing.T) {
		// 第一次读取（冷缓存）
		start := time.Now()
		_, err := logManager.ReadLogFile(logFile, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		coldDuration := time.Since(start)

		// 第二次读取（热缓存）
		start = time.Now()
		_, err = logManager.ReadLogFile(logFile, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		hotDuration := time.Since(start)

		t.Logf("冷缓存读取: %v, 热缓存读取: %v", coldDuration, hotDuration)

		// 热缓存应该明显更快
		if hotDuration >= coldDuration {
			t.Errorf("缓存没有提升性能: 冷=%v, 热=%v", coldDuration, hotDuration)
		}
	})

	// 测试3: 搜索性能
	t.Run("SearchPerformance", func(t *testing.T) {
		// 创建搜索引擎
		parsers := make(map[string]interfaces.LogParser)
		parsers["json"] = parser.NewJSONLogParser()
		parsers["common"] = parser.NewCommonLogParser()

		searchEngine := search.NewSearchEngine(parsers, logCache)

		query := types.SearchQuery{
			Path:   logFile,
			Query:  "ERROR",
			Limit:  100,
			Offset: 0,
		}

		// 第一次搜索
		start := time.Now()
		result1, err := searchEngine.Search(query)
		if err != nil {
			t.Fatal(err)
		}
		firstDuration := time.Since(start)

		// 第二次搜索（应该使用缓存）
		start = time.Now()
		result2, err := searchEngine.Search(query)
		if err != nil {
			t.Fatal(err)
		}
		secondDuration := time.Since(start)

		t.Logf("第一次搜索: %v, 第二次搜索: %v", firstDuration, secondDuration)
		t.Logf("搜索结果: %d 条", result1.TotalCount)

		// 验证结果一致性
		if result1.TotalCount != result2.TotalCount {
			t.Errorf("搜索结果不一致: %d vs %d", result1.TotalCount, result2.TotalCount)
		}

		// 第二次搜索应该更快（缓存效果）
		if secondDuration >= firstDuration {
			t.Logf("警告: 搜索缓存可能没有生效")
		}
	})

	// 测试4: 内存监控
	t.Run("MemoryMonitoring", func(t *testing.T) {
		// 获取性能统计
		if perfManager, ok := logManager.(interface {
			GetPerformanceStats() map[string]interface{}
		}); ok {
			stats := perfManager.GetPerformanceStats()

			t.Logf("性能统计: %+v", stats)

			// 检查内存统计
			if memStats, ok := stats["memory"]; ok {
				if ms, ok := memStats.(monitor.MemoryStats); ok {
					t.Logf("内存使用: %.2f%%, 分配: %d bytes", ms.UsagePercent, ms.Alloc)

					// 内存使用应该在合理范围内
					if ms.UsagePercent > 90 {
						t.Errorf("内存使用过高: %.2f%%", ms.UsagePercent)
					}
				}
			}
		}
	})

	// 测试5: 并发性能
	t.Run("ConcurrentPerformance", func(t *testing.T) {
		const numGoroutines = 10
		const numOperations = 50

		start := time.Now()

		// 创建通道收集结果
		results := make(chan error, numGoroutines)

		// 启动并发操作
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				var err error
				for j := 0; j < numOperations; j++ {
					// 交替进行读取和搜索操作
					if j%2 == 0 {
						_, err = logManager.ReadLogFile(logFile, int64(j*10), 50)
					} else {
						query := types.SearchQuery{
							Path:   logFile,
							Query:  fmt.Sprintf("line_%d", j),
							Limit:  10,
							Offset: 0,
						}
						_, err = logManager.SearchLogs(query)
					}

					if err != nil {
						break
					}
				}
				results <- err
			}(i)
		}

		// 等待所有goroutine完成
		var errors []error
		for i := 0; i < numGoroutines; i++ {
			if err := <-results; err != nil {
				errors = append(errors, err)
			}
		}

		duration := time.Since(start)
		totalOps := numGoroutines * numOperations

		t.Logf("并发测试完成: %d 个操作，耗时 %v，平均 %v/操作",
			totalOps, duration, duration/time.Duration(totalOps))

		if len(errors) > 0 {
			t.Errorf("并发测试中有 %d 个错误: %v", len(errors), errors[0])
		}

		// 性能要求：平均每操作不超过10ms
		avgDuration := duration / time.Duration(totalOps)
		if avgDuration > 10*time.Millisecond {
			t.Errorf("并发性能不达标: 平均 %v/操作", avgDuration)
		}
	})

	// 测试6: 资源池效果
	t.Run("ResourcePoolEffectiveness", func(t *testing.T) {
		// 创建文件池
		poolConfig := pool.PoolConfig{
			InitialSize: 2,
			MaxSize:     5,
			MaxIdleTime: time.Minute,
			MaxLifetime: time.Hour,
		}
		filePool := pool.NewFilePool(poolConfig)
		defer filePool.Close()

		// 测试资源复用
		start := time.Now()
		for i := 0; i < 100; i++ {
			resource, err := filePool.GetFileResource(logFile)
			if err != nil {
				t.Errorf("获取文件资源失败: %v", err)
				continue
			}

			// 模拟使用
			reader := resource.GetReader()
			if reader == nil {
				t.Error("获取读取器失败")
			}

			// 归还资源
			if err := filePool.PutFileResource(logFile, resource); err != nil {
				t.Errorf("归还文件资源失败: %v", err)
			}
		}
		duration := time.Since(start)

		t.Logf("资源池测试: 100次操作耗时 %v", duration)

		// 获取统计信息
		stats := filePool.GetStats()
		t.Logf("文件池统计: %+v", stats)

		// 验证资源复用
		if len(stats) == 0 {
			t.Error("没有文件池统计信息")
		}
	})
}

// createLargeLogFile 创建大型测试日志文件
func createLargeLogFile(path string, lines int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for i := 0; i < lines; i++ {
		level := "INFO"
		if i%10 == 0 {
			level = "ERROR"
		} else if i%5 == 0 {
			level = "WARN"
		}

		timestamp := time.Now().Add(-time.Duration(lines-i) * time.Second)
		line := fmt.Sprintf("%s [%s] line_%d: This is test log message number %d with some additional content to make it longer\n",
			timestamp.Format("2006-01-02T15:04:05Z07:00"), level, i, i)

		if _, err := file.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}

// BenchmarkPerformanceOptimizations 性能优化基准测试
func BenchmarkPerformanceOptimizations(b *testing.B) {
	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "bench_*.log")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// 写入测试数据
	for i := 0; i < 1000; i++ {
		line := fmt.Sprintf("2025-09-09T14:36:15Z [INFO] benchmark line %d\n", i)
		tmpFile.WriteString(line)
	}
	tmpFile.Close()

	// 创建优化的组件
	cfg := &config.Config{
		Server: config.ServerConfig{
			LogPaths:    []string{filepath.Dir(tmpFile.Name())},
			MaxFileSize: 10 * 1024 * 1024,
			CacheSize:   100,
		},
	}

	logCache := cache.NewMemoryCache(100, time.Hour)
	fileWatcher, _ := watcher.NewFileWatcher()
	logManager := manager.NewLogManager(cfg, fileWatcher, logCache)

	logManager.Start()
	defer logManager.Stop()

	b.ResetTimer()

	// 基准测试读取操作
	b.Run("OptimizedRead", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := logManager.ReadLogFile(tmpFile.Name(), int64(i%10)*10, 50)
			if err != nil {
				b.Error(err)
			}
		}
	})

	// 基准测试搜索操作
	b.Run("OptimizedSearch", func(b *testing.B) {
		query := types.SearchQuery{
			Path:   tmpFile.Name(),
			Query:  "INFO",
			Limit:  10,
			Offset: 0,
		}

		for i := 0; i < b.N; i++ {
			_, err := logManager.SearchLogs(query)
			if err != nil {
				b.Error(err)
			}
		}
	})
}
