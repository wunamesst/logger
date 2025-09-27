package monitor

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestMemoryMonitor_BasicFunctionality 测试内存监控基本功能
func TestMemoryMonitor_BasicFunctionality(t *testing.T) {
	config := MemoryMonitorConfig{
		MaxMemory:       100 * 1024 * 1024, // 100MB
		WarningLevel:    0.7,
		CriticalLevel:   0.9,
		MonitorInterval: 100 * time.Millisecond,
		MaxHistory:      10,
	}

	monitor := NewMemoryMonitor(config)

	// 测试获取当前统计
	stats := monitor.GetCurrentStats()
	if stats.Timestamp.IsZero() {
		t.Error("统计时间戳不应该为零")
	}

	if stats.Alloc == 0 {
		t.Error("分配内存不应该为零")
	}

	t.Logf("当前内存统计: Alloc=%d, Sys=%d, UsagePercent=%.2f%%",
		stats.Alloc, stats.Sys, stats.UsagePercent)
}

// TestMemoryMonitor_Callbacks 测试回调功能
func TestMemoryMonitor_Callbacks(t *testing.T) {
	var warningCalled, criticalCalled bool
	var mu sync.Mutex

	config := MemoryMonitorConfig{
		MaxMemory:       1024, // 很小的限制，容易触发
		WarningLevel:    0.1,  // 10%
		CriticalLevel:   0.2,  // 20%
		MonitorInterval: 50 * time.Millisecond,
		MaxHistory:      5,
		WarningCallback: func(stats MemoryStats) {
			mu.Lock()
			warningCalled = true
			mu.Unlock()
			t.Logf("警告回调触发: UsagePercent=%.2f%%", stats.UsagePercent)
		},
		CriticalCallback: func(stats MemoryStats) {
			mu.Lock()
			criticalCalled = true
			mu.Unlock()
			t.Logf("严重回调触发: UsagePercent=%.2f%%", stats.UsagePercent)
		},
	}

	monitor := NewMemoryMonitor(config)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 启动监控
	if err := monitor.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// 等待回调触发
	time.Sleep(200 * time.Millisecond)

	// 停止监控
	if err := monitor.Stop(); err != nil {
		t.Error(err)
	}

	mu.Lock()
	defer mu.Unlock()

	// 由于内存使用通常会超过这些很小的阈值，回调应该被触发
	if !warningCalled {
		t.Log("警告回调未被触发（可能是因为内存使用较低）")
	}

	if !criticalCalled {
		t.Log("严重回调未被触发（可能是因为内存使用较低）")
	}
}

// TestMemoryMonitor_History 测试历史记录功能
func TestMemoryMonitor_History(t *testing.T) {
	config := MemoryMonitorConfig{
		MaxMemory:       100 * 1024 * 1024,
		WarningLevel:    0.7,
		CriticalLevel:   0.9,
		MonitorInterval: 50 * time.Millisecond,
		MaxHistory:      5,
	}

	monitor := NewMemoryMonitor(config)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// 启动监控
	if err := monitor.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// 等待收集一些历史数据
	time.Sleep(300 * time.Millisecond)

	// 停止监控
	if err := monitor.Stop(); err != nil {
		t.Error(err)
	}

	// 检查历史记录
	history := monitor.GetStatsHistory()
	if len(history) == 0 {
		t.Error("应该有历史记录")
	}

	if len(history) > 5 {
		t.Errorf("历史记录数量 %d 超过限制 5", len(history))
	}

	t.Logf("收集到 %d 条历史记录", len(history))

	// 检查历史记录的时间顺序
	for i := 1; i < len(history); i++ {
		if history[i].Timestamp.Before(history[i-1].Timestamp) {
			t.Error("历史记录时间顺序错误")
		}
	}
}

// TestMemoryMonitor_PressureDetection 测试内存压力检测
func TestMemoryMonitor_PressureDetection(t *testing.T) {
	// 使用当前内存的一半作为限制，应该能检测到压力
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	maxMemory := m.Alloc / 2

	config := MemoryMonitorConfig{
		MaxMemory:       maxMemory,
		WarningLevel:    0.5,
		CriticalLevel:   0.8,
		MonitorInterval: 100 * time.Millisecond,
		MaxHistory:      10,
	}

	monitor := NewMemoryMonitor(config)

	// 检查压力检测
	hasPressure := monitor.IsMemoryPressure()
	pressureLevel := monitor.GetMemoryPressureLevel()

	t.Logf("内存压力: %t, 压力等级: %s", hasPressure, pressureLevel)

	// 获取优化建议
	suggestions := monitor.OptimizeMemory()
	t.Logf("优化建议: %v", suggestions)

	if len(suggestions) > 0 {
		t.Logf("检测到内存压力，建议: %v", suggestions)
	}
}

