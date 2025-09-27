package pool

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// MockResource 模拟资源
type MockResource struct {
	id     int
	closed bool
	valid  bool
}

func NewMockResource(id int) *MockResource {
	return &MockResource{
		id:    id,
		valid: true,
	}
}

func (mr *MockResource) Close() error {
	mr.closed = true
	mr.valid = false
	return nil
}

func (mr *MockResource) IsValid() bool {
	return mr.valid && !mr.closed
}

// BenchmarkResourcePool_Get 测试资源池获取性能
func BenchmarkResourcePool_Get(b *testing.B) {
	factory := func() (Resource, error) {
		return NewMockResource(0), nil
	}

	config := PoolConfig{
		InitialSize: 10,
		MaxSize:     100,
		MaxIdleTime: time.Minute,
		MaxLifetime: time.Hour,
	}

	pool, err := NewResourcePool(factory, config)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resource, err := pool.Get(ctx)
			if err != nil {
				b.Error(err)
				continue
			}
			pool.Put(resource)
		}
	})
}

// BenchmarkResourcePool_Concurrent 测试并发性能
func BenchmarkResourcePool_Concurrent(b *testing.B) {
	factory := func() (Resource, error) {
		return NewMockResource(0), nil
	}

	config := PoolConfig{
		InitialSize: 5,
		MaxSize:     50,
		MaxIdleTime: time.Minute,
		MaxLifetime: time.Hour,
	}

	pool, err := NewResourcePool(factory, config)
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resource, err := pool.Get(ctx)
			if err != nil {
				b.Error(err)
				continue
			}

			// 模拟使用资源
			time.Sleep(time.Microsecond)

			pool.Put(resource)
		}
	})
}

// TestResourcePool_Concurrency 测试并发安全性
func TestResourcePool_Concurrency(t *testing.T) {
	resourceCounter := 0
	factory := func() (Resource, error) {
		resourceCounter++
		return NewMockResource(resourceCounter), nil
	}

	config := PoolConfig{
		InitialSize: 2,
		MaxSize:     10,
		MaxIdleTime: time.Minute,
		MaxLifetime: time.Hour,
	}

	pool, err := NewResourcePool(factory, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 100

	ctx := context.Background()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				resource, err := pool.Get(ctx)
				if err != nil {
					t.Errorf("获取资源失败: %v", err)
					continue
				}

				// 检查资源有效性
				if !resource.IsValid() {
					t.Errorf("获取到无效资源")
				}

				// 模拟使用
				time.Sleep(time.Microsecond * 10)

				// 归还资源
				if err := pool.Put(resource); err != nil {
					t.Errorf("归还资源失败: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// 检查统计信息
	stats := pool.Stats()
	t.Logf("资源池统计: %+v", stats)

	if stats.Active < 0 {
		t.Errorf("活跃资源数不能为负数: %d", stats.Active)
	}
}

// TestFilePool_Performance 测试文件池性能
func TestFilePool_Performance(t *testing.T) {
	// 创建临时测试文件
	tmpFile, err := os.CreateTemp("", "test_log_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// 写入测试数据
	testData := "test log line 1\ntest log line 2\ntest log line 3\n"
	if _, err := tmpFile.WriteString(testData); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	config := PoolConfig{
		InitialSize: 2,
		MaxSize:     5,
		MaxIdleTime: time.Minute,
		MaxLifetime: time.Hour,
	}

	filePool := NewFilePool(config)
	defer filePool.Close()

	// 测试并发文件访问
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 50

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// 获取文件资源
				fileResource, err := filePool.GetFileResource(tmpFile.Name())
				if err != nil {
					t.Errorf("获取文件资源失败: %v", err)
					continue
				}

				// 读取文件内容
				reader := fileResource.GetReader()
				if reader == nil {
					t.Errorf("获取文件读取器失败")
					continue
				}

				// 模拟读取操作
				buffer := make([]byte, 100)
				_, err = reader.Read(buffer)
				if err != nil && err.Error() != "EOF" {
					t.Errorf("读取文件失败: %v", err)
				}

				// 重置文件位置
				if err := fileResource.Reset(); err != nil {
					t.Errorf("重置文件位置失败: %v", err)
				}

				// 归还文件资源
				if err := filePool.PutFileResource(tmpFile.Name(), fileResource); err != nil {
					t.Errorf("归还文件资源失败: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("文件池并发测试完成，耗时: %v", duration)
	t.Logf("总操作数: %d", numGoroutines*numOperations)
	t.Logf("平均每操作耗时: %v", duration/time.Duration(numGoroutines*numOperations))

	// 获取统计信息
	stats := filePool.GetStats()
	t.Logf("文件池统计: %+v", stats)
}

// BenchmarkFilePool_GetPut 测试文件池获取归还性能
func BenchmarkFilePool_GetPut(b *testing.B) {
	// 创建临时测试文件
	tmpFile, err := os.CreateTemp("", "bench_log_*.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	testData := "benchmark test data\n"
	for i := 0; i < 1000; i++ {
		tmpFile.WriteString(fmt.Sprintf("line %d: %s", i, testData))
	}
	tmpFile.Close()

	config := PoolConfig{
		InitialSize: 5,
		MaxSize:     20,
		MaxIdleTime: time.Minute,
		MaxLifetime: time.Hour,
	}

	filePool := NewFilePool(config)
	defer filePool.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			fileResource, err := filePool.GetFileResource(tmpFile.Name())
			if err != nil {
				b.Error(err)
				continue
			}

			// 模拟读取操作
			reader := fileResource.GetReader()
			buffer := make([]byte, 64)
			reader.Read(buffer)

			filePool.PutFileResource(tmpFile.Name(), fileResource)
		}
	})
}

// TestResourcePool_MemoryLeak 测试内存泄漏
func TestResourcePool_MemoryLeak(t *testing.T) {
	factory := func() (Resource, error) {
		return NewMockResource(0), nil
	}

	config := PoolConfig{
		InitialSize: 1,
		MaxSize:     5,
		MaxIdleTime: 100 * time.Millisecond, // 短过期时间
		MaxLifetime: 200 * time.Millisecond,
	}

	pool, err := NewResourcePool(factory, config)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// 创建和销毁大量资源
	for i := 0; i < 1000; i++ {
		resource, err := pool.Get(ctx)
		if err != nil {
			t.Error(err)
			continue
		}
		pool.Put(resource)
	}

	// 等待资源过期
	time.Sleep(300 * time.Millisecond)

	// 检查统计信息
	stats := pool.Stats()
	t.Logf("最终统计: %+v", stats)

	// 关闭池
	if err := pool.Close(); err != nil {
		t.Errorf("关闭资源池失败: %v", err)
	}

	// 再次尝试获取资源应该失败
	_, err = pool.Get(ctx)
	if err != ErrPoolClosed {
		t.Errorf("期望 ErrPoolClosed，实际 %v", err)
	}
}
