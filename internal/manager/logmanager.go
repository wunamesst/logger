package manager

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/local-log-viewer/internal/cache"
	"github.com/local-log-viewer/internal/config"
	"github.com/local-log-viewer/internal/interfaces"
	"github.com/local-log-viewer/internal/logger"
	"github.com/local-log-viewer/internal/monitor"
	"github.com/local-log-viewer/internal/parser"
	"github.com/local-log-viewer/internal/pool"
	"github.com/local-log-viewer/internal/types"
	"go.uber.org/zap"
)

// LogManager 日志管理器实现
type LogManager struct {
	config      *config.Config
	fileWatcher interfaces.FileWatcher
	parsers     map[string]interfaces.LogParser
	cache       interfaces.LogCache

	// 性能优化组件
	filePool      *pool.FilePool
	searchCache   *cache.SearchCache
	contentCache  *cache.FileContentCache
	memoryMonitor *monitor.MemoryMonitor

	// 文件监控相关
	watchedFiles  map[string]chan types.LogUpdate
	filePositions map[string]int64 // 记录每个文件的读取位置(字节偏移量)
	watchMutex    sync.RWMutex

	// 运行状态
	running bool
	stopCh  chan struct{}
	mutex   sync.RWMutex
}

// NewLogManager 创建新的日志管理器
func NewLogManager(cfg *config.Config, fileWatcher interfaces.FileWatcher, logCache interfaces.LogCache) interfaces.LogManager {
	// 创建文件池
	poolConfig := pool.PoolConfig{
		InitialSize:         2,
		MaxSize:             10,
		MaxIdleTime:         30 * time.Minute,
		MaxLifetime:         time.Hour,
		HealthCheckInterval: 5 * time.Minute,
	}
	filePool := pool.NewFilePool(poolConfig)

	// 创建搜索缓存
	searchCache := cache.NewSearchCache(logCache, 10*time.Minute, 1000)

	// 创建内容缓存
	contentCache := cache.NewFileContentCache(logCache, 5*time.Minute, 10*1024*1024) // 10MB

	// 创建内存监控器
	memoryConfig := monitor.MemoryMonitorConfig{
		MaxMemory:       100 * 1024 * 1024, // 100MB
		WarningLevel:    0.7,
		CriticalLevel:   0.9,
		MonitorInterval: 30 * time.Second,
		MaxHistory:      100,
		WarningCallback: func(stats monitor.MemoryStats) {
			// 内存警告时清理部分缓存
			if advancedCache, ok := logCache.(interfaces.AdvancedLogCache); ok {
				// 可以获取缓存统计并进行优化
				_ = advancedCache
			}
		},
		CriticalCallback: func(stats monitor.MemoryStats) {
			// 内存严重不足时强制清理缓存
			logCache.Clear()
		},
	}
	memoryMonitor := monitor.NewMemoryMonitor(memoryConfig)

	return &LogManager{
		config:        cfg,
		fileWatcher:   fileWatcher,
		parsers:       make(map[string]interfaces.LogParser),
		cache:         logCache,
		filePool:      filePool,
		searchCache:   searchCache,
		contentCache:  contentCache,
		memoryMonitor: memoryMonitor,
		watchedFiles:  make(map[string]chan types.LogUpdate),
		filePositions: make(map[string]int64),
		stopCh:        make(chan struct{}),
	}
}

// GetLogFiles 获取日志文件列表
func (lm *LogManager) GetLogFiles() ([]types.LogFile, error) {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	var allFiles []types.LogFile

	// 遍历所有配置的日志路径
	for _, rootPath := range lm.config.Server.LogPaths {
		absPath, err := filepath.Abs(rootPath)
		if err != nil {
			continue // 跳过无效路径
		}

		// 检查路径是否存在
		info, err := os.Stat(absPath)
		if err != nil {
			continue // 跳过不存在的路径
		}

		if info.IsDir() {
			// 如果是目录，递归扫描
			files, err := lm.scanDirectory(absPath)
			if err != nil {
				continue // 跳过扫描失败的目录
			}
			allFiles = append(allFiles, files...)
		} else {
			// 如果是文件，直接添加
			logFile := lm.createLogFile(absPath, info)
			allFiles = append(allFiles, logFile)
		}
	}

	// 构建树形结构
	return lm.buildFileTree(allFiles), nil
}