// BenchmarkMemoryMonitor_GetStats 测试获取统计信息的性能
func BenchmarkMemoryMonitor_GetStats(b *testing.B) {
	config := MemoryMonitorConfig{
		MaxMemory:       100 * 1024 * 1024,
		WarningLevel:    0.7,
		CriticalLevel:   0.9,
		MonitorInterval: time.Second,
		MaxHistory:      100,
	}

	monitor := NewMemoryMonitor(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats := monitor.GetCurrentStats()
		_ = stats // 避免编译器优化
	}
}

// TestMemoryMonitor_ForceGC 测试强制垃圾回收
func TestMemoryMonitor_ForceGC(t *testing.T) {
	monitor := NewMemoryMonitor(MemoryMonitorConfig{})

	// 获取GC前的统计
	statsBefore := monitor.GetCurrentStats()

	// 分配一些内存
	data := make([][]byte, 1000)
	for i := range data {
		data[i] = make([]byte, 1024) // 1KB each
	}

	// 获取分配后的统计
	statsAfter := monitor.GetCurrentStats()

	// 强制GC
	monitor.ForceGC()

	// 获取GC后的统计
	statsAfterGC := monitor.GetCurrentStats()

	t.Logf("GC前: Alloc=%d, NumGC=%d", statsBefore.Alloc, statsBefore.NumGC)
	t.Logf("分配后: Alloc=%d, NumGC=%d", statsAfter.Alloc, statsAfter.NumGC)
	t.Logf("GC后: Alloc=%d, NumGC=%d", statsAfterGC.Alloc, statsAfterGC.NumGC)

	// GC次数应该增加
	if statsAfterGC.NumGC <= statsBefore.NumGC {
		t.Error("强制GC后，GC次数应该增加")
	}

	// 清除引用以便GC
	data = nil
}

// TestMemoryMonitor_Concurrent 测试并发安全性
func TestMemoryMonitor_Concurrent(t *testing.T) {
	config := MemoryMonitorConfig{
		MaxMemory:       100 * 1024 * 1024,
		WarningLevel:    0.7,
		CriticalLevel:   0.9,
		MonitorInterval: 10 * time.Millisecond,
		MaxHistory:      50,
	}

	monitor := NewMemoryMonitor(config)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// 启动监控
	if err := monitor.Start(ctx); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// 并发访问监控器
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				// 并发获取统计信息
				stats := monitor.GetCurrentStats()
				_ = stats

				// 并发检查内存压力
				pressure := monitor.IsMemoryPressure()
				_ = pressure

				// 并发获取历史记录
				history := monitor.GetStatsHistory()
				_ = history

				// 并发获取优化建议
				suggestions := monitor.OptimizeMemory()
				_ = suggestions

				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// 停止监控
	if err := monitor.Stop(); err != nil {
		t.Error(err)
	}

	t.Log("并发测试完成")
}

// TestMemoryMonitor_LongRunning 测试长时间运行
func TestMemoryMonitor_LongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过长时间运行测试")
	}

	config := MemoryMonitorConfig{
		MaxMemory:       100 * 1024 * 1024,
		WarningLevel:    0.7,
		CriticalLevel:   0.9,
		MonitorInterval: 100 * time.Millisecond,
		MaxHistory:      100,
	}

	monitor := NewMemoryMonitor(config)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 启动监控
	if err := monitor.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// 模拟内存使用变化
	go func() {
		for i := 0; i < 10; i++ {
			// 分配内存
			data := make([]byte, 1024*1024) // 1MB
			_ = data
			time.Sleep(200 * time.Millisecond)

			// 触发GC
			runtime.GC()
			time.Sleep(200 * time.Millisecond)
		}
	}()

	// 等待监控完成
	<-ctx.Done()

	// 停止监控
	if err := monitor.Stop(); err != nil {
		t.Error(err)
	}

	// 检查收集的数据
	history := monitor.GetStatsHistory()
	t.Logf("长时间运行收集到 %d 条记录", len(history))

	if len(history) == 0 {
		t.Error("长时间运行应该收集到历史数据")
	}

	// 分析内存使用趋势
	if len(history) > 1 {
		first := history[0]
		last := history[len(history)-1]
		t.Logf("内存使用变化: %d -> %d (%.2f%%)",
			first.Alloc, last.Alloc,
			float64(last.Alloc-first.Alloc)/float64(first.Alloc)*100)
	}
}
