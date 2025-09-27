package cache

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/local-log-viewer/internal/interfaces"
	"github.com/local-log-viewer/internal/types"
)

// BenchmarkMemoryCache_Set 测试缓存设置性能
func BenchmarkMemoryCache_Set(b *testing.B) {
	cache := NewMemoryCache(10000, time.Hour)
	defer cache.(*MemoryCache).Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i)
			value := fmt.Sprintf("value_%d", i)
			cache.Set(key, value)
			i++
		}
	})
}

// BenchmarkMemoryCache_Get 测试缓存获取性能
func BenchmarkMemoryCache_Get(b *testing.B) {
	cache := NewMemoryCache(10000, time.Hour)
	defer cache.(*MemoryCache).Stop()

	// 预填充缓存
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		cache.Set(key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i%1000)
			cache.Get(key)
			i++
		}
	})
}

// BenchmarkMemoryCache_Mixed 测试混合操作性能
func BenchmarkMemoryCache_Mixed(b *testing.B) {
	cache := NewMemoryCache(10000, time.Hour)
	defer cache.(*MemoryCache).Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i)
			if i%3 == 0 {
				// 33% 写操作
				value := fmt.Sprintf("value_%d", i)
				cache.Set(key, value)
			} else {
				// 67% 读操作
				cache.Get(key)
			}
			i++
		}
	})
}

// TestMemoryCache_Concurrency 测试并发安全性
func TestMemoryCache_Concurrency(t *testing.T) {
	cache := NewMemoryCache(1000, time.Hour)
	defer cache.(*MemoryCache).Stop()

	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 1000

	// 启动多个goroutine进行并发操作
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)

				// 设置值
				cache.Set(key, value)

				// 获取值
				if got, found := cache.Get(key); found {
					if got != value {
						t.Errorf("期望值 %s，实际值 %s", value, got)
					}
				}

				// 删除值
				if j%10 == 0 {
					cache.Delete(key)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestMemoryCache_MemoryUsage 测试内存使用情况
func TestMemoryCache_MemoryUsage(t *testing.T) {
	// 获取初始内存使用
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	initialAlloc := m1.Alloc

	cache := NewMemoryCache(10000, time.Hour)
	defer cache.(*MemoryCache).Stop()

	// 添加大量数据
	dataSize := 1000
	for i := 0; i < dataSize; i++ {
		key := fmt.Sprintf("key_%d", i)
		// 创建较大的值（1KB）
		value := make([]byte, 1024)
		for j := range value {
			value[j] = byte(i % 256)
		}
		cache.Set(key, value)
	}

	// 获取使用后的内存
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	afterAlloc := m2.Alloc

	memoryUsed := afterAlloc - initialAlloc
	t.Logf("初始内存: %d bytes", initialAlloc)
	t.Logf("使用后内存: %d bytes", afterAlloc)
	t.Logf("内存增长: %d bytes", memoryUsed)

	// 检查缓存统计
	if advancedCache, ok := cache.(interfaces.AdvancedLogCache); ok {
		stats := advancedCache.GetStats()
		t.Logf("缓存统计: %+v", stats)
	}

	// 清空缓存
	cache.Clear()

	// 强制GC
	runtime.GC()
	runtime.GC()

	// 检查内存是否释放
	var m3 runtime.MemStats
	runtime.ReadMemStats(&m3)
	finalAlloc := m3.Alloc

	t.Logf("清空后内存: %d bytes", finalAlloc)

	// 内存应该有所减少（虽然不一定完全回到初始状态）
	if finalAlloc > afterAlloc {
		t.Errorf("清空缓存后内存没有减少")
	}
}

// TestMemoryCache_LRUEviction 测试LRU淘汰策略
func TestMemoryCache_LRUEviction(t *testing.T) {
	cache := NewMemoryCache(3, time.Hour) // 小缓存用于测试淘汰
	defer cache.(*MemoryCache).Stop()

	// 添加3个项目
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// 访问key1，使其成为最近使用的
	cache.Get("key1")

	// 添加第4个项目，应该淘汰key2或key3
	cache.Set("key4", "value4")

	// key1应该仍然存在（因为最近被访问）
	if _, found := cache.Get("key1"); !found {
		t.Error("key1应该仍然存在")
	}

	// key4应该存在（刚刚添加）
	if _, found := cache.Get("key4"); !found {
		t.Error("key4应该存在")
	}

	// 检查总项目数不超过限制
	if advancedCache, ok := cache.(interfaces.AdvancedLogCache); ok {
		if stats, ok := advancedCache.GetStats().(CacheStats); ok {
			if stats.ItemCount > 3 {
				t.Errorf("缓存项目数 %d 超过限制 3", stats.ItemCount)
			}
		}
	}
}

// TestMemoryCache_TTLExpiration 测试TTL过期
func TestMemoryCache_TTLExpiration(t *testing.T) {
	cache := NewMemoryCache(10, 100*time.Millisecond) // 100ms TTL
	defer cache.(*MemoryCache).Stop()

	// 设置一个值
	cache.Set("key1", "value1")

	// 立即检查应该存在
	if _, found := cache.Get("key1"); !found {
		t.Error("key1应该立即可用")
	}

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 现在应该过期了
	if _, found := cache.Get("key1"); found {
		t.Error("key1应该已经过期")
	}
}

// BenchmarkMemoryCache_LargeValues 测试大值性能
func BenchmarkMemoryCache_LargeValues(b *testing.B) {
	cache := NewMemoryCache(100, time.Hour)
	defer cache.(*MemoryCache).Stop()

	// 创建1MB的测试数据
	largeValue := make([]byte, 1024*1024)
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("large_key_%d", i)
		cache.Set(key, largeValue)
	}
}

// TestSearchCache_Performance 测试搜索缓存性能
func TestSearchCache_Performance(t *testing.T) {
	baseCache := NewMemoryCache(1000, time.Hour)
	defer baseCache.(*MemoryCache).Stop()

	searchCache := NewSearchCache(baseCache, 5*time.Minute, 100)

	// 创建测试查询
	query := types.SearchQuery{
		Path:    "/test/log.txt",
		Query:   "error",
		IsRegex: false,
		Offset:  0,
		Limit:   50,
	}

	// 创建测试结果
	result := &types.SearchResult{
		Entries:    make([]types.LogEntry, 50),
		TotalCount: 100,
		HasMore:    true,
		Offset:     0,
	}

	// 填充测试数据
	for i := range result.Entries {
		result.Entries[i] = types.LogEntry{
			Raw:     fmt.Sprintf("log line %d with error", i),
			LineNum: int64(i),
			Message: fmt.Sprintf("log line %d with error", i),
		}
	}

	// 测试设置和获取
	start := time.Now()
	searchCache.Set(query, result)
	setDuration := time.Since(start)

	start = time.Now()
	cachedResult, found := searchCache.Get(query)
	getDuration := time.Since(start)

	if !found {
		t.Error("应该找到缓存的搜索结果")
	}

	if cachedResult.TotalCount != result.TotalCount {
		t.Errorf("期望总数 %d，实际 %d", result.TotalCount, cachedResult.TotalCount)
	}

	t.Logf("搜索缓存设置耗时: %v", setDuration)
	t.Logf("搜索缓存获取耗时: %v", getDuration)

	// 性能要求：操作应该在10ms内完成（放宽要求）
	if setDuration > 10*time.Millisecond {
		t.Errorf("搜索缓存设置太慢: %v", setDuration)
	}

	if getDuration > 10*time.Millisecond {
		t.Errorf("搜索缓存获取太慢: %v", getDuration)
	}
}
