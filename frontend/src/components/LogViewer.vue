<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { ElButton, ElIcon, ElTag, ElTooltip, ElMessage } from 'element-plus'
import { VideoPlay, VideoPause, Refresh, ArrowUp, ArrowDown, Document } from '@element-plus/icons-vue'
import apiService, { type LogFile, type LogEntry, type LogContent, type SearchResult, type SearchQuery } from '../services/api'
import wsService, { type LogUpdate } from '../services/websocket'
import { LogFormattingManager } from '../services/logFormatting'

// Props
interface Props {
  file: LogFile | null
  searchQuery?: string
  searchHighlight?: boolean
  filterMode?: boolean
  isRegex?: boolean
  displaySettings?: {
    theme: 'system' | 'light-mode' | 'dark-mode'
    colorScheme: 'blue' | 'green' | 'purple' | 'orange' | 'gray' | 'cyan' | 'amber'
    fontSize: number
    lineHeight: number
    fontFamily: string
    showLineNumbers: boolean
    wordWrap: boolean
  }
}

const props = withDefaults(defineProps<Props>(), {
  searchQuery: '',
  searchHighlight: true,
  filterMode: false,
  isRegex: false,
  displaySettings: () => ({
    theme: 'system',
    colorScheme: 'blue',
    fontSize: 14,
    lineHeight: 1.5,
    fontFamily: 'Consolas, Monaco, "Courier New", monospace',
    showLineNumbers: true,
    wordWrap: false
  })
})

// Emits
const emit = defineEmits<{
  entryClick: [entry: LogEntry]
}>()

// Reactive state
const logEntries = ref<LogEntry[]>([])
const loading = ref(false)
const realTimeMode = ref(false)
const currentOffset = ref(0)
const totalLines = ref(0)
const hasMore = ref(false)
const containerRef = ref<HTMLElement>()
const isUserScrolling = ref(false)
const hoveredEntryIndex = ref(-1)
const formattedEntries = ref<Set<number>>(new Set())
const pendingNewLogs = ref<LogEntry[]>([])
const hasNewLogsAvailable = ref(false)
const internalSearchResults = ref<SearchResult | null>(null)

// 格式化管理器
const formattingManager = new LogFormattingManager()

// Performance settings
const getPerformanceSettings = () => {
  try {
    const saved = localStorage.getItem('logviewer-settings')
    if (saved) {
      const settings = JSON.parse(saved)
      return settings.performance || {}
    }
  } catch (error) {
    console.warn('Failed to load performance settings:', error)
  }
  return {
    maxLogLines: 10000,
    preloadLines: 100
  }
}

const performanceSettings = ref(getPerformanceSettings())

// Settings-based computed properties (reactive)
const showLineNumbers = computed(() => props.displaySettings?.showLineNumbers ?? true)

// Apply display settings via CSS variables
const applyDisplaySettings = () => {
  if (!props.displaySettings) return

  const root = document.documentElement
  const settings = props.displaySettings

  // Apply CSS variables for log display
  root.style.setProperty('--log-font-size', `${settings.fontSize}px`)
  root.style.setProperty('--log-line-height', settings.lineHeight.toString())
  root.style.setProperty('--log-font-family', settings.fontFamily)
  root.style.setProperty('--show-line-numbers', settings.showLineNumbers ? 'block' : 'none')
  root.style.setProperty('--word-wrap', settings.wordWrap ? 'break-word' : 'normal')

  console.log('Applied display settings:', {
    fontSize: settings.fontSize,
    lineHeight: settings.lineHeight,
    fontFamily: settings.fontFamily,
    showLineNumbers: settings.showLineNumbers,
    wordWrap: settings.wordWrap
  })
}

// Watch for display settings changes
watch(() => props.displaySettings, () => {
  applyDisplaySettings()
}, { deep: true, immediate: true })

// Format timestamp for display
const formatTimestamp = (timestamp: string): string => {
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// Check if string is valid JSON
const isValidJson = (str: string): boolean => {
  if (!str || typeof str !== 'string') return false

  // Trim whitespace
  str = str.trim()

  // Check if it starts and ends with JSON delimiters
  if (!(
    (str.startsWith('{') && str.endsWith('}')) ||
    (str.startsWith('[') && str.endsWith(']'))
  )) {
    return false
  }

  try {
    JSON.parse(str)
    return true
  } catch {
    return false
  }
}

// Format JSON string with indentation
const formatJson = (jsonStr: string): string => {
  try {
    const parsed = JSON.parse(jsonStr)
    return JSON.stringify(parsed, null, 2)
  } catch (error) {
    return jsonStr
  }
}

// Add basic syntax highlighting to JSON
const highlightJson = (jsonStr: string): string => {
  return jsonStr
    .replace(/"([^"]+)":/g, '<span class="json-key">"$1":</span>')
    .replace(/:\s*"([^"]*)"/g, ': <span class="json-string">"$1"</span>')
    .replace(/:\s*(-?\d+\.?\d*)/g, ': <span class="json-number">$1</span>')
    .replace(/:\s*(true|false)/g, ': <span class="json-boolean">$1</span>')
    .replace(/:\s*(null)/g, ': <span class="json-null">$1</span>')
}

