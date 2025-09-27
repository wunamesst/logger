package pool

import (
	"bufio"
	"context"
	"os"
	"sync"
	"time"
)

// FileResource 文件资源
type FileResource struct {
	file   *os.File
	reader *bufio.Reader
	path   string
	opened time.Time
	mutex  sync.Mutex
}

// NewFileResource 创建文件资源
func NewFileResource(path string) (*FileResource, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &FileResource{
		file:   file,
		reader: bufio.NewReaderSize(file, 64*1024), // 64KB 缓冲区
		path:   path,
		opened: time.Now(),
	}, nil
}

// Close 关闭文件资源
func (fr *FileResource) Close() error {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()

	if fr.file != nil {
		err := fr.file.Close()
		fr.file = nil
		fr.reader = nil
		return err
	}
	return nil
}

// IsValid 检查文件资源是否有效
func (fr *FileResource) IsValid() bool {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()

	if fr.file == nil {
		return false
	}

	// 检查文件是否仍然存在
	if _, err := os.Stat(fr.path); err != nil {
		return false
	}

	return true
}

// GetFile 获取文件句柄
func (fr *FileResource) GetFile() *os.File {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	return fr.file
}

// GetReader 获取缓冲读取器
func (fr *FileResource) GetReader() *bufio.Reader {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	return fr.reader
}

// Reset 重置文件位置
func (fr *FileResource) Reset() error {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()

	if fr.file == nil {
		return nil
	}

	_, err := fr.file.Seek(0, 0)
	if err != nil {
		return err
	}

	fr.reader.Reset(fr.file)
	return nil
}

// FilePool 文件池
type FilePool struct {
	pools  map[string]Pool
	mutex  sync.RWMutex
	config PoolConfig
}

// NewFilePool 创建文件池
func NewFilePool(config PoolConfig) *FilePool {
	return &FilePool{
		pools:  make(map[string]Pool),
		config: config,
	}
}

// GetFileResource 获取文件资源
func (fp *FilePool) GetFileResource(path string) (*FileResource, error) {
	fp.mutex.RLock()
	pool, exists := fp.pools[path]
	fp.mutex.RUnlock()

	if !exists {
		fp.mutex.Lock()
		// 双重检查
		if pool, exists = fp.pools[path]; !exists {
			factory := func() (Resource, error) {
				return NewFileResource(path)
			}

			var err error
			pool, err = NewResourcePool(factory, fp.config)
			if err != nil {
				fp.mutex.Unlock()
				return nil, err
			}
			fp.pools[path] = pool
		}
		fp.mutex.Unlock()
	}

	ctx := context.Background()
	resource, err := pool.Get(ctx)
	if err != nil {
		return nil, err
	}

	return resource.(*FileResource), nil
}

// PutFileResource 归还文件资源
func (fp *FilePool) PutFileResource(path string, resource *FileResource) error {
	fp.mutex.RLock()
	pool, exists := fp.pools[path]
	fp.mutex.RUnlock()

	if !exists {
		resource.Close()
		return nil
	}

	return pool.Put(resource)
}

// Close 关闭文件池
func (fp *FilePool) Close() error {
	fp.mutex.Lock()
	defer fp.mutex.Unlock()

	for _, pool := range fp.pools {
		pool.Close()
	}
	fp.pools = make(map[string]Pool)
	return nil
}

// GetStats 获取所有文件池的统计信息
func (fp *FilePool) GetStats() map[string]PoolStats {
	fp.mutex.RLock()
	defer fp.mutex.RUnlock()

	stats := make(map[string]PoolStats)
	for path, pool := range fp.pools {
		stats[path] = pool.Stats()
	}
	return stats
}
