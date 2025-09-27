<script setup lang="ts">
import { ref, onMounted, watch, nextTick } from 'vue'
import { Search, Setting, Close, Document, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import FileBrowser from '../components/FileBrowser.vue'
import LogViewer from '../components/VirtualScrollLogViewer.vue' // 使用虚拟滚动版本
import apiService, { type LogFile, type LogEntry, type SearchResult, type SearchQuery } from '../services/api'

// Display settings interface
interface DisplaySettings {
  theme: 'system' | 'light-mode' | 'dark-mode'
  colorScheme: 'blue' | 'green' | 'purple' | 'orange' | 'gray' | 'cyan' | 'amber'
  fontSize: number
  lineHeight: number
  fontFamily: string
  showLineNumbers: boolean
  wordWrap: boolean
}

// State management
const selectedFile = ref<LogFile | null>(null)
const searchQuery = ref<string>('')
const searchResults = ref<SearchResult | null>(null)
const isSearching = ref<boolean>(false)
const isRegexMode = ref<boolean>(false)
const filterMode = ref<boolean>(false)
const useSearchResults = ref<boolean>(false) // 是否使用搜索结果代替日志文件内容
const logViewerRef = ref() // LogViewer 组件引用

// Search navigation state
const currentMatchIndex = ref<number>(0)
const totalMatches = ref<number>(0)
const hasSearchMatches = ref<boolean>(false)

// Display settings state
const displaySettings = ref<DisplaySettings>({
  theme: 'system',
  colorScheme: 'blue',
  fontSize: 14,
  lineHeight: 1.5,
  fontFamily: 'Consolas, Monaco, "Courier New", monospace',
  showLineNumbers: true,
  wordWrap: false
})

// Load display settings from localStorage
const loadDisplaySettings = () => {
  try {
    const saved = localStorage.getItem('logviewer-settings')
    if (saved) {
      const settings = JSON.parse(saved)
      if (settings.display) {
        displaySettings.value = { ...displaySettings.value, ...settings.display }
      }
    }
  } catch (error) {
    console.error('Failed to load display settings:', error)
  }
}

// Watch for settings changes in localStorage
const watchForSettingsChanges = () => {
  const checkSettings = () => {
    loadDisplaySettings()
  }

  // Check every 500ms for settings changes
  setInterval(checkSettings, 500)

  // Also listen for storage events
  window.addEventListener('storage', (e) => {
    if (e.key === 'logviewer-settings') {
      loadDisplaySettings()
    }
  })
}

// Utility function to format file size
const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const size = bytes / Math.pow(k, i)
  return (size % 1 === 0 ? size.toString() : size.toFixed(1)) + ' ' + sizes[i]
}

// Event handlers
const handleFileSelect = (file: LogFile) => {
  selectedFile.value = file
  console.log('Selected file:', file)
}

// Search functionality
const performSearch = async () => {
  if (!selectedFile.value || !searchQuery.value.trim()) {
    return
  }

  isSearching.value = true

  try {
    const query: SearchQuery = {
      path: selectedFile.value.path,
      query: searchQuery.value.trim(),
      isRegex: isRegexMode.value,
      offset: 0,
      limit: 1000 // 增加限制以获取更多结果
    }

    const result = await apiService.searchLogs(query)
    searchResults.value = result
    console.log('Search results:', result)

    // 当在过滤模式下搜索时，使用搜索结果
    if (filterMode.value) {
      useSearchResults.value = true
    }

    // 延迟更新前端匹配计数，确保DOM已更新
    nextTick(() => {
      setTimeout(() => {
        updateFrontendMatchCounts()
      }, 100)
    })

  } catch (error) {
    console.error('Search failed:', error)
    // 搜索失败时重置计数
    totalMatches.value = 0
    currentMatchIndex.value = 0
    hasSearchMatches.value = false
  } finally {
    isSearching.value = false
  }
}

// 计算前端实际匹配项数量
const updateFrontendMatchCounts = () => {
  if (!searchQuery.value.trim()) {
    totalMatches.value = 0
    currentMatchIndex.value = 0
    hasSearchMatches.value = false
    return
  }

  // 等待下一个tick让DOM更新完成，然后计算高亮的匹配项
  nextTick(() => {
    const highlightedElements = document.querySelectorAll('mark')
    const matchCount = highlightedElements.length

    totalMatches.value = matchCount
    currentMatchIndex.value = 0
    hasSearchMatches.value = matchCount > 0

    console.log(`Found ${matchCount} matches in frontend`)
  })
}