// 切换格式化显示
const toggleFormatting = (index: number) => {
  const newSet = new Set(formattedEntries.value)
  if (newSet.has(index)) {
    newSet.delete(index)
  } else {
    newSet.add(index)
  }
  formattedEntries.value = newSet
}

// 检查是否应该显示格式化按钮
const shouldShowFormattingButton = (entry: LogEntry): boolean => {
  return formattingManager.shouldShowFormattingButton(entry)
}

// 获取格式化后的内容
const getFormattedContent = (entry: LogEntry, index: number): string => {
  let content: string

  if (!formattedEntries.value.has(index)) {
    // 显示原文
    content = entry.raw
  } else {
    // 显示格式化内容
    const result = formattingManager.formatLog(entry)
    content = result.content
  }

  // 应用搜索高亮
  if (props.searchQuery && props.searchHighlight) {
    return highlightText(content, props.searchQuery)
  }

  return content
}

// 获取显示类型（用于CSS样式）
const getDisplayType = (entry: LogEntry, index: number): string => {
  if (!formattedEntries.value.has(index)) {
    return 'raw-text'
  }

  const result = formattingManager.formatLog(entry)
  return result.displayType
}

// Get log level color
const getLogLevelColor = (level: string): string => {
  const levelColors: Record<string, string> = {
    ERROR: '#f56c6c',
    WARN: '#e6a23c',
    WARNING: '#e6a23c',
    INFO: '#409eff',
    DEBUG: '#909399',
    TRACE: '#c0c4cc'
  }
  return levelColors[level.toUpperCase()] || '#909399'
}

// Check if log entry matches search query
const isLogEntryMatch = (entry: LogEntry, query: string): boolean => {
  if (!query) return true

  const searchContent = [
    entry.message,
    ...(entry.fields ? Object.entries(entry.fields).map(([key, value]) => `${key}: ${JSON.stringify(value)}`) : [])
  ]

  if (props.isRegex) {
    try {
      const regex = new RegExp(query, 'gi')
      return searchContent.some(content => regex.test(content))
    } catch (error) {
      // If regex is invalid, fallback to simple string match
      console.warn('Invalid regex pattern, falling back to simple search:', error)
      const lowerQuery = query.toLowerCase()
      return searchContent.some(content => content.toLowerCase().includes(lowerQuery))
    }
  } else {
    // Simple string search (case insensitive)
    const lowerQuery = query.toLowerCase()
    return searchContent.some(content => content.toLowerCase().includes(lowerQuery))
  }
}

// Highlight search terms in text
const highlightText = (text: string, query: string): string => {
  if (!query || !props.searchHighlight) return text

  if (props.isRegex) {
    try {
      const regex = new RegExp(`(${query})`, 'gi')
      return text.replace(regex, '<mark class="search-highlight">$1</mark>')
    } catch (error) {
      // If regex is invalid, fallback to simple string replacement
      console.warn('Invalid regex pattern for highlighting, falling back to simple search:', error)
      const escapedQuery = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
      const regex = new RegExp(`(${escapedQuery})`, 'gi')
      return text.replace(regex, '<mark class="search-highlight">$1</mark>')
    }
  } else {
    // Simple string replacement (escape regex special characters)
    const escapedQuery = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const regex = new RegExp(`(${escapedQuery})`, 'gi')
    return text.replace(regex, '<mark class="search-highlight">$1</mark>')
  }
}

// Filter log entries based on search query and filter mode
const filteredLogEntries = computed(() => {
  if (!props.filterMode || !props.searchQuery) {
    return logEntries.value
  }

  return logEntries.value.filter(entry => isLogEntryMatch(entry, props.searchQuery))
})

