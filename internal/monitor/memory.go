package monitor

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// MemoryStats 内存统计信息
type MemoryStats struct {
	// 系统内存统计
	Alloc         uint64   `json:"alloc"`         // 当前分配的内存
	TotalAlloc    uint64   `json:"totalAlloc"`    // 总分配的内存
	Sys           uint64   `json:"sys"`           // 系统内存
	Lookups       uint64   `json:"lookups"`       // 指针查找次数
	Mallocs       uint64   `json:"mallocs"`       // 内存分配次数
	Frees         uint64   `json:"frees"`         // 内存释放次数
	HeapAlloc     uint64   `json:"heapAlloc"`     // 堆分配的内存
	HeapSys       uint64   `json:"heapSys"`       // 堆系统内存
	HeapIdle      uint64   `json:"heapIdle"`      // 堆空闲内存
	HeapInuse     uint64   `json:"heapInuse"`     // 堆使用中的内存
	HeapReleased  uint64   `json:"heapReleased"`  // 堆释放的内存
	HeapObjects   uint64   `json:"heapObjects"`   // 堆对象数量
	StackInuse    uint64   `json:"stackInuse"`    // 栈使用的内存
	StackSys      uint64   `json:"stackSys"`      // 栈系统内存
	MSpanInuse    uint64   `json:"mSpanInuse"`    // MSpan使用的内存
	MSpanSys      uint64   `json:"mSpanSys"`      // MSpan系统内存
	MCacheInuse   uint64   `json:"mCacheInuse"`   // MCache使用的内存
	MCacheSys     uint64   `json:"mCacheSys"`     // MCache系统内存
	BuckHashSys   uint64   `json:"buckHashSys"`   // 分析桶哈希表内存
	GCSys         uint64   `json:"gcSys"`         // GC系统内存
	OtherSys      uint64   `json:"otherSys"`      // 其他系统内存
	NextGC        uint64   `json:"nextGC"`        // 下次GC目标
	LastGC        uint64   `json:"lastGC"`        // 上次GC时间
	PauseTotalNs  uint64   `json:"pauseTotalNs"`  // GC暂停总时间
	PauseNs       []uint64 `json:"pauseNs"`       // GC暂停时间历史
	NumGC         uint32   `json:"numGC"`         // GC次数
	NumForcedGC   uint32   `json:"numForcedGC"`   // 强制GC次数
	GCCPUFraction float64  `json:"gcCPUFraction"` // GC CPU占用比例

	// 自定义统计
	UsagePercent float64   `json:"usagePercent"` // 内存使用百分比
	Timestamp    time.Time `json:"timestamp"`    // 统计时间
}

// MemoryMonitor 内存监控器
type MemoryMonitor struct {
	maxMemory     uint64
	warningLevel  float64
	criticalLevel float64

	// 回调函数
	warningCallback  func(stats MemoryStats)
	criticalCallback func(stats MemoryStats)

	// 监控状态
	running bool
	stopCh  chan struct{}
	mutex   sync.RWMutex

	// 统计历史
	statsHistory []MemoryStats
	maxHistory   int

	// 监控间隔
	interval time.Duration
}

// MemoryMonitorConfig 内存监控配置
type MemoryMonitorConfig struct {
	MaxMemory        uint64        // 最大内存限制（字节）
	WarningLevel     float64       // 警告阈值（百分比）
	CriticalLevel    float64       // 严重阈值（百分比）
	MonitorInterval  time.Duration // 监控间隔
	MaxHistory       int           // 最大历史记录数
	WarningCallback  func(MemoryStats)
	CriticalCallback func(MemoryStats)
}

// NewMemoryMonitor 创建内存监控器
func NewMemoryMonitor(config MemoryMonitorConfig) *MemoryMonitor {
	if config.MaxMemory == 0 {
		// 默认使用系统内存的80%
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		config.MaxMemory = uint64(float64(m.Sys) * 0.8)
	}

	if config.WarningLevel == 0 {
		config.WarningLevel = 0.7 // 70%
	}

	if config.CriticalLevel == 0 {
		config.CriticalLevel = 0.9 // 90%
	}

	if config.MonitorInterval == 0 {
		config.MonitorInterval = 30 * time.Second
	}

	if config.MaxHistory == 0 {
		config.MaxHistory = 100
	}

	return &MemoryMonitor{
		maxMemory:        config.MaxMemory,
		warningLevel:     config.WarningLevel,
		criticalLevel:    config.CriticalLevel,
		warningCallback:  config.WarningCallback,
		criticalCallback: config.CriticalCallback,
		interval:         config.MonitorInterval,
		maxHistory:       config.MaxHistory,
		stopCh:           make(chan struct{}),
		statsHistory:     make([]MemoryStats, 0, config.MaxHistory),
	}
}

// Start 启动内存监控
func (mm *MemoryMonitor) Start(ctx context.Context) error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if mm.running {
		return nil
	}

	mm.running = true
	go mm.monitor(ctx)

	return nil
}