const clearSearch = () => {
  searchQuery.value = ''
  searchResults.value = null
  filterMode.value = false
  useSearchResults.value = false
  // 重置搜索导航状态
  totalMatches.value = 0
  currentMatchIndex.value = 0
  hasSearchMatches.value = false
}

const handleEntrySelect = (entry: LogEntry) => {
  console.log('Selected entry from search:', entry)
  // TODO: Highlight entry in log viewer
}

const handleLogEntryClick = (entry: LogEntry) => {
  console.log('Log entry clicked:', entry)
  // TODO: Show log entry details
}

const handleSearchKeyup = (event: KeyboardEvent) => {
  if (event.key === 'Enter') {
    performSearch()
  }
}

// 搜索导航方法
const navigateToPreviousMatch = () => {
  if (hasSearchMatches.value && currentMatchIndex.value > 0) {
    currentMatchIndex.value--
    scrollToMatch(currentMatchIndex.value)
  }
}

const navigateToNextMatch = () => {
  if (hasSearchMatches.value && currentMatchIndex.value < totalMatches.value - 1) {
    currentMatchIndex.value++
    scrollToMatch(currentMatchIndex.value)
  }
}

const scrollToMatch = (matchIndex: number) => {
  // 获取所有高亮的匹配项
  const highlightedElements = document.querySelectorAll('mark')

  if (matchIndex < 0 || matchIndex >= highlightedElements.length) {
    return
  }

  // 滚动到指定的匹配项
  const targetElement = highlightedElements[matchIndex]
  if (targetElement) {
    // 找到包含这个mark元素的整个日志行
    const logEntry = targetElement.closest('.log-entry')

    if (logEntry) {
      // 清除所有之前的高亮
      document.querySelectorAll('.log-entry.search-highlight-active').forEach(el => {
        el.classList.remove('search-highlight-active')
      })

      // 高亮当前行
      logEntry.classList.add('search-highlight-active')

      // 滚动到当前行
      logEntry.scrollIntoView({
        behavior: 'smooth',
        block: 'center',
        inline: 'nearest'
      })

      // 2秒后移除高亮
      setTimeout(() => {
        logEntry.classList.remove('search-highlight-active')
      }, 2000)
    } else {
      // 如果找不到日志行，回退到原来的方式
      targetElement.scrollIntoView({
        behavior: 'smooth',
        block: 'center',
        inline: 'nearest'
      })
    }

    console.log(`Scrolled to match ${matchIndex + 1} of ${highlightedElements.length}`)
  }
}

// 监听过滤模式变化
watch(filterMode, (newFilterMode) => {
  if (newFilterMode && searchQuery.value.trim()) {
    // 启用过滤模式且有搜索条件时，执行搜索并使用搜索结果
    performSearch()
  } else if (!newFilterMode) {
    // 关闭过滤模式时，回到正常的日志查看模式
    useSearchResults.value = false
  }
})

// 监听搜索条件变化
watch(searchQuery, (newQuery) => {
  if (!newQuery.trim()) {
    // 清空搜索条件时，回到正常模式
    useSearchResults.value = false
    searchResults.value = null
    // 重置搜索导航状态
    totalMatches.value = 0
    currentMatchIndex.value = 0
    hasSearchMatches.value = false
  } else {
    // 有搜索条件时，延迟计算匹配项（等待DOM更新）
    nextTick(() => {
      setTimeout(() => {
        updateFrontendMatchCounts()
      }, 100) // 给一点时间让搜索高亮渲染完成
    })
  }
})

onMounted(() => {
  // Component-specific initialization if needed
  loadDisplaySettings()
  watchForSettingsChanges()
})
</script>