// Execute search when in filter mode
const executeSearch = async () => {
  if (!props.file || !props.searchQuery.trim()) {
    internalSearchResults.value = null
    return
  }

  try {
    const searchQuery: SearchQuery = {
      path: props.file.path,
      query: props.searchQuery.trim(),
      isRegex: props.isRegex,
      offset: 0,
      limit: 50000 // 大幅增加搜索范围以覆盖更多内容
    }

    const result = await apiService.searchLogs(searchQuery)
    internalSearchResults.value = result
    console.log('Internal search completed:', result)

    // 如果搜索结果达到限制且hasMore为true，提示用户可能有更多结果
    if (result.hasMore && result.entries.length >= 50000) {
      console.warn('Search results may be incomplete due to large result set')
    }
  } catch (error) {
    console.error('Internal search failed:', error)
    internalSearchResults.value = null
  }
}

// Get display entries (search results, filtered, or all)
const displayEntries = computed(() => {
  // 如果在过滤模式下有内部搜索结果，使用内部搜索结果
  if (props.filterMode && internalSearchResults.value && internalSearchResults.value.entries.length > 0) {
    return internalSearchResults.value.entries
  }

  // 否则使用过滤逻辑或原始日志
  return props.filterMode ? filteredLogEntries.value : logEntries.value
})

// Apply performance limits
const enforceLogLimits = () => {
  const maxLines = performanceSettings.value.maxLogLines || 10000
  if (logEntries.value.length > maxLines) {
    const excess = logEntries.value.length - maxLines
    logEntries.value.splice(0, excess)
    console.log(`Applied line limit: removed ${excess} entries, current: ${logEntries.value.length}`)
  }
}

// Load log content
const loadLogContent = async (offset = 0, limit?: number, append = false) => {
  if (!props.file || loading.value) return

  const actualLimit = limit || performanceSettings.value.preloadLines || 100
  loading.value = true

  try {
    const content: LogContent = await apiService.getLogContent(props.file.path, offset, actualLimit)

    if (append) {
      logEntries.value.push(...content.entries)
    } else {
      logEntries.value = content.entries
      currentOffset.value = 0
    }

    totalLines.value = content.totalLines
    hasMore.value = content.hasMore
    currentOffset.value = content.offset + content.entries.length

    // Apply performance limits
    enforceLogLimits()

  } catch (error) {
    console.error('Failed to load log content:', error)
    ElMessage.error('加载日志内容失败')
  } finally {
    loading.value = false
  }
}

// Load more content
const loadMore = async () => {
  if (!hasMore.value || loading.value) return
  await loadLogContent(currentOffset.value, undefined, true)
}

// Refresh log content
const refresh = async () => {
  if (!props.file) return
  await loadLogContent(0, Math.max(performanceSettings.value.preloadLines || 100, logEntries.value.length))
  if (realTimeMode.value) {
    nextTick(() => {
      scrollToBottom()
    })
  }
}

// Toggle real-time mode
const toggleRealTimeMode = () => {
  realTimeMode.value = !realTimeMode.value
  if (realTimeMode.value) {
    nextTick(() => {
      scrollToBottom()
    })
  }
}

// Scroll to top
const scrollToTop = () => {
  if (containerRef.value) {
    containerRef.value.scrollTop = 0
    isUserScrolling.value = false
  }
}

// Scroll to bottom - 使用真实 scrollHeight 确保准确定位
const scrollToBottom = () => {
  if (!containerRef.value) return

  isUserScrolling.value = false
  const container = containerRef.value

  // 等待DOM渲染完成后再执行滚动
  const performScroll = () => {
    const maxScrollTop = container.scrollHeight - container.clientHeight

    console.log('Scrolling to bottom:', {
      scrollHeight: container.scrollHeight,
      clientHeight: container.clientHeight,
      maxScrollTop,
      currentScrollTop: container.scrollTop
    })

    container.scrollTop = maxScrollTop

    // 验证滚动位置（100ms后检查）
    setTimeout(() => {
      const finalMaxScrollTop = container.scrollHeight - container.clientHeight
      if (Math.abs(container.scrollTop - finalMaxScrollTop) > 10) {
        console.log('Scroll position correction needed:', {
          current: container.scrollTop,
          expected: finalMaxScrollTop
        })
        container.scrollTop = finalMaxScrollTop
      } else {
        console.log('Scroll position verified: at bottom ✓')
      }
    }, 100)
  }

  nextTick(() => {
    requestAnimationFrame(performScroll)
  })
}

