package pool

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrPoolClosed = errors.New("pool is closed")
	ErrPoolEmpty  = errors.New("pool is empty")
)

// Resource 资源接口
type Resource interface {
	// Close 关闭资源
	Close() error
	// IsValid 检查资源是否有效
	IsValid() bool
}

// Factory 资源工厂函数
type Factory func() (Resource, error)

// Pool 资源池接口
type Pool interface {
	// Get 获取资源
	Get(ctx context.Context) (Resource, error)
	// Put 归还资源
	Put(resource Resource) error
	// Close 关闭资源池
	Close() error
	// Stats 获取统计信息
	Stats() PoolStats
}

// PoolConfig 资源池配置
type PoolConfig struct {
	// InitialSize 初始大小
	InitialSize int
	// MaxSize 最大大小
	MaxSize int
	// MaxIdleTime 最大空闲时间
	MaxIdleTime time.Duration
	// MaxLifetime 最大生命周期
	MaxLifetime time.Duration
	// HealthCheckInterval 健康检查间隔
	HealthCheckInterval time.Duration
}

// pooledResource 池化资源
type pooledResource struct {
	resource   Resource
	createTime time.Time
	lastUsed   time.Time
}

// ResourcePool 资源池实现
type ResourcePool struct {
	factory Factory
	config  PoolConfig

	resources chan *pooledResource
	mutex     sync.RWMutex
	closed    bool

	// 统计信息
	created   int64
	borrowed  int64
	returned  int64
	destroyed int64

	// 清理协程
	stopCh chan struct{}
}

// NewResourcePool 创建新的资源池
func NewResourcePool(factory Factory, config PoolConfig) (Pool, error) {
	if config.MaxSize <= 0 {
		config.MaxSize = 10
	}
	if config.InitialSize < 0 {
		config.InitialSize = 0
	}
	if config.InitialSize > config.MaxSize {
		config.InitialSize = config.MaxSize
	}
	if config.MaxIdleTime <= 0 {
		config.MaxIdleTime = 30 * time.Minute
	}
	if config.MaxLifetime <= 0 {
		config.MaxLifetime = time.Hour
	}
	if config.HealthCheckInterval <= 0 {
		config.HealthCheckInterval = 5 * time.Minute
	}

	pool := &ResourcePool{
		factory:   factory,
		config:    config,
		resources: make(chan *pooledResource, config.MaxSize),
		stopCh:    make(chan struct{}),
	}

	// 创建初始资源
	for i := 0; i < config.InitialSize; i++ {
		resource, err := factory()
		if err != nil {
			pool.Close()
			return nil, err
		}

		pooledRes := &pooledResource{
			resource:   resource,
			createTime: time.Now(),
			lastUsed:   time.Now(),
		}

		select {
		case pool.resources <- pooledRes:
			pool.created++
		default:
			resource.Close()
		}
	}

	// 启动清理协程
	go pool.cleanup()

	return pool, nil
}

// Get 获取资源
func (p *ResourcePool) Get(ctx context.Context) (Resource, error) {
	p.mutex.RLock()
	if p.closed {
		p.mutex.RUnlock()
		return nil, ErrPoolClosed
	}
	p.mutex.RUnlock()

	select {
	case pooledRes := <-p.resources:
		// 检查资源是否有效
		if p.isResourceValid(pooledRes) {
			pooledRes.lastUsed = time.Now()
			p.borrowed++
			return pooledRes.resource, nil
		}
		// 资源无效，销毁并创建新的
		pooledRes.resource.Close()
		p.destroyed++

		// 创建新资源
		resource, err := p.factory()
		if err != nil {
			return nil, err
		}
		p.created++
		p.borrowed++
		return resource, nil

	case <-ctx.Done():
		return nil, ctx.Err()

	default:
		// 池为空，尝试创建新资源
		resource, err := p.factory()
		if err != nil {
			return nil, err
		}
		p.created++
		p.borrowed++
		return resource, nil
	}
}

// Put 归还资源
func (p *ResourcePool) Put(resource Resource) error {
	if resource == nil {
		return nil
	}

	p.mutex.RLock()
	if p.closed {
		p.mutex.RUnlock()
		resource.Close()
		return ErrPoolClosed
	}
	p.mutex.RUnlock()

	pooledRes := &pooledResource{
		resource:   resource,
		createTime: time.Now(), // 这里应该保留原始创建时间，但为了简化先用当前时间
		lastUsed:   time.Now(),
	}

	select {
	case p.resources <- pooledRes:
		p.returned++
		return nil
	default:
		// 池已满，直接关闭资源
		resource.Close()
		p.destroyed++
		return nil
	}
}

// Close 关闭资源池
func (p *ResourcePool) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.stopCh)

	// 关闭所有资源
	close(p.resources)
	for pooledRes := range p.resources {
		pooledRes.resource.Close()
		p.destroyed++
	}

	return nil
}

// Stats 获取统计信息
func (p *ResourcePool) Stats() PoolStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return PoolStats{
		Created:     p.created,
		Borrowed:    p.borrowed,
		Returned:    p.returned,
		Destroyed:   p.destroyed,
		Active:      p.borrowed - p.returned,
		Idle:        int64(len(p.resources)),
		MaxSize:     int64(p.config.MaxSize),
		InitialSize: int64(p.config.InitialSize),
	}
}

// isResourceValid 检查资源是否有效
func (p *ResourcePool) isResourceValid(pooledRes *pooledResource) bool {
	now := time.Now()

	// 检查生命周期
	if now.Sub(pooledRes.createTime) > p.config.MaxLifetime {
		return false
	}

	// 检查空闲时间
	if now.Sub(pooledRes.lastUsed) > p.config.MaxIdleTime {
		return false
	}

	// 检查资源本身是否有效
	return pooledRes.resource.IsValid()
}

// cleanup 清理过期资源
func (p *ResourcePool) cleanup() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupExpiredResources()
		case <-p.stopCh:
			return
		}
	}
}

// cleanupExpiredResources 清理过期资源
func (p *ResourcePool) cleanupExpiredResources() {
	p.mutex.RLock()
	if p.closed {
		p.mutex.RUnlock()
		return
	}
	p.mutex.RUnlock()

	// 检查池中的资源
	resourceCount := len(p.resources)
	for i := 0; i < resourceCount; i++ {
		select {
		case pooledRes := <-p.resources:
			if p.isResourceValid(pooledRes) {
				// 资源有效，放回池中
				select {
				case p.resources <- pooledRes:
				default:
					// 池已满，关闭资源
					pooledRes.resource.Close()
					p.destroyed++
				}
			} else {
				// 资源无效，关闭
				pooledRes.resource.Close()
				p.destroyed++
			}
		default:
			// 没有更多资源
			return
		}
	}
}

// PoolStats 资源池统计信息
type PoolStats struct {
	Created     int64 `json:"created"`
	Borrowed    int64 `json:"borrowed"`
	Returned    int64 `json:"returned"`
	Destroyed   int64 `json:"destroyed"`
	Active      int64 `json:"active"`
	Idle        int64 `json:"idle"`
	MaxSize     int64 `json:"maxSize"`
	InitialSize int64 `json:"initialSize"`
}