<template>
  <el-container class="log-viewer-container">
    <!-- File Browser Sidebar -->
    <el-aside width="320px" class="file-browser-sidebar">
      <FileBrowser @file-select="handleFileSelect" />
    </el-aside>

    <!-- Main Content Area -->
    <el-container>
      <!-- Enhanced Toolbar -->
      <el-header height="auto" class="toolbar elevated-surface">
        <div class="toolbar-content">
          <!-- File Information and Search Section (Single Row) -->
          <div class="main-toolbar-row" v-if="selectedFile">
            <!-- File Information -->
            <div class="file-info-compact">
              <div class="file-icon">
                <el-icon><Document /></el-icon>
              </div>
              <div class="file-details">
                <h4 class="file-name">{{ selectedFile.name }}</h4>
                <p class="file-path">{{ selectedFile.path }}</p>
              </div>
            </div>

            <!-- Search Controls -->
            <div class="search-section">
              <div class="search-wrapper">
                <el-input
                  v-model="searchQuery"
                  placeholder="输入搜索关键词..."
                  class="search-input"
                  size="default"
                  :disabled="!selectedFile || isSearching"
                  @keyup="handleSearchKeyup"
                  clearable
                  @clear="clearSearch"
                  :loading="isSearching"
                >
                  <template #prefix>
                    <el-icon class="search-icon">
                      <Search />
                    </el-icon>
                  </template>
                </el-input>

                <!-- Search Navigation -->
                <div class="search-navigation" v-if="searchQuery.trim()">
                  <el-button-group class="nav-buttons">
                    <el-button
                      size="small"
                      :disabled="!hasSearchMatches || currentMatchIndex <= 0"
                      @click="navigateToPreviousMatch"
                      title="上一个匹配项"
                    >
                      <el-icon><ArrowUp /></el-icon>
                    </el-button>
                    <el-button
                      size="small"
                      :disabled="!hasSearchMatches || currentMatchIndex >= totalMatches - 1"
                      @click="navigateToNextMatch"
                      title="下一个匹配项"
                    >
                      <el-icon><ArrowDown /></el-icon>
                    </el-button>
                  </el-button-group>

                  <span class="match-counter" v-if="hasSearchMatches">
                    第 {{ currentMatchIndex + 1 }} 个，共 {{ totalMatches }} 个匹配
                  </span>
                  <span class="match-counter" v-else-if="searchQuery.trim() && !isSearching">
                    无匹配项
                  </span>
                </div>

                <div class="search-controls">
                  <el-checkbox
                    v-model="isRegexMode"
                    :disabled="!selectedFile || isSearching"
                    class="regex-toggle"
                  >
                    正则
                  </el-checkbox>

                  <el-checkbox
                    v-model="filterMode"
                    :disabled="!selectedFile || isSearching"
                    class="filter-toggle"
                  >
                    过滤
                  </el-checkbox>
                </div>
              </div>
            </div>
          </div>

          <!-- Empty State -->
          <div v-else class="empty-state">
            <p class="empty-text">请从左侧选择一个日志文件</p>
          </div>
        </div>
      </el-header>

      <!-- Content Area -->
      <el-main class="main-content">
        <div class="content-wrapper">
          <LogViewer
            ref="logViewerRef"
            :file="selectedFile"
            :search-query="searchQuery.trim()"
            :search-highlight="true"
            :filter-mode="filterMode"
            :is-regex="isRegexMode"
            :display-settings="displaySettings"
            @entry-click="handleLogEntryClick"
          />
        </div>
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.log-viewer-container {
  height: 100%;
  background: var(--app-bg-color);
  gap: var(--space-md);
  padding: var(--space-md);
}

.file-browser-sidebar {
  background: transparent;
  border: none;
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.toolbar {
  background: var(--app-bg-secondary);
  border: 1px solid var(--app-border-color);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  backdrop-filter: var(--app-backdrop-blur);
  -webkit-backdrop-filter: var(--app-backdrop-blur);
  padding: var(--space-md) var(--space-lg);
  margin-bottom: var(--space-md);
  transition: all var(--transition-normal);
  height: auto !important;
  min-height: 60px;
}

.toolbar:hover {
  box-shadow: var(--shadow-md);
  border-color: var(--app-border-hover);
}

.toolbar-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  width: 100%;
}

.main-toolbar-row {
  display: flex;
  align-items: center;
  gap: var(--space-lg);
  width: 100%;
}

.file-info-compact {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-shrink: 0;
}

.file-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--primary-500), var(--primary-600));
  border-radius: var(--radius-lg);
  color: white;
  box-shadow: var(--shadow-sm);
  transition: all var(--transition-normal);
}

.file-icon:hover {
  transform: translateY(-1px);
  box-shadow: var(--shadow-md);
}

.file-details {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
}