// Handle scroll event
const handleScroll = (event: Event) => {
  const target = event.target as HTMLElement

  // Check if user is scrolling manually
  const isAtBottom = target.scrollTop >= target.scrollHeight - target.clientHeight - 50

  if (!isAtBottom) {
    isUserScrolling.value = true
  } else if (realTimeMode.value) {
    isUserScrolling.value = false
  }

  // Load more content when near bottom
  const scrollBottom = target.scrollTop + target.clientHeight
  const threshold = target.scrollHeight - 200

  if (scrollBottom >= threshold && hasMore.value && !loading.value) {
    loadMore()
  }
}

// Handle log entry click
const handleEntryClick = (entry: LogEntry) => {
  emit('entryClick', entry)
}

// Apply pending new logs
const applyPendingLogs = () => {
  if (pendingNewLogs.value.length > 0) {
    // 在过滤模式下，需要检查是否已存在于搜索结果中
    if (props.filterMode && props.searchQuery && internalSearchResults.value) {
      // 过滤掉已存在于搜索结果中的重复项
      const newUniqueEntries = pendingNewLogs.value.filter(pendingEntry => {
        // 检查是否已存在于当前显示的搜索结果中
        const existsInSearchResults = internalSearchResults.value?.entries.some(existingEntry =>
          existingEntry.lineNum === pendingEntry.lineNum &&
          existingEntry.message === pendingEntry.message
        ) || false

        // 检查是否已存在于主日志列表中
        const existsInMainLogs = logEntries.value.some(existingEntry =>
          existingEntry.lineNum === pendingEntry.lineNum &&
          existingEntry.message === pendingEntry.message
        )

        return !existsInSearchResults && !existsInMainLogs
      })

      if (newUniqueEntries.length > 0) {
        logEntries.value.push(...newUniqueEntries)
        totalLines.value += newUniqueEntries.length
        console.log(`过滤模式：已应用 ${newUniqueEntries.length} 条待处理的新日志（原 ${pendingNewLogs.value.length} 条，去重后 ${newUniqueEntries.length} 条）`)
      } else {
        console.log(`过滤模式：所有 ${pendingNewLogs.value.length} 条待处理日志都是重复的，已跳过`)
      }
    } else {
      // 正常模式下，也进行去重检查
      const newUniqueEntries = pendingNewLogs.value.filter(pendingEntry =>
        !logEntries.value.some(existingEntry =>
          existingEntry.lineNum === pendingEntry.lineNum &&
          existingEntry.message === pendingEntry.message
        )
      )

      if (newUniqueEntries.length > 0) {
        logEntries.value.push(...newUniqueEntries)
        totalLines.value += newUniqueEntries.length
        console.log(`正常模式：已应用 ${newUniqueEntries.length} 条待处理的新日志（原 ${pendingNewLogs.value.length} 条，去重后 ${newUniqueEntries.length} 条）`)
      } else {
        console.log(`正常模式：所有 ${pendingNewLogs.value.length} 条待处理日志都是重复的，已跳过`)
      }
    }

    // Apply limits
    enforceLogLimits()

    // Clear pending logs
    pendingNewLogs.value = []
    hasNewLogsAvailable.value = false

    // Auto-scroll if in real-time mode and user isn't manually scrolling
    if (realTimeMode.value && !isUserScrolling.value) {
      nextTick(() => {
        scrollToBottom()
      })
    }
  }
}