// scanDirectory 递归扫描目录
func (lm *LogManager) scanDirectory(dirPath string) ([]types.LogFile, error) {
	var files []types.LogFile

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误的文件/目录
		}

		// 跳过隐藏文件和目录
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 只处理常见的日志文件扩展名
		if !info.IsDir() && lm.isLogFile(path) {
			// 检查文件大小限制
			if info.Size() > lm.config.Server.MaxFileSize {
				return nil // 跳过过大的文件
			}

			logFile := lm.createLogFile(path, info)
			files = append(files, logFile)
		}

		return nil
	})

	return files, err
}

// isLogFile 判断是否为日志文件
func (lm *LogManager) isLogFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	name := strings.ToLower(filepath.Base(path))

	// 明确的日志文件扩展名
	logExtensions := []string{".log", ".out", ".err"}
	for _, logExt := range logExtensions {
		if ext == logExt {
			return true
		}
	}

	// 对于 .txt 和 .json，需要检查文件名是否包含日志关键词
	if ext == ".txt" || ext == ".json" {
		logKeywords := []string{"log", "access", "error", "debug", "info", "warn"}
		for _, keyword := range logKeywords {
			if strings.Contains(name, keyword) {
				return true
			}
		}
	}

	// 对于没有扩展名的文件，检查是否包含日志关键词
	if ext == "" {
		logKeywords := []string{"log", "access", "error", "debug", "info", "warn"}
		for _, keyword := range logKeywords {
			if strings.Contains(name, keyword) {
				return true
			}
		}
	}

	return false
}

// createLogFile 创建日志文件对象
func (lm *LogManager) createLogFile(path string, info os.FileInfo) types.LogFile {
	return types.LogFile{
		Path:        path,
		Name:        info.Name(),
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		IsDirectory: info.IsDir(),
	}
}

// buildFileTree 构建文件树形结构
func (lm *LogManager) buildFileTree(files []types.LogFile) []types.LogFile {
	if len(files) == 0 {
		return files
	}

	// 按路径排序
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	// 构建目录映射
	dirMap := make(map[string]*types.LogFile)
	var roots []types.LogFile

	for _, file := range files {
		dir := filepath.Dir(file.Path)

		// 创建目录节点（如果不存在）
		if _, exists := dirMap[dir]; !exists {
			dirInfo, err := os.Stat(dir)
			if err == nil {
				dirFile := types.LogFile{
					Path:        dir,
					Name:        filepath.Base(dir),
					Size:        0,
					ModTime:     dirInfo.ModTime(),
					IsDirectory: true,
					Children:    []types.LogFile{},
				}
				dirMap[dir] = &dirFile
			}
		}

		// 将文件添加到对应目录
		if dirNode, exists := dirMap[dir]; exists {
			dirNode.Children = append(dirNode.Children, file)
		}
	}

	// 构建根节点列表
	for _, dirFile := range dirMap {
		parentDir := filepath.Dir(dirFile.Path)
		if parentNode, exists := dirMap[parentDir]; exists && parentDir != dirFile.Path {
			// 这是子目录，添加到父目录
			parentNode.Children = append(parentNode.Children, *dirFile)
		} else {
			// 这是根目录
			roots = append(roots, *dirFile)
		}
	}

	// 如果没有目录结构，直接返回文件列表
	if len(roots) == 0 {
		return files
	}

	return roots
}