// Stop 停止内存监控
func (mm *MemoryMonitor) Stop() error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if !mm.running {
		return nil
	}

	mm.running = false
	close(mm.stopCh)

	return nil
}

// GetCurrentStats 获取当前内存统计
func (mm *MemoryMonitor) GetCurrentStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := MemoryStats{
		Alloc:         m.Alloc,
		TotalAlloc:    m.TotalAlloc,
		Sys:           m.Sys,
		Lookups:       m.Lookups,
		Mallocs:       m.Mallocs,
		Frees:         m.Frees,
		HeapAlloc:     m.HeapAlloc,
		HeapSys:       m.HeapSys,
		HeapIdle:      m.HeapIdle,
		HeapInuse:     m.HeapInuse,
		HeapReleased:  m.HeapReleased,
		HeapObjects:   m.HeapObjects,
		StackInuse:    m.StackInuse,
		StackSys:      m.StackSys,
		MSpanInuse:    m.MSpanInuse,
		MSpanSys:      m.MSpanSys,
		MCacheInuse:   m.MCacheInuse,
		MCacheSys:     m.MCacheSys,
		BuckHashSys:   m.BuckHashSys,
		GCSys:         m.GCSys,
		OtherSys:      m.OtherSys,
		NextGC:        m.NextGC,
		LastGC:        m.LastGC,
		PauseTotalNs:  m.PauseTotalNs,
		NumGC:         m.NumGC,
		NumForcedGC:   m.NumForcedGC,
		GCCPUFraction: m.GCCPUFraction,
		UsagePercent:  float64(m.Alloc) / float64(mm.maxMemory) * 100,
		Timestamp:     time.Now(),
	}

	// 复制暂停时间历史（最近10次）
	pauseCount := len(m.PauseNs)
	if pauseCount > 10 {
		pauseCount = 10
	}
	stats.PauseNs = make([]uint64, pauseCount)
	copy(stats.PauseNs, m.PauseNs[:pauseCount])

	return stats
}

// GetStatsHistory 获取统计历史
func (mm *MemoryMonitor) GetStatsHistory() []MemoryStats {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	// 返回副本
	history := make([]MemoryStats, len(mm.statsHistory))
	copy(history, mm.statsHistory)
	return history
}

// IsMemoryPressure 检查是否存在内存压力
func (mm *MemoryMonitor) IsMemoryPressure() bool {
	stats := mm.GetCurrentStats()
	return stats.UsagePercent > mm.warningLevel*100
}

// ForceGC 强制垃圾回收
func (mm *MemoryMonitor) ForceGC() {
	runtime.GC()
}

// monitor 监控循环
func (mm *MemoryMonitor) monitor(ctx context.Context) {
	ticker := time.NewTicker(mm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mm.checkMemory()
		case <-ctx.Done():
			return
		case <-mm.stopCh:
			return
		}
	}
}

// checkMemory 检查内存使用情况
func (mm *MemoryMonitor) checkMemory() {
	stats := mm.GetCurrentStats()

	// 添加到历史记录
	mm.mutex.Lock()
	mm.statsHistory = append(mm.statsHistory, stats)
	if len(mm.statsHistory) > mm.maxHistory {
		mm.statsHistory = mm.statsHistory[1:]
	}
	mm.mutex.Unlock()

	// 检查阈值
	usagePercent := stats.UsagePercent / 100

	if usagePercent >= mm.criticalLevel {
		if mm.criticalCallback != nil {
			mm.criticalCallback(stats)
		}
		// 自动触发GC
		runtime.GC()
	} else if usagePercent >= mm.warningLevel {
		if mm.warningCallback != nil {
			mm.warningCallback(stats)
		}
	}
}

// GetMemoryPressureLevel 获取内存压力等级
func (mm *MemoryMonitor) GetMemoryPressureLevel() string {
	stats := mm.GetCurrentStats()
	usagePercent := stats.UsagePercent / 100

	if usagePercent >= mm.criticalLevel {
		return "critical"
	} else if usagePercent >= mm.warningLevel {
		return "warning"
	}
	return "normal"
}

// OptimizeMemory 内存优化建议
func (mm *MemoryMonitor) OptimizeMemory() []string {
	stats := mm.GetCurrentStats()
	var suggestions []string

	usagePercent := stats.UsagePercent / 100

	if usagePercent > mm.warningLevel {
		suggestions = append(suggestions, "内存使用率较高，建议清理缓存")

		if stats.HeapIdle > stats.HeapInuse {
			suggestions = append(suggestions, "堆内存碎片较多，建议触发GC")
		}

		if stats.NumGC == 0 || time.Since(time.Unix(0, int64(stats.LastGC))) > time.Minute*5 {
			suggestions = append(suggestions, "长时间未进行GC，建议手动触发")
		}
	}

	return suggestions
}