// Handle WebSocket log updates
const handleLogUpdate = (update: LogUpdate) => {
  if (!props.file) return

  // Check if this update is for the current file
  const currentPath = props.file.path
  const currentFilename = currentPath.split('/').pop() || currentPath
  const updatePath = update.path
  const updateFilename = updatePath.split('/').pop() || updatePath

  if (updatePath !== currentPath && updateFilename !== currentFilename) {
    return
  }

  if (update.type === 'append') {
    // 如果在过滤模式下且有搜索条件，进行客户端实时过滤
    if (props.filterMode && props.searchQuery && internalSearchResults.value) {
      // 创建去重函数：检查条目是否已存在于搜索结果中
      const isDuplicate = (newEntry: LogEntry): boolean => {
        return internalSearchResults.value?.entries.some(existingEntry =>
          existingEntry.lineNum === newEntry.lineNum &&
          existingEntry.message === newEntry.message
        ) || false
      }

      // 检查新日志是否匹配搜索条件，并过滤掉重复项
      const matchingEntries = update.entries.filter(entry =>
        isLogEntryMatch(entry, props.searchQuery) && !isDuplicate(entry)
      )

      const nonMatchingEntries = update.entries.filter(entry =>
        !isLogEntryMatch(entry, props.searchQuery)
      )

      if (matchingEntries.length > 0 && realTimeMode.value) {
        // 只有当实时模式启用时，匹配的日志才直接添加到搜索结果中
        internalSearchResults.value.entries.push(...matchingEntries)
        internalSearchResults.value.totalMatches += matchingEntries.length
        console.log(`实时模式：自动添加 ${matchingEntries.length} 条匹配的新日志到搜索结果（已去重）`)

        // 在实时模式下自动滚动到底部
        if (!isUserScrolling.value) {
          nextTick(() => {
            scrollToBottom()
          })
        }
      } else if (matchingEntries.length > 0 && !realTimeMode.value) {
        // 实时模式关闭时，匹配的日志也放入待处理队列（去重检查）
        const newMatchingEntries = matchingEntries.filter(entry =>
          !pendingNewLogs.value.some(pendingEntry =>
            pendingEntry.lineNum === entry.lineNum && pendingEntry.message === entry.message
          )
        )
        if (newMatchingEntries.length > 0) {
          pendingNewLogs.value.push(...newMatchingEntries)
          hasNewLogsAvailable.value = true
          console.log(`实时模式已关闭，${newMatchingEntries.length} 条匹配的新日志放入待处理队列（已去重）`)
        }
      }

      // 对于非匹配的日志，也进行去重检查
      if (nonMatchingEntries.length > 0) {
        const newNonMatchingEntries = nonMatchingEntries.filter(entry =>
          !pendingNewLogs.value.some(pendingEntry =>
            pendingEntry.lineNum === entry.lineNum && pendingEntry.message === entry.message
          )
        )
        if (newNonMatchingEntries.length > 0) {
          pendingNewLogs.value.push(...newNonMatchingEntries)
          hasNewLogsAvailable.value = true
          console.log(`${newNonMatchingEntries.length} 条不匹配的新日志放入待处理队列（已去重）`)
        }
      }
    } else {
      // 正常模式下直接添加新日志（添加去重检查）
      const newEntries = update.entries.filter(entry =>
        !logEntries.value.some(existingEntry =>
          existingEntry.lineNum === entry.lineNum && existingEntry.message === entry.message
        )
      )

      if (newEntries.length > 0) {
        logEntries.value.push(...newEntries)
        totalLines.value += newEntries.length
        console.log(`正常模式：添加 ${newEntries.length} 条新日志（已去重）`)

        // Apply limits
        enforceLogLimits()

        // Auto-scroll if in real-time mode and user isn't manually scrolling
        if (realTimeMode.value && !isUserScrolling.value) {
          nextTick(() => {
            scrollToBottom()
          })
        }
      } else {
        console.log('正常模式：所有日志条目都是重复的，已跳过')
      }
    }
  } else if (update.type === 'truncate') {
    refresh()
  }
}

// Subscribe to file updates
const subscribeToFile = (filePath: string) => {
  console.log('Subscribing to file:', filePath)

  const attemptSubscribe = () => {
    if (wsService.isConnected) {
      wsService.send({
        type: 'subscribe',
        path: filePath
      })
    } else {
      setTimeout(attemptSubscribe, 100)
    }
  }

  attemptSubscribe()
}

// Unsubscribe from file updates
const unsubscribeFromFile = (filePath: string) => {
  wsService.send({
    type: 'unsubscribe',
    path: filePath
  })
}

// Watch for file changes
watch(() => props.file, (newFile, oldFile) => {
  if (oldFile?.path) {
    unsubscribeFromFile(oldFile.path)
  }

  if (newFile) {
    realTimeMode.value = false
    isUserScrolling.value = false
    formattedEntries.value = new Set() // Clear formatted state
    pendingNewLogs.value = [] // Clear pending logs
    hasNewLogsAvailable.value = false // Reset new logs indicator
    loadLogContent()

    setTimeout(() => {
      subscribeToFile(newFile.path)
    }, 500)
  } else {
    logEntries.value = []
    currentOffset.value = 0
    totalLines.value = 0
    hasMore.value = false
    formattedEntries.value = new Set() // Clear formatted state
    pendingNewLogs.value = [] // Clear pending logs
    hasNewLogsAvailable.value = false // Reset new logs indicator
  }
}, { immediate: true })

// Watch for filter mode and search query changes
watch(() => [props.filterMode, props.searchQuery], async ([newFilterMode, newSearchQuery]) => {
  if (newFilterMode && newSearchQuery) {
    // 启用过滤模式且有搜索条件时，执行搜索
    console.log('Filter mode activated, executing search...')
    await executeSearch()
  } else {
    // 清空搜索结果
    internalSearchResults.value = null
  }
})

