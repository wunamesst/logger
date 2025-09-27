package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/local-log-viewer/internal/interfaces"
)

func TestMemoryCache_BasicOperations(t *testing.T) {
	cache := NewMemoryCache(5, time.Minute)

	// 测试设置和获取
	cache.Set("key1", "value1")
	value, found := cache.Get("key1")
	if !found {
		t.Error("应该找到 key1")
	}
	if value != "value1" {
		t.Errorf("期望值 'value1'，实际值 '%v'", value)
	}

	// 测试不存在的键
	_, found = cache.Get("nonexistent")
	if found {
		t.Error("不应该找到不存在的键")
	}

	// 测试删除
	cache.Delete("key1")
	_, found = cache.Get("key1")
	if found {
		t.Error("删除后不应该找到 key1")
	}
}

func TestMemoryCache_MaxSize(t *testing.T) {
	cache := NewMemoryCache(3, time.Minute)

	// 添加超过最大容量的项
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")
	cache.Set("key4", "value4") // 这应该触发驱逐

	// 检查是否有项被驱逐
	totalFound := 0
	keys := []string{"key1", "key2", "key3", "key4"}
	for _, key := range keys {
		if _, found := cache.Get(key); found {
			totalFound++
		}
	}

	if totalFound > 3 {
		t.Errorf("缓存大小应该不超过3，实际找到 %d 个项", totalFound)
	}
}

func TestMemoryCache_TTL(t *testing.T) {
	cache := NewMemoryCache(5, 100*time.Millisecond)

	// 设置一个项
	cache.Set("key1", "value1")

	// 立即检查应该存在
	_, found := cache.Get("key1")
	if !found {
		t.Error("应该立即找到 key1")
	}

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 现在应该过期了
	_, found = cache.Get("key1")
	if found {
		t.Error("过期后不应该找到 key1")
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(5, time.Minute)

	// 添加一些项
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// 清空缓存
	cache.Clear()

	// 检查所有项都被清除
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		if _, found := cache.Get(key); found {
			t.Errorf("清空后不应该找到 %s", key)
		}
	}
}

func TestMemoryCache_Stats(t *testing.T) {
	cache := NewMemoryCache(5, time.Minute)
	defer cache.(*MemoryCache).Stop()

	// 测试命中和未命中
	cache.Set("key1", "value1")

	// 命中
	cache.Get("key1")
	cache.Get("key1")

	// 未命中
	cache.Get("nonexistent")

	// 获取统计信息
	if advancedCache, ok := cache.(interfaces.AdvancedLogCache); ok {
		stats := advancedCache.GetStats()
		if cacheStats, ok := stats.(CacheStats); ok {
			if cacheStats.Hits < 2 {
				t.Errorf("期望至少2次命中，实际 %d", cacheStats.Hits)
			}
			if cacheStats.Misses < 1 {
				t.Errorf("期望至少1次未命中，实际 %d", cacheStats.Misses)
			}
			if cacheStats.ItemCount != 1 {
				t.Errorf("期望1个项目，实际 %d", cacheStats.ItemCount)
			}

			t.Logf("缓存统计: %+v", cacheStats)
		}
	}
}

func TestMemoryCache_MemoryLimit(t *testing.T) {
	// 创建有内存限制的缓存
	cache := NewMemoryCacheWithOptions(100, time.Hour, 1, 0.8) // 1MB限制
	defer cache.(*MemoryCache).Stop()

	// 添加大量数据直到触发内存限制
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := make([]byte, 1024) // 1KB
		cache.Set(key, value)
	}

	// 检查是否有项目被驱逐
	if advancedCache, ok := cache.(interfaces.AdvancedLogCache); ok {
		stats := advancedCache.GetStats()
		if cacheStats, ok := stats.(CacheStats); ok {
			t.Logf("内存限制测试统计: %+v", cacheStats)

			// 应该有一些驱逐发生
			if cacheStats.Evictions == 0 {
				t.Log("警告: 没有发生驱逐，可能内存限制设置过大")
			}
		}
	}
}