// ReadLogFile 读取日志文件内容
func (lm *LogManager) ReadLogFile(path string, offset int64, limit int) (*types.LogContent, error) {
	// 检查内存压力
	if lm.memoryMonitor.IsMemoryPressure() {
		// 内存压力大时，清理部分缓存
		lm.cache.Clear()
	}

	// 检查文件是否存在
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}

	// 检查文件大小限制
	if info.Size() > lm.config.Server.MaxFileSize {
		return nil, fmt.Errorf("文件过大，超过限制 %d 字节", lm.config.Server.MaxFileSize)
	}

	// 尝试从缓存获取，包含文件修改时间以确保缓存失效
	cacheKey := fmt.Sprintf("file:%s:%d:%d:%d", path, offset, limit, info.ModTime().Unix())
	if cached, found := lm.cache.Get(cacheKey); found {
		if content, ok := cached.(*types.LogContent); ok {
			return content, nil
		}
	}

	// 使用文件池获取文件资源
	fileResource, err := lm.filePool.GetFileResource(path)
	if err != nil {
		return nil, fmt.Errorf("获取文件资源失败: %w", err)
	}
	defer lm.filePool.PutFileResource(path, fileResource)

	// 重置文件位置
	if err := fileResource.Reset(); err != nil {
		return nil, fmt.Errorf("重置文件位置失败: %w", err)
	}

	// 流式读取文件内容
	content, err := lm.readFileContentOptimized(fileResource, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}

	// 缓存结果（如果内存允许）
	if !lm.memoryMonitor.IsMemoryPressure() {
		lm.cache.Set(cacheKey, content)
	}

	return content, nil
}

// readFileContent 流式读取文件内容
func (lm *LogManager) readFileContent(file *os.File, offset int64, limit int) (*types.LogContent, error) {
	// 计算总行数（估算）
	totalLines, err := lm.countLines(file)
	if err != nil {
		return nil, err
	}

	// 重新定位到文件开头
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	// 创建新的扫描器
	scanner := bufio.NewScanner(file)

	// 跳过指定行数
	for i := int64(0); i < offset && scanner.Scan(); i++ {
		// 跳过行
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 读取指定数量的行
	var entries []types.LogEntry
	lineNum := offset

	for i := 0; i < limit && scanner.Scan(); i++ {
		line := scanner.Text()

		// 创建日志条目
		entry := types.LogEntry{
			Raw:     line,
			LineNum: lineNum,
		}

		// 尝试解析日志行
		if parser := lm.findParser(line); parser != nil {
			if parsed, err := parser.Parse(line); err == nil {
				entry = *parsed
				entry.LineNum = lineNum
			}
		}

		entries = append(entries, entry)
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 检查是否还有更多内容
	hasMore := scanner.Scan()

	return &types.LogContent{
		Entries:    entries,
		TotalLines: totalLines,
		HasMore:    hasMore,
		Offset:     offset,
	}, nil
}

// countLines 计算文件总行数
func (lm *LogManager) countLines(file *os.File) (int64, error) {
	// 保存当前位置
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	// 回到文件开头
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return 0, err
	}

	// 计算行数
	var lines int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines++
	}

	// 恢复原来的位置
	_, err = file.Seek(currentPos, io.SeekStart)
	if err != nil {
		return 0, err
	}

	return lines, scanner.Err()
}