// Watch for filter mode changes
watch(() => [props.filterMode, props.searchQuery], ([newFilterMode, newSearchQuery], [oldFilterMode, oldSearchQuery]) => {
  // 当从过滤模式切换到非过滤模式时，应用待处理的日志
  if (oldFilterMode && !newFilterMode && pendingNewLogs.value.length > 0) {
    console.log('退出过滤模式，应用待处理的日志')
    applyPendingLogs()
  }

  // 当搜索条件清空时，也应用待处理的日志
  if (oldSearchQuery && !newSearchQuery && pendingNewLogs.value.length > 0) {
    console.log('清空搜索条件，应用待处理的日志')
    applyPendingLogs()
  }
})

// Watch for settings changes
watch(() => {
  try {
    const saved = localStorage.getItem('logviewer-settings')
    return saved ? JSON.parse(saved).performance : null
  } catch {
    return null
  }
}, (newSettings) => {
  if (newSettings) {
    performanceSettings.value = { ...newSettings }
    enforceLogLimits()
  }
})

// Lifecycle hooks
onMounted(() => {
  wsService.on('log_update', handleLogUpdate)
  applyDisplaySettings()
})

// Scroll to specific search match
const scrollToMatch = (matchIndex: number) => {
  if (!containerRef.value || !internalSearchResults.value?.entries) return

  const entries = internalSearchResults.value.entries
  if (matchIndex < 0 || matchIndex >= entries.length) return

  // 找到对应的匹配项
  const targetEntry = entries[matchIndex]

  // 在 displayEntries 中找到这个条目的索引
  const displayIndex = displayEntries.value.findIndex(entry =>
    entry.lineNum === targetEntry.lineNum &&
    entry.message === targetEntry.message
  )

  if (displayIndex !== -1) {
    // 计算滚动位置
    const entryHeight = 60 // 估算每个日志条目的高度
    const scrollPosition = displayIndex * entryHeight

    // 滚动到位置
    containerRef.value.scrollTop = scrollPosition
    isUserScrolling.value = false

    console.log(`Scrolled to match ${matchIndex + 1} at display index ${displayIndex}`)
  }
}

// 暴露方法供父组件调用
defineExpose({
  scrollToMatch,
  refresh,
  toggleRealTimeMode,
  scrollToTop,
  scrollToBottom
})

onUnmounted(() => {
  wsService.off('log_update', handleLogUpdate)

  if (props.file?.path) {
    unsubscribeFromFile(props.file.path)
  }
})
</script>