.file-name {
  margin: 0;
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
  color: var(--app-text-color);
  line-height: var(--line-height-tight);
  white-space: nowrap;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-path {
  margin: 0;
  font-size: var(--font-size-xs);
  color: var(--app-text-muted);
  font-family: var(--log-font-family);
  line-height: var(--line-height-normal);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.search-section {
  flex: 1;
  display: flex;
  flex-direction: row-reverse;
  gap: var(--space-sm);
}

.search-wrapper {
  display: flex;
  align-items: center;
  gap: var(--space-md);
}

.search-input {
  flex: 1;
  max-width: 300px;
}

:deep(.search-input .el-input__wrapper) {
  background: var(--app-bg-tertiary);
  border: 1px solid var(--app-border-color);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-xs);
  transition: all var(--transition-normal);
}

:deep(.search-input .el-input__wrapper:hover) {
  border-color: var(--primary-300);
  box-shadow: var(--shadow-sm);
}

:deep(.search-input .el-input__wrapper.is-focus) {
  border-color: var(--primary-500);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.search-icon {
  color: var(--app-text-muted);
}

.search-controls {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.regex-toggle,
.filter-toggle {
  background: var(--app-bg-tertiary);
  padding: var(--space-xs) var(--space-sm);
  border-radius: var(--radius-md);
  border: 1px solid var(--app-border-color);
  transition: all var(--transition-normal);
  margin-right: 0;
}

.regex-toggle:hover,
.filter-toggle:hover {
  background: var(--app-bg-secondary);
  border-color: var(--app-border-hover);
}

.mode-toggle {
  border-radius: var(--radius-md);
  font-weight: var(--font-weight-medium);
  font-size: var(--font-size-sm);
  padding: var(--space-xs) var(--space-sm);
  transition: all var(--transition-normal);
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-lg);
}

.empty-text {
  margin: 0;
  font-size: var(--font-size-sm);
  color: var(--app-text-muted);
  font-style: italic;
}

.main-content {
  padding: 0;
  background: transparent;
  overflow: hidden;
}

.content-wrapper {
  height: 100%;
  background: var(--app-bg-secondary);
  border: 1px solid var(--app-border-color);
  border-radius: var(--radius-lg);
  overflow: hidden;
  box-shadow: var(--shadow-sm);
  transition: all var(--transition-normal);
}

.content-wrapper:hover {
  box-shadow: var(--shadow-md);
  border-color: var(--app-border-hover);
}

/* Responsive Design */
@media (max-width: 1200px) {
  .search-input {
    max-width: 300px;
  }
}

@media (max-width: 768px) {
  .log-viewer-container {
    gap: var(--space-sm);
    padding: var(--space-sm);
  }

  .file-browser-sidebar {
    width: 280px !important;
  }

  .toolbar {
    padding: var(--space-md);
  }

  .toolbar-content {
    gap: var(--space-md);
  }

  .file-header {
    gap: var(--space-sm);
  }

  .file-icon {
    width: 40px;
    height: 40px;
  }

  .file-name {
    font-size: var(--font-size-base);
  }

  .search-wrapper {
    flex-direction: column;
    align-items: stretch;
    gap: var(--space-sm);
  }

  .search-input {
    max-width: none;
  }

  .search-controls {
    justify-content: space-between;
  }

  .control-group {
    justify-content: center;
  }
}

@media (max-width: 480px) {
  .file-browser-sidebar {
    width: 240px !important;
  }

  .file-meta {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-xs);
  }

  .toolbar {
    min-height: auto;
  }

  .actions-section {
    gap: var(--space-sm);
  }
}

/* Animation Classes */
.slide-in {
  animation: slideIn var(--transition-normal) ease-out;
}

.fade-in {
  animation: fadeIn var(--transition-normal) ease-out;
}

/* Search Navigation Highlight */
:deep(.log-entry.search-highlight-active) {
  background-color: rgba(255, 235, 59, 0.3) !important;
  border-left: 3px solid #ffeb3b !important;
  box-shadow: 0 0 10px rgba(255, 235, 59, 0.5);
  transform: scale(1.01);
  transition: all 0.3s ease;
  z-index: 10;
  position: relative;
}

/* Search navigation buttons */
.search-navigation {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.nav-buttons {
  display: flex;
  background: var(--app-bg-tertiary);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.nav-buttons .el-button {
  border-radius: 0;
  border: none;
  background: transparent;
  min-width: 32px;
  height: 28px;
}

.nav-buttons .el-button:hover {
  background: var(--primary-100);
}

.nav-buttons .el-button:first-child {
  border-right: 1px solid var(--app-border-color);
}

.match-counter {
  font-size: var(--font-size-xs);
  color: var(--app-text-muted);
  white-space: nowrap;
  margin-left: var(--space-sm);
}
</style>