// readFileContentOptimized 优化的流式读取文件内容
func (lm *LogManager) readFileContentOptimized(fileResource *pool.FileResource, offset int64, limit int) (*types.LogContent, error) {
	file := fileResource.GetFile()
	reader := fileResource.GetReader()

	// 计算总行数（估算）
	totalLines, err := lm.countLines(file)
	if err != nil {
		return nil, err
	}

	// 重新定位到文件开头
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	reader.Reset(file)

	// 创建新的扫描器，使用更大的缓冲区
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024) // 64KB 缓冲区
	scanner.Buffer(buf, 1024*1024)  // 最大1MB行长度

	// 跳过指定行数
	for i := int64(0); i < offset && scanner.Scan(); i++ {
		// 跳过行
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 读取指定数量的行
	var entries []types.LogEntry
	lineNum := offset

	for i := 0; i < limit && scanner.Scan(); i++ {
		line := scanner.Text()

		// 创建日志条目
		entry := types.LogEntry{
			Raw:     line,
			LineNum: lineNum,
		}

		// 尝试解析日志行
		if parser := lm.findParser(line); parser != nil {
			if parsed, err := parser.Parse(line); err == nil {
				entry = *parsed
				entry.LineNum = lineNum
			}
		}

		entries = append(entries, entry)
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 检查是否还有更多内容
	hasMore := scanner.Scan()

	return &types.LogContent{
		Entries:    entries,
		TotalLines: totalLines,
		HasMore:    hasMore,
		Offset:     offset,
	}, nil
}

// findParser 查找合适的解析器
func (lm *LogManager) findParser(line string) interfaces.LogParser {
	for _, parser := range lm.parsers {
		if parser.CanParse(line) {
			return parser
		}
	}
	return nil
}

// SearchLogs 搜索日志内容
func (lm *LogManager) SearchLogs(query types.SearchQuery) (*types.SearchResult, error) {
	// 尝试从搜索缓存获取结果
	if result, found := lm.searchCache.Get(query); found {
		return result, nil
	}

	// 检查内存压力
	if lm.memoryMonitor.IsMemoryPressure() {
		// 内存压力大时，限制搜索结果数量
		if query.Limit > 100 {
			query.Limit = 100
		}
	}

	// 执行搜索（这里是基本实现，实际搜索逻辑在搜索引擎中）
	result := &types.SearchResult{
		Entries:    []types.LogEntry{},
		TotalCount: 0,
		HasMore:    false,
		Offset:     query.Offset,
	}

	// 缓存搜索结果（如果内存允许）
	if !lm.memoryMonitor.IsMemoryPressure() {
		lm.searchCache.Set(query, result)
	}

	return result, nil
}

// WatchFile 监控文件变化
func (lm *LogManager) WatchFile(path string) (<-chan types.LogUpdate, error) {
	lm.watchMutex.Lock()
	defer lm.watchMutex.Unlock()

	// 使用传入的路径作为完整路径（不再重复拼接）
	fullPath := path
	logger.Debug("WatchFile called",
		zap.String("requested_path", path),
		zap.String("full_path", fullPath))

	// 检查是否已经在监控
	if ch, exists := lm.watchedFiles[fullPath]; exists {
		logger.Debug("File already being watched", zap.String("path", fullPath))
		return ch, nil
	}

	// 初始化文件位置为文件末尾，只读取新增内容
	if _, exists := lm.filePositions[fullPath]; !exists {
		if fileInfo, err := os.Stat(fullPath); err == nil {
			lm.filePositions[fullPath] = fileInfo.Size()
			logger.Debug("初始化文件读取位置",
				zap.String("path", fullPath),
				zap.Int64("position", fileInfo.Size()))
		} else {
			// 如果获取文件信息失败，设置为 0
			lm.filePositions[fullPath] = 0
		}
	}

	// 创建更新通道
	updateCh := make(chan types.LogUpdate, 100)
	lm.watchedFiles[fullPath] = updateCh

	// 设置文件监控回调
	err := lm.fileWatcher.WatchFile(fullPath, func(event types.FileEvent) {
		logger.Debug("File event received",
			zap.String("path", fullPath),
			zap.String("event_type", event.Type))
		lm.handleFileEvent(fullPath, event, updateCh)
	})

	if err != nil {
		delete(lm.watchedFiles, fullPath)
		close(updateCh)
		logger.Error("Failed to watch file",
			zap.String("path", fullPath),
			zap.Error(err))
		return nil, fmt.Errorf("监控文件失败: %w", err)
	}

	logger.Info("Started watching file", zap.String("path", fullPath))
	return updateCh, nil
}

// handleFileEvent 处理文件事件
func (lm *LogManager) handleFileEvent(path string, event types.FileEvent, updateCh chan types.LogUpdate) {
	switch event.Type {
	case "modify":
		// 文件被修改，读取新增内容
		lm.handleFileModify(path, updateCh)
	case "delete":
		// 文件被删除
		update := types.LogUpdate{
			Path:    path,
			Entries: []types.LogEntry{},
			Type:    "delete",
		}
		select {
		case updateCh <- update:
		default:
			// 通道已满，跳过
		}
	case "create":
		// 文件被创建，读取全部内容
		lm.handleFileCreate(path, updateCh)
	}
}

// handleFileModify 处理文件修改事件
func (lm *LogManager) handleFileModify(path string, updateCh chan types.LogUpdate) {
	// 使用互斥锁保护整个读取-更新过程,保证原子性
	// 这是最简单可靠的方案,类似 tail -f 的串行处理
	lm.watchMutex.Lock()
	defer lm.watchMutex.Unlock()

	// 获取文件信息
	fileInfo, err := os.Stat(path)
	if err != nil {
		logger.Error("获取文件信息失败", zap.String("path", path), zap.Error(err))
		return
	}

	currentSize := fileInfo.Size()

	// 获取上次读取的位置
	lastPosition := lm.filePositions[path]

	// 处理文件截断或轮转的情况
	if currentSize < lastPosition {
		logger.Info("检测到文件被截断或轮转",
			zap.String("path", path),
			zap.Int64("lastPosition", lastPosition),
			zap.Int64("currentSize", currentSize))
		lastPosition = 0
		lm.filePositions[path] = 0
	}

	// 如果没有新内容，直接返回
	if currentSize <= lastPosition {
		logger.Debug("文件没有新内容",
			zap.String("path", path),
			zap.Int64("currentSize", currentSize),
			zap.Int64("lastPosition", lastPosition))
		return
	}

	// 打开文件
	file, err := os.Open(path)
	if err != nil {
		logger.Error("打开文件失败", zap.String("path", path), zap.Error(err))
		return
	}
	defer file.Close()

	// 定位到上次读取的位置
	_, err = file.Seek(lastPosition, io.SeekStart)
	if err != nil {
		logger.Error("定位文件位置失败",
			zap.String("path", path),
			zap.Int64("position", lastPosition),
			zap.Error(err))
		return
	}

	// 使用缓冲读取器读取所有新增内容
	reader := bufio.NewReader(file)
	scanner := bufio.NewScanner(reader)

	// 设置较大的缓冲区
	buf := make([]byte, 0, 64*1024) // 64KB
	scanner.Buffer(buf, 1024*1024)  // 最大 1MB 行长度

	var entries []types.LogEntry

	// 读取所有新增的行,不设置行数限制
	// 这是 tail -f 的核心行为:每次读取从 lastPosition 到 EOF 的所有内容
	for scanner.Scan() {
		line := scanner.Text()

		// 创建日志条目
		entry := types.LogEntry{
			Raw:     line,
			LineNum: -1, // 实时更新不需要行号
		}

		// 尝试解析日志行
		if parser := lm.findParser(line); parser != nil {
			if parsed, err := parser.Parse(line); err == nil {
				entry = *parsed
				entry.LineNum = -1
			}
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		logger.Error("读取文件内容失败", zap.String("path", path), zap.Error(err))
		return
	}

	// 如果没有读取到新内容，直接返回
	if len(entries) == 0 {
		logger.Debug("没有读取到新日志条目", zap.String("path", path))
		return
	}

	// 更新文件读取位置
	newPosition, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		logger.Error("获取文件当前位置失败", zap.String("path", path), zap.Error(err))
		// 即使失败也继续，使用文件大小作为新位置
		newPosition = currentSize
	}

	lm.filePositions[path] = newPosition

	// 构造更新消息
	update := types.LogUpdate{
		Path:    path,
		Entries: entries,
		Type:    "append",
	}

	logger.Debug("发送文件更新",
		zap.String("path", path),
		zap.Int("entries", len(entries)),
		zap.Int64("lastPosition", lastPosition),
		zap.Int64("newPosition", newPosition))

	// 发送更新(使用非阻塞 select,避免在锁内长时间等待)
	select {
	case updateCh <- update:
		logger.Debug("文件更新已发送", zap.String("path", path))
	default:
		logger.Warn("更新通道已满，跳过更新", zap.String("path", path))
	}
}

// handleFileCreate 处理文件创建事件
func (lm *LogManager) handleFileCreate(path string, updateCh chan types.LogUpdate) {
	content, err := lm.ReadLogFile(path, 0, 100)
	if err != nil {
		return
	}

	update := types.LogUpdate{
		Path:    path,
		Entries: content.Entries,
		Type:    "create",
	}

	select {
	case updateCh <- update:
	default:
		// 通道已满，跳过
	}
}

// Start 启动日志管理器
func (lm *LogManager) Start() error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if lm.running {
		return fmt.Errorf("日志管理器已经在运行")
	}

	// 启动内存监控器
	ctx := context.Background()
	if err := lm.memoryMonitor.Start(ctx); err != nil {
		return fmt.Errorf("启动内存监控器失败: %w", err)
	}

	// 初始化日志解析器
	lm.initializeParsers()

	lm.running = true
	return nil
}