<template>
  <div class="log-viewer" v-if="file">
    <!-- Toolbar -->
    <div class="log-toolbar">
      <div class="toolbar-left">
        <el-tooltip :content="realTimeMode ? '停止实时更新' : '开启实时更新'" placement="bottom">
          <el-button
            size="small"
            :type="realTimeMode ? 'success' : 'default'"
            :icon="realTimeMode ? VideoPause : VideoPlay"
            @click="toggleRealTimeMode"
          >
            {{ realTimeMode ? '停止实时' : '开启实时' }}
          </el-button>
        </el-tooltip>

        <el-tooltip content="刷新日志" placement="bottom">
          <el-button
            size="small"
            :icon="Refresh"
            :loading="loading"
            @click="refresh"
          >
            刷新
          </el-button>
        </el-tooltip>

        <!-- 新日志可用提示 -->
        <el-tooltip v-if="hasNewLogsAvailable" content="点击应用新的日志条目" placement="bottom">
          <el-button
            size="small"
            type="warning"
            :icon="Refresh"
            @click="applyPendingLogs"
            class="new-logs-button"
          >
            有新日志 ({{ pendingNewLogs.length }})
          </el-button>
        </el-tooltip>

        <div class="log-stats">
          <span class="stat-item">总行数: {{ totalLines.toLocaleString() }}</span>
          <span class="stat-item">已加载: {{ logEntries.length.toLocaleString() }}</span>
          <span v-if="internalSearchResults" class="stat-item search-info">
            搜索结果: {{ internalSearchResults.totalMatches.toLocaleString() }}
          </span>
          <span v-else-if="filterMode && searchQuery" class="stat-item filter-info">
            过滤显示: {{ filteredLogEntries.length.toLocaleString() }}
          </span>
          <span v-if="hasMore" class="stat-item has-more">还有更多...</span>
        </div>
      </div>

      <div class="toolbar-right">
        <el-tooltip content="滚动到顶部" placement="bottom">
          <el-button size="small" :icon="ArrowUp" @click="scrollToTop" />
        </el-tooltip>

        <el-tooltip content="滚动到底部" placement="bottom">
          <el-button size="small" :icon="ArrowDown" @click="scrollToBottom" />
        </el-tooltip>

        <el-tag
          :type="realTimeMode ? (isUserScrolling ? 'warning' : 'success') : 'info'"
          size="small"
          class="status-indicator"
        >
          {{ realTimeMode ? (isUserScrolling ? '实时模式 - 手动浏览' : '实时模式 - 自动滚动') : '手动模式' }}
        </el-tag>
      </div>
    </div>

    <!-- Log Content -->
    <div
      ref="containerRef"
      class="log-container"
      @scroll="handleScroll"
      v-loading="loading && logEntries.length === 0"
    >
      <div
        v-for="(entry, index) in displayEntries"
        :key="index"
        class="log-entry"
        :class="{
          'log-error': entry.level.toUpperCase() === 'ERROR',
          'log-warn': ['WARN', 'WARNING'].includes(entry.level.toUpperCase()),
          'log-info': entry.level.toUpperCase() === 'INFO',
          'log-debug': entry.level.toUpperCase() === 'DEBUG',
          'entry-hovered': hoveredEntryIndex === index
        }"
        @click="handleEntryClick(entry)"
        @mouseenter="hoveredEntryIndex = index"
        @mouseleave="hoveredEntryIndex = -1"
      >
        <div class="entry-header">
          <span class="entry-timestamp">{{ formatTimestamp(entry.timestamp) }}</span>
          <span
            class="entry-level-text"
            :style="{ color: getLogLevelColor(entry.level) }"
          >
            {{ entry.level.toUpperCase() }}
          </span>

          <!-- Formatting Button -->
          <el-button
            v-if="shouldShowFormattingButton(entry)"
            size="small"
            type="primary"
            :plain="!formattedEntries.has(index)"
            :icon="Document"
            @click.stop="toggleFormatting(index)"
            class="json-format-btn"
          >
            {{ formattedEntries.has(index) ? '原文' : '格式化' }}
          </el-button>

          <span class="entry-line-num" v-show="showLineNumbers">#{{ entry.lineNum }}</span>
        </div>

        <div class="entry-content">
          <!-- Log content display using strategy pattern -->
          <div
            class="entry-message"
            :class="getDisplayType(entry, index)"
            v-html="getFormattedContent(entry, index)"
          />
        </div>
      </div>

      <!-- No results indicator for filter mode -->
      <div v-if="filterMode && searchQuery && displayEntries.length === 0 && logEntries.length > 0" class="no-filter-results">
        <span v-if="internalSearchResults">没有找到匹配的搜索结果</span>
        <span v-else>没有匹配的日志条目</span>
      </div>

      <!-- Loading more indicator -->
      <div v-if="loading && logEntries.length > 0" class="loading-more">
        <span>加载更多...</span>
      </div>

      <!-- No more data indicator -->
      <div v-if="!hasMore && logEntries.length > 0" class="no-more-data">
        <span>已显示所有日志</span>
      </div>
    </div>
  </div>

  <!-- Empty state -->
  <div v-else class="log-viewer-empty">
    <el-empty description="请选择一个日志文件开始查看" />
  </div>
</template>

<style scoped>
.log-viewer {
  height: 100%;
  display: flex;
  flex-direction: column;
  background-color: var(--color-background);
}

.log-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 16px;
  border-bottom: 1px solid var(--color-border);
  background-color: var(--color-background-soft);
  flex-shrink: 0;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.log-stats {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-left: 16px;
}

.stat-item {
  font-size: 12px;
  color: var(--color-text-2);
}

.has-more {
  color: #409eff;
  font-weight: 500;
}

.filter-info {
  color: #67c23a;
  font-weight: 500;
}

.search-info {
  color: #409eff;
  font-weight: 500;
}

.status-indicator {
  font-size: 11px;
  height: 20px;
  line-height: 18px;
}

.new-logs-button {
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(230, 162, 60, 0.7);
  }
  70% {
    box-shadow: 0 0 0 10px rgba(230, 162, 60, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(230, 162, 60, 0);
  }
}

.log-container {
  flex: 1;
  overflow: auto;
  background-color: #323232;
  color: #ffffff;
  font-family: var(--log-font-family, 'Consolas', 'Monaco', 'Courier New', monospace);
  font-size: var(--log-font-size, 14px);
  line-height: var(--log-line-height, 1.5);
}

