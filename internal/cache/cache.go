package cache

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/local-log-viewer/internal/interfaces"
)

// cacheItem 缓存项
type cacheItem struct {
	value       interface{}
	expiration  time.Time
	accessTime  time.Time
	accessCount int64
	size        int64
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	items       map[string]cacheItem
	mutex       sync.RWMutex
	maxSize     int
	maxMemory   int64
	currentSize int64
	ttl         time.Duration
	stopCh      chan struct{}
	running     bool

	// 统计信息
	hits      int64
	misses    int64
	evictions int64

	// 内存监控
	memoryLimit int64
	gcThreshold float64
}

// NewMemoryCache 创建新的内存缓存
func NewMemoryCache(maxSize int, ttl time.Duration) interfaces.LogCache {
	return NewMemoryCacheWithOptions(maxSize, ttl, 0, 0.8)
}

// NewMemoryCacheWithOptions 创建带选项的内存缓存
func NewMemoryCacheWithOptions(maxSize int, ttl time.Duration, memoryLimit int64, gcThreshold float64) interfaces.LogCache {
	if memoryLimit <= 0 {
		// 默认使用系统内存的 10%
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memoryLimit = int64(m.Sys) / 10
	}

	cache := &MemoryCache{
		items:       make(map[string]cacheItem),
		maxSize:     maxSize,
		maxMemory:   memoryLimit * 1024 * 1024, // 转换为字节
		ttl:         ttl,
		stopCh:      make(chan struct{}),
		memoryLimit: memoryLimit * 1024 * 1024,
		gcThreshold: gcThreshold,
	}

	// 启动清理协程
	go cache.cleanup()
	cache.running = true

	return cache
}

// Get 获取缓存内容
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	item, exists := c.items[key]
	if !exists {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	// 检查是否过期
	now := time.Now()
	if now.After(item.expiration) {
		// 删除过期项
		c.currentSize -= item.size
		delete(c.items, key)
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	// 更新访问统计
	item.accessTime = now
	atomic.AddInt64(&item.accessCount, 1)
	c.items[key] = item
	atomic.AddInt64(&c.hits, 1)

	return item.value, true
}

// Set 设置缓存内容
func (c *MemoryCache) Set(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	itemSize := c.estimateSize(value)

	// 检查是否需要删除现有项
	if existingItem, exists := c.items[key]; exists {
		c.currentSize -= existingItem.size
	}

	// 检查内存限制
	for c.currentSize+itemSize > c.memoryLimit && len(c.items) > 0 {
		c.evictLRU()
	}

	// 如果缓存已满，删除最旧的项
	for len(c.items) >= c.maxSize && c.maxSize > 0 {
		c.evictLRU()
	}

	// 设置过期时间
	expiration := now.Add(c.ttl)
	c.items[key] = cacheItem{
		value:       value,
		expiration:  expiration,
		accessTime:  now,
		accessCount: 0,
		size:        itemSize,
	}
	c.currentSize += itemSize

	// 检查是否需要触发GC
	if c.shouldTriggerGC() {
		go runtime.GC()
	}
}

// Delete 删除缓存内容
func (c *MemoryCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if item, exists := c.items[key]; exists {
		c.currentSize -= item.size
		delete(c.items, key)
	}
}

// Clear 清空缓存
func (c *MemoryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.items = make(map[string]cacheItem)
	c.currentSize = 0
}

// evictLRU 使用LRU策略删除缓存项
func (c *MemoryCache) evictLRU() {
	var lruKey string
	var lruScore float64

	for key, item := range c.items {
		// 计算LRU分数：结合访问时间和访问频率
		timeSinceAccess := time.Since(item.accessTime).Seconds()
		accessFreq := float64(item.accessCount)
		score := timeSinceAccess / (accessFreq + 1) // +1 避免除零

		if lruKey == "" || score > lruScore {
			lruKey = key
			lruScore = score
		}
	}

	if lruKey != "" {
		if item, exists := c.items[lruKey]; exists {
			c.currentSize -= item.size
			delete(c.items, lruKey)
			atomic.AddInt64(&c.evictions, 1)
		}
	}
}

// cleanup 清理过期项
func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpired()
		case <-c.stopCh:
			return
		}
	}
}

// cleanupExpired 清理过期项
func (c *MemoryCache) cleanupExpired() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.expiration) {
			delete(c.items, key)
		}
	}
}

// Stop 停止缓存
func (c *MemoryCache) Stop() {
	if c.running {
		close(c.stopCh)
		c.running = false
	}
}

// estimateSize 估算对象大小
func (c *MemoryCache) estimateSize(value interface{}) int64 {
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case []interface{}:
		size := int64(0)
		for _, item := range v {
			size += c.estimateSize(item)
		}
		return size
	default:
		// 对于其他类型，使用固定估算值
		return 1024 // 1KB 默认估算
	}
}

// shouldTriggerGC 检查是否应该触发GC
func (c *MemoryCache) shouldTriggerGC() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 如果当前内存使用超过阈值，触发GC
	memoryUsageRatio := float64(m.Alloc) / float64(m.Sys)
	return memoryUsageRatio > c.gcThreshold
}

// GetStats 获取缓存统计信息
func (c *MemoryCache) GetStats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return CacheStats{
		Hits:        atomic.LoadInt64(&c.hits),
		Misses:      atomic.LoadInt64(&c.misses),
		Evictions:   atomic.LoadInt64(&c.evictions),
		ItemCount:   len(c.items),
		CurrentSize: c.currentSize,
		MaxSize:     int64(c.maxSize),
		MaxMemory:   c.maxMemory,
		MemoryUsed:  int64(m.Alloc),
		HitRatio:    c.calculateHitRatio(),
	}
}

// calculateHitRatio 计算命中率
func (c *MemoryCache) calculateHitRatio() float64 {
	hits := atomic.LoadInt64(&c.hits)
	misses := atomic.LoadInt64(&c.misses)
	total := hits + misses

	if total == 0 {
		return 0.0
	}

	return float64(hits) / float64(total)
}

// CacheStats 缓存统计信息
type CacheStats struct {
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	Evictions   int64   `json:"evictions"`
	ItemCount   int     `json:"itemCount"`
	CurrentSize int64   `json:"currentSize"`
	MaxSize     int64   `json:"maxSize"`
	MaxMemory   int64   `json:"maxMemory"`
	MemoryUsed  int64   `json:"memoryUsed"`
	HitRatio    float64 `json:"hitRatio"`
}