// Stop 停止日志管理器
func (lm *LogManager) Stop() error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if !lm.running {
		return nil
	}

	// 停止内存监控器
	if err := lm.memoryMonitor.Stop(); err != nil {
		return fmt.Errorf("停止内存监控器失败: %w", err)
	}

	// 关闭文件池
	if err := lm.filePool.Close(); err != nil {
		return fmt.Errorf("关闭文件池失败: %w", err)
	}

	// 关闭所有监控通道
	lm.watchMutex.Lock()
	for path, ch := range lm.watchedFiles {
		close(ch)
		delete(lm.watchedFiles, path)
	}
	lm.watchMutex.Unlock()

	// 清空缓存
	lm.cache.Clear()

	lm.running = false
	close(lm.stopCh)

	return nil
}

// initializeParsers 初始化日志解析器
func (lm *LogManager) initializeParsers() {
	// 添加通用日志解析器
	lm.parsers["common"] = parser.NewCommonLogParser()

	// 添加JSON日志解析器
	lm.parsers["json"] = parser.NewJSONLogParser()
}

// AddParser 添加日志解析器
func (lm *LogManager) AddParser(name string, parser interfaces.LogParser) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	lm.parsers[name] = parser
}

// RemoveParser 移除日志解析器
func (lm *LogManager) RemoveParser(name string) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	delete(lm.parsers, name)
}