.log-entry {
  padding: 8px 12px;
  border-bottom: 1px solid #2d2d2d;
  cursor: pointer;
  transition: background-color 0.2s;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.log-entry:hover {
  background-color: #2d2d2d;
}

.log-entry.log-error {
  border-left: 3px solid #f56c6c;
}

.log-entry.log-warn {
  border-left: 3px solid #e6a23c;
}

.log-entry.log-info {
  border-left: 3px solid #409eff;
}

.log-entry.log-debug {
  border-left: 3px solid #909399;
}

.entry-header {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.entry-timestamp {
  color: #858585;
  font-size: 12px;
  font-weight: 500;
  min-width: 140px;
  flex-shrink: 0;
}

.entry-level {
  font-size: 11px;
  height: 20px;
  line-height: 18px;
  padding: 0 6px;
  min-width: 60px;
  text-align: center;
  flex-shrink: 0;
}

.entry-level-text {
  font-size: 11px;
  font-weight: 600;
  min-width: 60px;
  text-align: left;
  flex-shrink: 0;
  text-transform: uppercase;
}

.entry-line-num {
  color: #858585;
  font-size: 11px;
  margin-left: auto;
  flex-shrink: 0;
}

/* Current line highlight */
.log-entry.entry-hovered {
  background-color: rgba(255, 255, 255, 0.8);
  box-shadow: inset 3px 0 0 rgba(255, 255, 255, 0.8);
  transition: all 0.2s ease;
}

.entry-content {
  flex: 1;
  min-width: 0;
}

.entry-message {
  color: #ffffff;
  word-break: var(--word-wrap, break-word);
  white-space: pre-wrap;
  line-height: var(--log-line-height, 1.5);
  font-size: var(--log-font-size, 14px);
}

.entry-fields {
  margin-top: 4px;
  padding-left: 16px;
}

.field-item {
  display: block;
  margin: 2px 0;
  font-size: 12px;
}

.field-key {
  color: #9cdcfe;
  font-weight: 500;
}

.field-value {
  color: #ce9178;
  margin-left: 4px;
}

.loading-more,
.no-more-data,
.no-filter-results {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  color: #909399;
  background-color: #2d2d2d;
  font-size: 12px;
}

.no-filter-results {
  color: #e6a23c;
  background-color: rgba(230, 162, 60, 0.1);
}

.log-viewer-empty {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Search highlight */
:deep(.search-highlight) {
  background-color: #ffeb3b;
  color: #000;
  padding: 1px 2px;
  border-radius: 2px;
}

/* JSON Format Button */
.json-format-btn {
  font-size: 11px;
  height: 22px;
  padding: 0 10px;
  margin-left: 8px;
  min-width: 76px;
}

/* Raw Text Display (default) */
.entry-message.raw-text {
  color: #ffffff;
  word-break: var(--word-wrap, break-word);
  white-space: pre-wrap;
  line-height: var(--log-line-height, 1.5);
  font-size: var(--log-font-size, 14px);
}

/* JSON Formatted Display */
.entry-message.json-formatted {
  background-color: #1e1e1e;
  border: 1px solid #444;
  border-radius: 4px;
  padding: 12px;
  margin: 4px 0;
  font-family: 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.4;
  overflow-x: auto;
  white-space: pre;
}

/* WebServer Formatted Display */
.entry-message.webserver-formatted {
  background-color: #2a2a2a;
  border: 1px solid #555;
  border-radius: 4px;
  padding: 10px;
  margin: 4px 0;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.3;
}

.webserver-formatted .field-row {
  display: flex;
  margin: 2px 0;
  align-items: flex-start;
}

.webserver-formatted .field-label {
  color: #9cdcfe;
  font-weight: 500;
  min-width: 80px;
  margin-right: 8px;
  flex-shrink: 0;
}

.webserver-formatted .field-value {
  color: #ce9178;
  word-break: break-word;
  flex: 1;
}

/* JSON Syntax Highlighting */
:deep(.json-key) {
  color: #9cdcfe;
  font-weight: 500;
}

:deep(.json-string) {
  color: #ce9178;
}

:deep(.json-number) {
  color: #b5cea8;
}

:deep(.json-boolean) {
  color: #569cd6;
  font-weight: 500;
}

:deep(.json-null) {
  color: #569cd6;
  font-style: italic;
}

/* Scrollbar styling */
.log-container::-webkit-scrollbar {
  width: 8px;
}

.log-container::-webkit-scrollbar-track {
  background: #2d2d2d;
}

.log-container::-webkit-scrollbar-thumb {
  background: #555;
  border-radius: 4px;
}

.log-container::-webkit-scrollbar-thumb:hover {
  background: #777;
}
</style>
