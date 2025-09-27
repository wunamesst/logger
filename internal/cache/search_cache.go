package cache

import (
	"crypto/md5"
	"fmt"
	"sync"
	"time"

	"github.com/local-log-viewer/internal/interfaces"
	"github.com/local-log-viewer/internal/types"
)

// SearchCache 搜索结果缓存
type SearchCache struct {
	cache      interfaces.LogCache
	mutex      sync.RWMutex
	ttl        time.Duration
	maxResults int
}

// NewSearchCache 创建搜索缓存
func NewSearchCache(cache interfaces.LogCache, ttl time.Duration, maxResults int) *SearchCache {
	return &SearchCache{
		cache:      cache,
		ttl:        ttl,
		maxResults: maxResults,
	}
}

// Get 获取搜索结果
func (sc *SearchCache) Get(query types.SearchQuery) (*types.SearchResult, bool) {
	key := sc.generateKey(query)

	if cached, found := sc.cache.Get(key); found {
		if result, ok := cached.(*CachedSearchResult); ok {
			// 检查是否过期
			if time.Now().Before(result.ExpiresAt) {
				return result.Result, true
			}
			// 过期了，删除缓存
			sc.cache.Delete(key)
		}
	}

	return nil, false
}

// Set 设置搜索结果
func (sc *SearchCache) Set(query types.SearchQuery, result *types.SearchResult) {
	// 限制缓存的结果数量
	if len(result.Entries) > sc.maxResults {
		limitedResult := *result
		limitedResult.Entries = result.Entries[:sc.maxResults]
		result = &limitedResult
	}

	key := sc.generateKey(query)
	cachedResult := &CachedSearchResult{
		Result:    result,
		ExpiresAt: time.Now().Add(sc.ttl),
		Query:     query,
	}

	sc.cache.Set(key, cachedResult)
}

// InvalidateFile 使文件相关的缓存失效
func (sc *SearchCache) InvalidateFile(filePath string) {
	// 这里需要遍历所有缓存项来找到相关的搜索结果
	// 为了简化，我们可以使用文件路径前缀来标识相关缓存
	// 实际实现中可能需要更复杂的索引机制
	sc.cache.Clear() // 简化实现：清空所有缓存
}

// generateKey 生成缓存键
func (sc *SearchCache) generateKey(query types.SearchQuery) string {
	// 创建查询的唯一标识
	data := fmt.Sprintf("%s|%s|%t|%v|%v|%v|%d|%d",
		query.Path,
		query.Query,
		query.IsRegex,
		query.StartTime.Unix(),
		query.EndTime.Unix(),
		query.Levels,
		query.Offset,
		query.Limit,
	)

	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("search:%x", hash)
}

// CachedSearchResult 缓存的搜索结果
type CachedSearchResult struct {
	Result    *types.SearchResult `json:"result"`
	ExpiresAt time.Time           `json:"expiresAt"`
	Query     types.SearchQuery   `json:"query"`
}

// FileContentCache 文件内容缓存
type FileContentCache struct {
	cache     interfaces.LogCache
	mutex     sync.RWMutex
	ttl       time.Duration
	maxSize   int64
	chunkSize int64
}

// NewFileContentCache 创建文件内容缓存
func NewFileContentCache(cache interfaces.LogCache, ttl time.Duration, maxSize int64) *FileContentCache {
	return &FileContentCache{
		cache:     cache,
		ttl:       ttl,
		maxSize:   maxSize,
		chunkSize: 1024 * 1024, // 1MB 块大小
	}
}

// GetChunk 获取文件块
func (fcc *FileContentCache) GetChunk(filePath string, offset int64, size int64) ([]byte, bool) {
	key := fmt.Sprintf("chunk:%s:%d:%d", filePath, offset, size)

	if cached, found := fcc.cache.Get(key); found {
		if chunk, ok := cached.(*CachedFileChunk); ok {
			if time.Now().Before(chunk.ExpiresAt) {
				return chunk.Data, true
			}
			fcc.cache.Delete(key)
		}
	}

	return nil, false
}

// SetChunk 设置文件块
func (fcc *FileContentCache) SetChunk(filePath string, offset int64, data []byte) {
	// 检查数据大小限制
	if int64(len(data)) > fcc.maxSize {
		return
	}

	key := fmt.Sprintf("chunk:%s:%d:%d", filePath, offset, int64(len(data)))
	chunk := &CachedFileChunk{
		Data:      data,
		ExpiresAt: time.Now().Add(fcc.ttl),
		FilePath:  filePath,
		Offset:    offset,
	}

	fcc.cache.Set(key, chunk)
}

// InvalidateFile 使文件缓存失效
func (fcc *FileContentCache) InvalidateFile(filePath string) {
	// 简化实现：清空所有缓存
	// 实际实现中应该只清除特定文件的缓存
	fcc.cache.Clear()
}

// CachedFileChunk 缓存的文件块
type CachedFileChunk struct {
	Data      []byte    `json:"data"`
	ExpiresAt time.Time `json:"expiresAt"`
	FilePath  string    `json:"filePath"`
	Offset    int64     `json:"offset"`
}

// MultiLevelCache 多级缓存
type MultiLevelCache struct {
	l1Cache interfaces.LogCache // 内存缓存
	l2Cache interfaces.LogCache // 可选的二级缓存
	mutex   sync.RWMutex
}

// NewMultiLevelCache 创建多级缓存
func NewMultiLevelCache(l1Cache, l2Cache interfaces.LogCache) *MultiLevelCache {
	return &MultiLevelCache{
		l1Cache: l1Cache,
		l2Cache: l2Cache,
	}
}

// Get 获取缓存内容
func (mlc *MultiLevelCache) Get(key string) (interface{}, bool) {
	// 先尝试L1缓存
	if value, found := mlc.l1Cache.Get(key); found {
		return value, true
	}

	// 如果L1缓存未命中，尝试L2缓存
	if mlc.l2Cache != nil {
		if value, found := mlc.l2Cache.Get(key); found {
			// 将数据提升到L1缓存
			mlc.l1Cache.Set(key, value)
			return value, true
		}
	}

	return nil, false
}

// Set 设置缓存内容
func (mlc *MultiLevelCache) Set(key string, value interface{}) {
	// 同时设置到两级缓存
	mlc.l1Cache.Set(key, value)
	if mlc.l2Cache != nil {
		mlc.l2Cache.Set(key, value)
	}
}

// Delete 删除缓存内容
func (mlc *MultiLevelCache) Delete(key string) {
	mlc.l1Cache.Delete(key)
	if mlc.l2Cache != nil {
		mlc.l2Cache.Delete(key)
	}
}

// Clear 清空缓存
func (mlc *MultiLevelCache) Clear() {
	mlc.l1Cache.Clear()
	if mlc.l2Cache != nil {
		mlc.l2Cache.Clear()
	}
}