// GetPerformanceStats 获取性能统计信息
func (lm *LogManager) GetPerformanceStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// 内存统计
	memStats := lm.memoryMonitor.GetCurrentStats()
	stats["memory"] = memStats

	// 缓存统计
	if advancedCache, ok := lm.cache.(interfaces.AdvancedLogCache); ok {
		stats["cache"] = advancedCache.GetStats()
	}

	// 文件池统计
	stats["filePool"] = lm.filePool.GetStats()

	// 内存压力等级
	stats["memoryPressureLevel"] = lm.memoryMonitor.GetMemoryPressureLevel()

	// 优化建议
	stats["optimizationSuggestions"] = lm.memoryMonitor.OptimizeMemory()

	return stats
}

// OptimizePerformance 执行性能优化
func (lm *LogManager) OptimizePerformance() error {
	// 检查内存压力
	if lm.memoryMonitor.IsMemoryPressure() {
		// 清理缓存
		lm.cache.Clear()

		// 强制垃圾回收
		lm.memoryMonitor.ForceGC()
	}

	return nil
}

// ReadLogFileFromTail 从文件尾部读取日志内容
func (lm *LogManager) ReadLogFileFromTail(path string, lines int) (*types.LogContent, error) {
	// 检查内存压力
	if lm.memoryMonitor.IsMemoryPressure() {
		lm.cache.Clear()
	}

	// 检查文件是否存在
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}

	// 检查文件大小限制
	if info.Size() > lm.config.Server.MaxFileSize {
		return nil, fmt.Errorf("文件过大，超过限制 %d 字节", lm.config.Server.MaxFileSize)
	}

	// 检查缓存
	cacheKey := fmt.Sprintf("tail:%s:%d:%d", path, lines, info.ModTime().Unix())
	if cached, found := lm.cache.Get(cacheKey); found {
		if content, ok := cached.(*types.LogContent); ok {
			return content, nil
		}
	}

	// 使用文件池获取文件资源
	fileResource, err := lm.filePool.GetFileResource(path)
	if err != nil {
		return nil, fmt.Errorf("获取文件资源失败: %w", err)
	}
	defer lm.filePool.PutFileResource(path, fileResource)

	// 从文件尾部读取内容
	content, err := lm.readFromTailOptimized(fileResource, lines)
	if err != nil {
		return nil, fmt.Errorf("读取文件尾部内容失败: %w", err)
	}

	// 缓存结果
	if !lm.memoryMonitor.IsMemoryPressure() {
		lm.cache.Set(cacheKey, content)
	}

	return content, nil
}

