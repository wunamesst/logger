package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/local-log-viewer/internal/logger"
	"go.uber.org/zap"
)

// Hook 关闭钩子函数
type Hook func(ctx context.Context) error

// Manager 优雅关闭管理器
type Manager struct {
	hooks    []Hook
	mu       sync.RWMutex
	timeout  time.Duration
	signals  []os.Signal
	shutdown chan struct{}
	done     chan struct{}
}

// NewManager 创建关闭管理器
func NewManager(timeout time.Duration) *Manager {
	return &Manager{
		hooks:    make([]Hook, 0),
		timeout:  timeout,
		signals:  []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		shutdown: make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// AddHook 添加关闭钩子
func (m *Manager) AddHook(hook Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, hook)
}

// SetSignals 设置监听的信号
func (m *Manager) SetSignals(signals ...os.Signal) {
	m.signals = signals
}

// Start 开始监听关闭信号
func (m *Manager) Start() {
	go m.listen()
}

// Shutdown 手动触发关闭
func (m *Manager) Shutdown() {
	select {
	case <-m.shutdown:
		// 已经在关闭中
		return
	default:
		close(m.shutdown)
	}
}

// Wait 等待关闭完成
func (m *Manager) Wait() {
	<-m.done
}

// listen 监听关闭信号
func (m *Manager) listen() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, m.signals...)

	select {
	case sig := <-sigChan:
		logger.Info("received shutdown signal", zap.String("signal", sig.String()))
		m.executeShutdown()
	case <-m.shutdown:
		logger.Info("manual shutdown triggered")
		m.executeShutdown()
	}
}

// executeShutdown 执行关闭流程
func (m *Manager) executeShutdown() {
	defer close(m.done)

	logger.Info("starting graceful shutdown", zap.Duration("timeout", m.timeout))

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	m.mu.RLock()
	hooks := make([]Hook, len(m.hooks))
	copy(hooks, m.hooks)
	m.mu.RUnlock()

	// 并发执行所有关闭钩子
	var wg sync.WaitGroup
	errors := make(chan error, len(hooks))

	for i, hook := range hooks {
		wg.Add(1)
		go func(index int, h Hook) {
			defer wg.Done()

			logger.Debug("executing shutdown hook", zap.Int("hook_index", index))

			if err := h(ctx); err != nil {
				logger.Error("shutdown hook failed",
					zap.Int("hook_index", index),
					zap.Error(err),
				)
				errors <- err
			} else {
				logger.Debug("shutdown hook completed", zap.Int("hook_index", index))
			}
		}(i, hook)
	}

	// 等待所有钩子完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("all shutdown hooks completed successfully")
	case <-ctx.Done():
		logger.Warn("shutdown timeout reached, some hooks may not have completed")
	}

	// 收集错误
	close(errors)
	errorCount := 0
	for err := range errors {
		errorCount++
		logger.Error("shutdown error", zap.Error(err))
	}

	if errorCount > 0 {
		logger.Warn("shutdown completed with errors", zap.Int("error_count", errorCount))
	} else {
		logger.Info("graceful shutdown completed successfully")
	}
}