// readFromTailOptimized 优化的从文件尾部读取
func (lm *LogManager) readFromTailOptimized(fileResource *pool.FileResource, lines int) (*types.LogContent, error) {
	file := fileResource.GetFile()
	reader := fileResource.GetReader()

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()

	if fileSize == 0 {
		return &types.LogContent{
			Entries:    []types.LogEntry{},
			TotalLines: 0,
			HasMore:    false,
			Offset:     0,
		}, nil
	}

	// 估算每行的平均字节数，初始假设为100字节/行
	avgLineSize := int64(100)

	// 计算初始读取位置，预留一些缓冲
	initialReadSize := int64(lines) * avgLineSize * 2
	if initialReadSize > fileSize {
		initialReadSize = fileSize
	}

	startPos := fileSize - initialReadSize
	if startPos < 0 {
		startPos = 0
	}

	// 定位到计算的起始位置
	_, err = file.Seek(startPos, io.SeekStart)
	if err != nil {
		return nil, err
	}
	reader.Reset(file)

	// 创建扫描器
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024) // 64KB缓冲区
	scanner.Buffer(buf, 1024*1024)  // 最大1MB行长度

	var allLines []string
	lineNum := int64(0)

	// 如果不是从文件开头开始读取，跳过第一行（可能是不完整的）
	if startPos > 0 && scanner.Scan() {
		// 跳过可能不完整的第一行
	}

	// 读取所有行
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 如果读取的行数不够，且不是从文件开头开始的，尝试向前读取更多
	if len(allLines) < lines && startPos > 0 {
		// 重新计算读取位置，增加读取范围
		newReadSize := initialReadSize * 2
		if newReadSize > fileSize {
			newReadSize = fileSize
		}

		newStartPos := fileSize - newReadSize
		if newStartPos < 0 {
			newStartPos = 0
		}

		if newStartPos != startPos {
			// 重新读取
			_, err = file.Seek(newStartPos, io.SeekStart)
			if err != nil {
				return nil, err
			}
			reader.Reset(file)
			scanner = bufio.NewScanner(reader)
			scanner.Buffer(buf, 1024*1024)

			allLines = []string{}

			// 如果不是从文件开头开始，跳过第一行
			if newStartPos > 0 && scanner.Scan() {
				// 跳过可能不完整的第一行
			}

			for scanner.Scan() {
				allLines = append(allLines, scanner.Text())
			}

			if err := scanner.Err(); err != nil {
				return nil, err
			}
		}
	}

	// 计算总行数（估算）
	totalLines, err := lm.countLines(file)
	if err != nil {
		// 如果计算失败，使用估算值
		totalLines = int64(len(allLines))
	}

	// 只保留最后的指定行数
	startIndex := 0
	if len(allLines) > lines {
		startIndex = len(allLines) - lines
	}

	resultLines := allLines[startIndex:]
	entries := make([]types.LogEntry, 0, len(resultLines))

	// 计算起始行号
	startLineNum := totalLines - int64(len(resultLines))
	if startLineNum < 0 {
		startLineNum = 0
	}

	// 创建日志条目
	for i, line := range resultLines {
		entry := types.LogEntry{
			Raw:     line,
			LineNum: startLineNum + int64(i),
		}

		// 尝试解析日志行
		if parser := lm.findParser(line); parser != nil {
			if parsed, err := parser.Parse(line); err == nil {
				entry = *parsed
				entry.LineNum = startLineNum + int64(i)
			}
		}

		entries = append(entries, entry)
	}

	return &types.LogContent{
		Entries:    entries,
		TotalLines: totalLines,
		HasMore:    startLineNum > 0, // 如果起始行号大于0，说明还有更多内容
		Offset:     startLineNum,
	}, nil
}

// GetLogPaths 获取配置的日志目录列表
func (lm *LogManager) GetLogPaths() []string {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	return lm.config.Server.LogPaths
}

