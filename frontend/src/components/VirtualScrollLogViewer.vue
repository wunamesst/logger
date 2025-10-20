<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { ElButton, ElIcon, ElTag, ElTooltip, ElMessage } from 'element-plus'
import { VideoPlay, VideoPause, Refresh, ArrowUp, ArrowDown, Document } from '@element-plus/icons-vue'
import apiService, { type LogFile, type LogEntry, type LogContent, type SearchResult, type SearchQuery } from '../services/api'
import wsService, { type LogUpdate } from '../services/websocket'
import { LogFormattingManager } from '../services/logFormatting'

// Props - 与原LogViewer保持一致
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
    wordWrap: true
  })
})

// Emits
const emit = defineEmits<{
  entryClick: [entry: LogEntry]
}>()

// 虚拟滚动配置 - 改为动态高度模式
const BASE_ITEM_HEIGHT = 60 // 基础行高（未展开时），与CSS min-height保持一致
const EXPANDED_ITEM_HEIGHT = 200 // 展开时的大概高度
const VISIBLE_COUNT = 30 // 可见区域显示的条目数
const BUFFER_SIZE = 10 // 缓冲区大小

// Reactive state - 重构为支持tail -f模式
const logEntries = ref<LogEntry[]>([])
const loading = ref(false)
const realTimeMode = ref(false)
// 重新设计状态管理 - 基于行号而不是offset
const earliestLineNum = ref<number | null>(null) // 当前最早的行号
const latestLineNum = ref<number | null>(null)   // 当前最新的行号
const totalLines = ref(0)
const hasEarlierData = ref(true)  // 是否还有更早的数据（向上加载）
const hasLaterData = ref(false)   // 是否还有更新的数据（非实时模式）
const containerRef = ref<HTMLElement>()
const scrollContentRef = ref<HTMLElement>()
const isUserScrolling = ref(false)
const isAutoScrolling = ref(false) // 标记是否正在自动滚动
const hoveredEntryIndex = ref(-1)
const formattedEntries = ref<Set<number>>(new Set())
const pendingNewLogs = ref<LogEntry[]>([])
const hasNewLogsAvailable = ref(false)
const internalSearchResults = ref<SearchResult | null>(null)
// 加载状态标识
const isLoadingEarlier = ref(false) // 正在加载更早的数据
const isInitialLoad = ref(true)     // 是否是初始加载
// 日志去重哈希集合
const seenHashes = ref<Set<string>>(new Set())

// 计算日志条目的哈希值(轻量级哈希算法)
const hashLogEntry = (entry: LogEntry): string => {
  // 使用时间戳、级别和消息内容生成哈希
  const str = `${entry.timestamp}|${entry.level}|${entry.message}`
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i)
    hash = ((hash << 5) - hash) + char
    hash = hash & hash // Convert to 32bit integer
  }
  return hash.toString(36) // 转换为36进制字符串，更紧凑
}

// 虚拟滚动状态
const scrollTop = ref(0)
const containerHeight = ref(0)
const isScrolling = ref(false)
const scrollTimeout = ref<number | null>(null)
const itemHeights = ref<Map<number, number>>(new Map()) // 存储每个条目的实际高度
const itemElements = ref<Map<number, HTMLElement>>(new Map()) // 存储元素引用
const cachedTotalHeight = ref(0) // 缓存的总高度
const lastCalculatedIndex = ref(-1) // 上次计算到的索引
const heightDirty = ref(true) // 标记高度是否需要重新计算

// 格式化管理器
const formattingManager = new LogFormattingManager()

// 性能设置
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

// 获取条目高度（动态计算，优先使用实际DOM高度）
const getItemHeight = (index: number): number => {
  // 先检查缓存的实际高度
  if (itemHeights.value.has(index)) {
    return itemHeights.value.get(index)!
  }

  // 尝试从DOM元素获取实际高度
  if (itemElements.value.has(index)) {
    const element = itemElements.value.get(index)!
    const actualHeight = element.offsetHeight
    if (actualHeight > 0) {
      // 缓存实际高度
      itemHeights.value.set(index, actualHeight)
      return actualHeight
    }
  }

  // 估算高度：根据是否展开来决定
  const isExpanded = formattedEntries.value.has(index)
  return isExpanded ? EXPANDED_ITEM_HEIGHT : BASE_ITEM_HEIGHT
}

// 优化的总高度计算 - 使用数据驱动而非DOM驱动，解决虚拟滚动高度计算问题
const getTotalHeight = (): number => {
  const entries = displayEntries.value
  if (entries.length === 0) {
    cachedTotalHeight.value = 0
    lastCalculatedIndex.value = -1
    heightDirty.value = false
    return 0
  }

  // 数据驱动的高度计算：基于条目数量而非DOM元素
  let totalHeight = entries.length * BASE_ITEM_HEIGHT

  // 为展开的条目添加额外高度
  formattedEntries.value.forEach(index => {
    if (index < entries.length) {
      totalHeight += (EXPANDED_ITEM_HEIGHT - BASE_ITEM_HEIGHT)
    }
  })

  // 更新缓存
  cachedTotalHeight.value = totalHeight
  lastCalculatedIndex.value = entries.length - 1
  heightDirty.value = false

  return totalHeight
}

// 计算可见区域的日志条目 - 优化版本
const visibleEntries = computed(() => {
  const entries = displayEntries.value
  if (entries.length === 0) return {
    items: [],
    startIndex: 0,
    endIndex: 0,
    offsetY: 0,
    totalHeight: 0
  }

  const viewportTop = scrollTop.value
  const viewportBottom = viewportTop + containerHeight.value

  // 使用二分查找优化起始索引计算
  let startIndex = findStartIndex(entries, viewportTop)
  let endIndex = findEndIndex(entries, viewportBottom, startIndex)

  // 添加缓冲区
  startIndex = Math.max(0, startIndex - BUFFER_SIZE)
  endIndex = Math.min(entries.length, endIndex + BUFFER_SIZE)

  // 计算偏移量
  let offsetY = 0
  for (let i = 0; i < startIndex; i++) {
    offsetY += getItemHeight(i)
  }

  return {
    items: entries.slice(startIndex, endIndex),
    startIndex,
    endIndex,
    offsetY,
    totalHeight: getTotalHeight()
  }
})

// 二分查找起始索引
const findStartIndex = (entries: LogEntry[], viewportTop: number): number => {
  let currentY = 0
  let left = 0
  let right = entries.length - 1

  while (left <= right) {
    const mid = Math.floor((left + right) / 2)

    // 计算到mid位置的累积高度
    let midY = 0
    for (let i = 0; i <= mid; i++) {
      midY += getItemHeight(i)
    }

    if (midY < viewportTop) {
      left = mid + 1
    } else {
      right = mid - 1
    }
  }

  return Math.max(0, left)
}

// 查找结束索引
const findEndIndex = (entries: LogEntry[], viewportBottom: number, startIndex: number): number => {
  let currentY = 0

  // 计算到startIndex的累积高度
  for (let i = 0; i < startIndex; i++) {
    currentY += getItemHeight(i)
  }

  // 从startIndex开始查找
  for (let i = startIndex; i < entries.length; i++) {
    currentY += getItemHeight(i)
    if (currentY >= viewportBottom) {
      return i + 1
    }
  }

  return entries.length
}

// 虚拟滚动容器总高度（动态计算）
const totalHeight = computed(() => getTotalHeight())

// Settings-based computed properties
const showLineNumbers = computed(() => props.displaySettings?.showLineNumbers ?? true)

// 应用显示设置
const applyDisplaySettings = () => {
  if (!props.displaySettings) return

  const root = document.documentElement
  const settings = props.displaySettings

  root.style.setProperty('--log-font-size', `${settings.fontSize}px`)
  root.style.setProperty('--log-line-height', settings.lineHeight.toString())
  root.style.setProperty('--log-font-family', settings.fontFamily)
  root.style.setProperty('--show-line-numbers', settings.showLineNumbers ? 'block' : 'none')
  root.style.setProperty('--word-wrap', settings.wordWrap ? 'break-word' : 'normal')
}

// Watch for display settings changes
watch(() => props.displaySettings, () => {
  applyDisplaySettings()
}, { deep: true, immediate: true })

// 格式化时间戳
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

// JSON检查和格式化函数 - 从原LogViewer复制
const isValidJson = (str: string): boolean => {
  if (!str || typeof str !== 'string') return false
  str = str.trim()
  if (!((str.startsWith('{') && str.endsWith('}')) || (str.startsWith('[') && str.endsWith(']')))) {
    return false
  }
  try {
    JSON.parse(str)
    return true
  } catch {
    return false
  }
}

const formatJson = (jsonStr: string): string => {
  try {
    const parsed = JSON.parse(jsonStr)
    return JSON.stringify(parsed, null, 2)
  } catch (error) {
    return jsonStr
  }
}

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

// 检查是否显示格式化按钮
const shouldShowFormattingButton = (entry: LogEntry): boolean => {
  return formattingManager.shouldShowFormattingButton(entry)
}

// 获取格式化内容
const getFormattedContent = (entry: LogEntry, index: number): string => {
  let content: string

  if (!formattedEntries.value.has(index)) {
    content = entry.raw
  } else {
    const result = formattingManager.formatLog(entry)
    content = result.content
  }

  if (props.searchQuery && props.searchHighlight) {
    return highlightText(content, props.searchQuery)
  }

  return content
}

// 获取显示类型
const getDisplayType = (entry: LogEntry, index: number): string => {
  if (!formattedEntries.value.has(index)) {
    return 'raw-text'
  }
  const result = formattingManager.formatLog(entry)
  return result.displayType
}

// 获取日志级别颜色
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

// 检查日志条目是否匹配搜索查询
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
      console.warn('Invalid regex pattern, falling back to simple search:', error)
      const lowerQuery = query.toLowerCase()
      return searchContent.some(content => content.toLowerCase().includes(lowerQuery))
    }
  } else {
    const lowerQuery = query.toLowerCase()
    return searchContent.some(content => content.toLowerCase().includes(lowerQuery))
  }
}

// 高亮搜索文本
const highlightText = (text: string, query: string): string => {
  if (!query || !props.searchHighlight) return text

  if (props.isRegex) {
    try {
      const regex = new RegExp(`(${query})`, 'gi')
      return text.replace(regex, '<mark class="search-highlight">$1</mark>')
    } catch (error) {
      console.warn('Invalid regex pattern for highlighting:', error)
      const escapedQuery = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
      const regex = new RegExp(`(${escapedQuery})`, 'gi')
      return text.replace(regex, '<mark class="search-highlight">$1</mark>')
    }
  } else {
    const escapedQuery = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const regex = new RegExp(`(${escapedQuery})`, 'gi')
    return text.replace(regex, '<mark class="search-highlight">$1</mark>')
  }
}

// 过滤日志条目
const filteredLogEntries = computed(() => {
  if (!props.filterMode || !props.searchQuery) {
    return logEntries.value
  }
  return logEntries.value.filter(entry => isLogEntryMatch(entry, props.searchQuery))
})

// 执行搜索
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
      limit: 50000
    }

    const result = await apiService.searchLogs(searchQuery)
    internalSearchResults.value = result
    console.log('Internal search completed:', result)
  } catch (error) {
    console.error('Internal search failed:', error)
    internalSearchResults.value = null
  }
}

// 获取显示条目
const displayEntries = computed(() => {
  if (props.filterMode && internalSearchResults.value && internalSearchResults.value.entries.length > 0) {
    return internalSearchResults.value.entries
  }
  return props.filterMode ? filteredLogEntries.value : logEntries.value
})

// 应用性能限制 - 适应新的数据结构
const enforceLogLimits = () => {
  const maxLines = performanceSettings.value.maxLogLines || 10000
  if (logEntries.value.length > maxLines) {
    const excess = logEntries.value.length - maxLines

    // 从数组开头移除过多的历史数据（保留最新的数据）
    const removedEntries = logEntries.value.splice(0, excess)

    // 清理被移除条目的哈希
    removedEntries.forEach(entry => {
      seenHashes.value.delete(hashLogEntry(entry))
    })

    // 清理被移除条目的高度缓存
    removedEntries.forEach((_, index) => {
      itemHeights.value.delete(index)
      itemElements.value.delete(index)
    })

    // 重新映射剩余条目的高度缓存
    const newItemHeights = new Map<number, number>()
    const newItemElements = new Map<number, HTMLElement>()

    for (let i = 0; i < logEntries.value.length; i++) {
      const oldIndex = i + excess
      if (itemHeights.value.has(oldIndex)) {
        newItemHeights.set(i, itemHeights.value.get(oldIndex)!)
      }
      if (itemElements.value.has(oldIndex)) {
        newItemElements.set(i, itemElements.value.get(oldIndex)!)
      }
    }

    itemHeights.value = newItemHeights
    itemElements.value = newItemElements

    // 更新最早的行号
    if (logEntries.value.length > 0) {
      earliestLineNum.value = logEntries.value[0].lineNum
      // 如果移除了数据，说明可能还有更早的数据
      hasEarlierData.value = true
    }

    // 标记高度需要重新计算
    heightDirty.value = true
    cachedTotalHeight.value = 0
    lastCalculatedIndex.value = -1

    console.log(`Memory optimized - removed ${excess} oldest entries, current: ${logEntries.value.length}`, {
      newEarliestLineNum: earliestLineNum.value,
      latestLineNum: latestLineNum.value,
      cacheSize: itemHeights.value.size,
      hashSetSize: seenHashes.value.size
    })
  }
}

// 初始加载：使用tail API获取最后N条日志
const loadInitialLogs = async (limit = 100) => {
  if (!props.file || loading.value) return

  console.log('Loading initial logs using tail API:', { file: props.file.path, limit })
  loading.value = true
  isInitialLoad.value = true

  try {
    const content = await apiService.getLogContentFromTail(props.file.path, limit)

    // 初始化数据状态
    logEntries.value = content.entries
    totalLines.value = content.totalLines

    // 清空并重新构建哈希集合
    seenHashes.value.clear()
    content.entries.forEach(entry => {
      seenHashes.value.add(hashLogEntry(entry))
    })

    // 设置行号范围
    if (content.entries.length > 0) {
      earliestLineNum.value = content.entries[0].lineNum
      latestLineNum.value = content.entries[content.entries.length - 1].lineNum
      // 如果最早的行号大于1，说明还有更早的数据
      hasEarlierData.value = earliestLineNum.value > 1
      // 初始加载是从尾部开始，所以没有更新的数据
      hasLaterData.value = false
    } else {
      earliestLineNum.value = null
      latestLineNum.value = null
      hasEarlierData.value = false
      hasLaterData.value = false
    }

    // 标记数据完全替换
    markDataChanged('replace')

    enforceLogLimits()

    console.log('Initial load completed:', {
      entriesCount: content.entries.length,
      earliestLineNum: earliestLineNum.value,
      latestLineNum: latestLineNum.value,
      hasEarlierData: hasEarlierData.value
    })

    // 初始加载后自动滚动到底部 - 增加延迟确保DOM和虚拟滚动完全渲染
    nextTick(() => {
      // 给更多时间让虚拟滚动计算完成（150ms）
      setTimeout(() => {
        scrollToBottom(false)
        isInitialLoad.value = false
      }, 150)
    })

  } catch (error) {
    console.error('Failed to load initial logs:', error)
    ElMessage.error('加载日志内容失败')
  } finally {
    loading.value = false
  }
}

// 向上加载更早的数据 - 优化位置保持算法
const loadEarlierLogs = async (limit = 100) => {
  if (!props.file || isLoadingEarlier.value || !hasEarlierData.value || !earliestLineNum.value) {
    return
  }

  console.log('Loading earlier logs:', { fromLineNum: earliestLineNum.value, limit })
  isLoadingEarlier.value = true

  try {
    // 从当前最早行号向前读取
    const endLineNum = earliestLineNum.value - 1
    const startLineNum = Math.max(1, endLineNum - limit + 1)

    const content = await apiService.getLogContent(
      props.file.path,
      startLineNum - 1, // API使用offset，从0开始
      endLineNum - startLineNum + 1
    )

    if (content.entries.length > 0) {
      // === 优化的位置保持算法 ===
      const scrollContainer = containerRef.value
      if (!scrollContainer) return

      // 1. 记录当前状态
      const previousScrollTop = scrollContainer.scrollTop
      const previousFirstVisibleIndex = Math.floor(previousScrollTop / BASE_ITEM_HEIGHT)
      const previousFirstEntry = logEntries.value[previousFirstVisibleIndex]

      console.log('Before insertion:', {
        previousScrollTop,
        previousFirstVisibleIndex,
        previousFirstEntryLineNum: previousFirstEntry?.lineNum,
        currentEntriesCount: logEntries.value.length
      })

      // 2. 插入新数据到数组开头
      logEntries.value.unshift(...content.entries)

      // 将新加载的历史条目的哈希添加到集合中
      content.entries.forEach(entry => {
        seenHashes.value.add(hashLogEntry(entry))
      })

      // 标记数据前置插入
      markDataChanged('prepend')

      // 3. 更新行号范围
      earliestLineNum.value = content.entries[0].lineNum
      hasEarlierData.value = earliestLineNum.value > 1

      console.log('Earlier logs loaded:', {
        newEntriesCount: content.entries.length,
        newEarliestLineNum: earliestLineNum.value,
        hasEarlierData: hasEarlierData.value,
        totalEntriesCount: logEntries.value.length
      })

      // 4. 等待DOM更新后调整滚动位置
      await nextTick()

      if (!isInitialLoad.value && previousFirstEntry) {
        // 找到原来的第一个可见条目在新数组中的位置
        const newIndexOfPreviousFirst = logEntries.value.findIndex(
          entry => entry.lineNum === previousFirstEntry.lineNum
        )

        if (newIndexOfPreviousFirst !== -1) {
          // 计算新的滚动位置
          let newScrollTop = 0
          for (let i = 0; i < newIndexOfPreviousFirst; i++) {
            newScrollTop += getItemHeight(i)
          }

          // 加上原来在第一个条目内的偏移量
          const offsetWithinFirstItem = previousScrollTop - (previousFirstVisibleIndex * BASE_ITEM_HEIGHT)
          newScrollTop += Math.max(0, offsetWithinFirstItem)

          console.log('Position calculation:', {
            newIndexOfPreviousFirst,
            newScrollTop,
            offsetWithinFirstItem,
            previousFirstEntryLineNum: previousFirstEntry.lineNum
          })

          // 平滑调整滚动位置
          scrollContainer.scrollTop = newScrollTop
          scrollTop.value = newScrollTop

          console.log('Scroll position adjusted:', {
            from: previousScrollTop,
            to: newScrollTop,
            difference: newScrollTop - previousScrollTop
          })
        }
      }
    }

    enforceLogLimits()

  } catch (error) {
    console.error('Failed to load earlier logs:', error)
    ElMessage.error('加载历史日志失败')
  } finally {
    isLoadingEarlier.value = false
  }
}

// 刷新日志内容 - 重新使用tail模式
const refresh = async () => {
  if (!props.file) return

  console.log('Refreshing logs in tail mode')
  // 重置状态
  logEntries.value = []
  earliestLineNum.value = null
  latestLineNum.value = null
  hasEarlierData.value = true
  hasLaterData.value = false
  formattedEntries.value.clear()
  seenHashes.value.clear() // 清空哈希集合

  // 重新加载最后N条
  await loadInitialLogs(performanceSettings.value.preloadLines || 100)

  if (realTimeMode.value) {
    nextTick(() => {
      autoScrollToBottom()
    })
  }
}

// 切换实时模式
const toggleRealTimeMode = () => {
  realTimeMode.value = !realTimeMode.value
  if (realTimeMode.value) {
    // 启用实时模式时，立即重置用户滚动状态并滚动到底部
    isUserScrolling.value = false
    nextTick(() => {
      autoScrollToBottom()
    })
  }
}

// 虚拟滚动处理函数 - 优化向上加载触发逻辑
const handleScroll = (event: Event) => {
  const target = event.target as HTMLElement
  scrollTop.value = target.scrollTop

  // 设置滚动状态
  isScrolling.value = true
  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
  }
  scrollTimeout.value = setTimeout(() => {
    isScrolling.value = false
  }, 150)

  // 检查是否在底部（用于实时模式的自动滚动控制）
  const isAtBottom = target.scrollTop >= target.scrollHeight - target.clientHeight - 50

  // 智能用户滚动检测：只有在非自动滚动期间的滚动才算用户滚动
  if (!isAutoScrolling.value) {
    if (!isAtBottom && realTimeMode.value) {
      isUserScrolling.value = true
    } else if (isAtBottom && realTimeMode.value) {
      isUserScrolling.value = false
    }
  }

  // 优化的向上加载逻辑：更智能的触发条件
  const scrollPercentage = target.scrollTop / (target.scrollHeight - target.clientHeight)
  const isNearTop = scrollPercentage < 0.1 // 在顶部10%区域内
  const isScrollingUp = scrollTop.value < (containerRef.value?.dataset.lastScrollTop ? parseFloat(containerRef.value.dataset.lastScrollTop) : 0)

  // 记录上次滚动位置用于判断滚动方向
  if (containerRef.value) {
    containerRef.value.dataset.lastScrollTop = target.scrollTop.toString()
  }

  // 只有在用户向上滚动且接近顶部时才触发加载
  if (isNearTop && isScrollingUp && hasEarlierData.value && !isLoadingEarlier.value && !isInitialLoad.value) {
    console.log('User scrolled up near top, triggering earlier data load:', {
      scrollPercentage: scrollPercentage.toFixed(3),
      scrollTop: target.scrollTop,
      isScrollingUp
    })
    loadEarlierLogs()
  }
}

// 滚动到顶部
const scrollToTop = () => {
  if (containerRef.value) {
    containerRef.value.scrollTop = 0
    scrollTop.value = 0
    isUserScrolling.value = false
  }
}

// 滚动到底部 - 使用真实 scrollHeight 确保准确定位
const scrollToBottom = (smooth = false) => {
  if (!containerRef.value) return

  isAutoScrolling.value = true
  isUserScrolling.value = false
  const container = containerRef.value

  if (smooth) {
    // 平滑滚动模式:直接滚动到最大位置
    const maxScrollTop = container.scrollHeight - container.clientHeight
    container.scrollTo({
      top: maxScrollTop,
      behavior: 'smooth'
    })
    setTimeout(() => {
      isAutoScrolling.value = false
    }, 1000)
    return
  }

  // 迭代滚动算法:逐步触发虚拟滚动渲染底部内容
  const performIterativeScroll = async (iteration = 0, maxIterations = 5) => {
    const maxScrollTop = container.scrollHeight - container.clientHeight

    console.log(`Iterative scroll (iteration ${iteration}):`, {
      scrollHeight: container.scrollHeight,
      clientHeight: container.clientHeight,
      maxScrollTop,
      currentScrollTop: container.scrollTop,
      entriesCount: displayEntries.value.length
    })

    // 设置滚动位置到当前最大值
    container.scrollTop = maxScrollTop
    scrollTop.value = maxScrollTop

    // 等待虚拟滚动重新计算和DOM更新
    await nextTick()
    await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)))

    // 延迟50ms让虚拟滚动完成渲染
    await new Promise(resolve => setTimeout(resolve, 50))

    // 检查scrollHeight是否增长(说明虚拟滚动渲染了更多内容)
    const newMaxScrollTop = container.scrollHeight - container.clientHeight
    const scrollTopChanged = Math.abs(newMaxScrollTop - maxScrollTop) > 5

    if (scrollTopChanged && iteration < maxIterations - 1) {
      // scrollHeight增长了,继续下一次迭代
      console.log(`ScrollHeight increased: ${maxScrollTop} -> ${newMaxScrollTop}, continuing...`)
      return performIterativeScroll(iteration + 1, maxIterations)
    } else {
      // 已经到底或达到最大迭代次数
      const finalScrollTop = container.scrollHeight - container.clientHeight
      container.scrollTop = finalScrollTop
      scrollTop.value = finalScrollTop

      console.log('Iterative scroll completed:', {
        iterations: iteration + 1,
        finalScrollHeight: container.scrollHeight,
        finalScrollTop,
        reachedBottom: container.scrollTop >= finalScrollTop - 5
      })

      isAutoScrolling.value = false
    }
  }

  // 等待DOM渲染完成后开始迭代
  nextTick(() => {
    requestAnimationFrame(() => performIterativeScroll())
  })
}

// 优化的实时滚动：使用迭代算法实现准确的 tail -f 效果
const autoScrollToBottom = async () => {
  if (!containerRef.value || !realTimeMode.value || isUserScrolling.value) {
    return
  }

  isAutoScrolling.value = true
  const container = containerRef.value

  // 使用相同的迭代滚动算法
  const performIterativeScroll = async (iteration = 0, maxIterations = 5) => {
    const maxScrollTop = container.scrollHeight - container.clientHeight

    console.log(`Auto-scroll iteration ${iteration}:`, {
      scrollHeight: container.scrollHeight,
      clientHeight: container.clientHeight,
      maxScrollTop,
      currentScrollTop: container.scrollTop,
      entriesCount: displayEntries.value.length
    })

    // 设置滚动位置到当前最大值
    container.scrollTop = maxScrollTop
    scrollTop.value = maxScrollTop

    // 等待虚拟滚动重新计算和DOM更新
    await nextTick()
    await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)))

    // 延迟50ms让虚拟滚动完成渲染
    await new Promise(resolve => setTimeout(resolve, 50))

    // 检查scrollHeight是否增长
    const newMaxScrollTop = container.scrollHeight - container.clientHeight
    const scrollTopChanged = Math.abs(newMaxScrollTop - maxScrollTop) > 5

    if (scrollTopChanged && iteration < maxIterations - 1) {
      console.log(`Auto-scroll: ScrollHeight increased ${maxScrollTop} -> ${newMaxScrollTop}, continuing...`)
      return performIterativeScroll(iteration + 1, maxIterations)
    } else {
      // 到底或达到最大迭代次数
      const finalScrollTop = container.scrollHeight - container.clientHeight
      container.scrollTop = finalScrollTop
      scrollTop.value = finalScrollTop

      console.log('Auto-scroll completed:', {
        iterations: iteration + 1,
        finalScrollHeight: container.scrollHeight,
        finalScrollTop
      })

      isAutoScrolling.value = false
    }
  }

  // 确保DOM更新完成后开始迭代 - 增加额外的 nextTick 提高可靠性
  await nextTick()
  await nextTick() // 额外的 nextTick 确保 Vue 完全处理完响应式更新
  requestAnimationFrame(() => performIterativeScroll())
}

// 标记数据已变化，需要重新计算高度
const markDataChanged = (changeType: 'append' | 'prepend' | 'replace' = 'append') => {
  if (changeType === 'replace') {
    // 完全替换数据时，清空所有缓存
    itemHeights.value.clear()
    itemElements.value.clear()
    cachedTotalHeight.value = 0
    lastCalculatedIndex.value = -1
  }
  heightDirty.value = true

  // 强制触发虚拟滚动视口重新计算
  // 通过微小改变 scrollTop 值来触发 visibleEntries 计算属性的重新计算
  if (changeType === 'append' && containerRef.value) {
    nextTick(() => {
      const container = containerRef.value
      if (container) {
        const currentScrollTop = container.scrollTop
        // 临时微调 scrollTop 强制触发响应式更新
        scrollTop.value = currentScrollTop + 0.1
        requestAnimationFrame(() => {
          scrollTop.value = currentScrollTop
        })
      }
    })
  }
}

// 更新条目高度缓存
const updateItemHeight = (index: number, height: number) => {
  const oldHeight = itemHeights.value.get(index) || getItemHeight(index)
  itemHeights.value.set(index, height)

  // 如果高度有变化，标记需要重新计算
  if (Math.abs(oldHeight - height) > 1) {
    heightDirty.value = true
  }
}

// 处理日志条目点击
const handleEntryClick = (entry: LogEntry) => {
  emit('entryClick', entry)
}

// 应用待处理的新日志 - 从原LogViewer复制逻辑
const applyPendingLogs = () => {
  if (pendingNewLogs.value.length > 0) {
    if (props.filterMode && props.searchQuery && internalSearchResults.value) {
      const newUniqueEntries = pendingNewLogs.value.filter(pendingEntry => {
        const existsInSearchResults = internalSearchResults.value?.entries.some(existingEntry =>
          existingEntry.lineNum === pendingEntry.lineNum &&
          existingEntry.message === pendingEntry.message
        ) || false

        const existsInMainLogs = logEntries.value.some(existingEntry =>
          existingEntry.lineNum === pendingEntry.lineNum &&
          existingEntry.message === pendingEntry.message
        )

        return !existsInSearchResults && !existsInMainLogs
      })

      if (newUniqueEntries.length > 0) {
        logEntries.value.push(...newUniqueEntries)
        totalLines.value += newUniqueEntries.length
      }
    } else {
      const newUniqueEntries = pendingNewLogs.value.filter(pendingEntry =>
        !logEntries.value.some(existingEntry =>
          existingEntry.lineNum === pendingEntry.lineNum &&
          existingEntry.message === pendingEntry.message
        )
      )

      if (newUniqueEntries.length > 0) {
        logEntries.value.push(...newUniqueEntries)
        totalLines.value += newUniqueEntries.length
      }
    }

    enforceLogLimits()
    pendingNewLogs.value = []
    hasNewLogsAvailable.value = false

    if (realTimeMode.value && !isUserScrolling.value) {
      nextTick(() => {
        scrollToBottom()
      })
    }
  }
}

// 处理WebSocket日志更新 - 重构支持新的数据结构
const handleLogUpdate = (update: LogUpdate) => {
  if (!props.file) return

  const currentPath = props.file.path
  const currentFilename = currentPath.split('/').pop() || currentPath
  const updatePath = update.path
  const updateFilename = updatePath.split('/').pop() || updatePath

  if (updatePath !== currentPath && updateFilename !== currentFilename) {
    return
  }

  console.log('Received WebSocket log update:', {
    type: update.type,
    entriesCount: update.entries.length,
    realTimeMode: realTimeMode.value,
    isUserScrolling: isUserScrolling.value
  })

  if (update.type === 'append') {
    // 使用哈希算法过滤出新的条目
    const newEntries = update.entries.filter(entry => {
      const hash = hashLogEntry(entry)
      return !seenHashes.value.has(hash)
    })

    if (newEntries.length > 0) {
      console.log('Adding new real-time entries:', {
        newEntriesCount: newEntries.length,
        firstNewLineNum: newEntries[0]?.lineNum,
        lastNewLineNum: newEntries[newEntries.length - 1]?.lineNum,
        currentLatestLineNum: latestLineNum.value
      })

      // 追加新数据到数组末尾
      logEntries.value.push(...newEntries)
      totalLines.value += newEntries.length

      // 将新条目的哈希添加到集合中
      newEntries.forEach(entry => {
        seenHashes.value.add(hashLogEntry(entry))
      })

      // 标记数据追加 - 强制触发响应式更新
      markDataChanged('append')

      // 更新最新行号
      const lastEntry = newEntries[newEntries.length - 1]
      if (lastEntry && (!latestLineNum.value || lastEntry.lineNum > latestLineNum.value)) {
        latestLineNum.value = lastEntry.lineNum
      }

      enforceLogLimits()

      console.log('LogEntries updated, new total:', logEntries.value.length)
      console.log('DisplayEntries computed:', displayEntries.value.length)

      // 关键修复：依赖 markDataChanged 的强制更新机制触发虚拟滚动重新计算
      // 使用 nextTick 确保 Vue 响应式系统完成数据更新
      nextTick(() => {
        console.log('After nextTick - displayEntries:', displayEntries.value.length)

        // 实时模式下的自动滚动
        if (realTimeMode.value && !isUserScrolling.value) {
          console.log('Auto-scrolling to bottom for real-time updates')
          // 简化异步调用链，减少延迟
          requestAnimationFrame(() => {
            autoScrollToBottom()
          })
        } else if (realTimeMode.value && isUserScrolling.value) {
          // 用户正在手动滚动，显示有新日志的提示
          console.log('User is scrolling, showing new logs notification')
          pendingNewLogs.value.push(...newEntries)
          hasNewLogsAvailable.value = true
        } else {
          // 非实时模式：不做任何操作，让 markDataChanged 的强制更新机制生效
          // markDataChanged('append') 已经通过微调 scrollTop (+0.1) 触发了虚拟滚动重新计算
          console.log('Not in real-time mode, relying on markDataChanged for viewport update')
        }
      })
    } else {
      console.log('No new entries after filtering (all duplicates)')
    }
  } else if (update.type === 'truncate') {
    console.log('File truncated, refreshing...')
    refresh()
  }
}

// 文件监控函数
const subscribeToFile = (filePath: string) => {
  const attemptSubscribe = () => {
    console.log('Attempting to subscribe to file:', filePath, 'WebSocket connected:', wsService.isConnected)
    if (wsService.isConnected) {
      console.log('Sending subscribe message for:', filePath)
      wsService.send({
        type: 'subscribe',
        path: filePath
      })
    } else {
      console.log('WebSocket not connected, retrying in 100ms')
      setTimeout(attemptSubscribe, 100)
    }
  }
  attemptSubscribe()
}

const unsubscribeFromFile = (filePath: string) => {
  wsService.send({
    type: 'unsubscribe',
    path: filePath
  })
}

// Watch for file changes - 重构支持新的数据加载模式
watch(() => props.file, (newFile, oldFile) => {
  if (oldFile?.path) {
    unsubscribeFromFile(oldFile.path)
  }

  if (newFile) {
    // 重置所有状态
    realTimeMode.value = false
    isUserScrolling.value = false
    formattedEntries.value = new Set()
    pendingNewLogs.value = []
    hasNewLogsAvailable.value = false
    scrollTop.value = 0
    isInitialLoad.value = true
    seenHashes.value.clear() // 切换文件时清空哈希集合

    // 重置新的状态变量
    earliestLineNum.value = null
    latestLineNum.value = null
    hasEarlierData.value = true
    hasLaterData.value = false
    isLoadingEarlier.value = false

    // 使用新的初始加载函数
    loadInitialLogs()

    setTimeout(() => {
      subscribeToFile(newFile.path)
    }, 500)
  } else {
    // 清空所有数据
    logEntries.value = []
    earliestLineNum.value = null
    latestLineNum.value = null
    totalLines.value = 0
    hasEarlierData.value = false
    hasLaterData.value = false
    formattedEntries.value = new Set()
    pendingNewLogs.value = []
    hasNewLogsAvailable.value = false
    scrollTop.value = 0
  }
}, { immediate: true })

// Watch for filter mode and search query changes
watch(() => [props.filterMode, props.searchQuery], async ([newFilterMode, newSearchQuery], [oldFilterMode, oldSearchQuery]) => {
  if (newFilterMode && newSearchQuery) {
    await executeSearch()
  } else {
    internalSearchResults.value = null

    // 🔧 修复:当退出过滤模式或清空搜索时,清理待处理日志队列
    // 因为这些日志可能已经在主日志列表中,避免重复显示
    if ((oldFilterMode && !newFilterMode) || (oldSearchQuery && !newSearchQuery)) {
      pendingNewLogs.value = []
      hasNewLogsAvailable.value = false

      // 如果当前在实时模式且不在手动滚动状态,自动滚动到底部
      if (realTimeMode.value && !isUserScrolling.value) {
        nextTick(() => {
          autoScrollToBottom()
        })
      }
    }
  }
})

// 获取容器高度
const updateContainerHeight = () => {
  if (containerRef.value) {
    containerHeight.value = containerRef.value.clientHeight
  }
}

// 滚动到特定匹配项（适应动态高度）
const scrollToMatch = (matchIndex: number) => {
  if (!containerRef.value || !internalSearchResults.value?.entries) return

  const entries = internalSearchResults.value.entries
  if (matchIndex < 0 || matchIndex >= entries.length) return

  // 计算到指定索引的累积高度
  let targetScrollTop = 0
  for (let i = 0; i < matchIndex; i++) {
    targetScrollTop += getItemHeight(i)
  }

  containerRef.value.scrollTop = targetScrollTop
  scrollTop.value = targetScrollTop
  isUserScrolling.value = false
}

// 更新defineExpose，移除jumpToLatest
defineExpose({
  scrollToMatch,
  refresh,
  toggleRealTimeMode,
  scrollToTop,
  scrollToBottom
})

// Lifecycle hooks
onMounted(() => {
  wsService.on('log_update', handleLogUpdate)
  applyDisplaySettings()
  updateContainerHeight()

  // 监听窗口大小变化
  window.addEventListener('resize', updateContainerHeight)
})

onUnmounted(() => {
  wsService.off('log_update', handleLogUpdate)
  window.removeEventListener('resize', updateContainerHeight)

  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
  }

  if (props.file?.path) {
    unsubscribeFromFile(props.file.path)
  }
})
</script>

<template>
  <div class="virtual-log-viewer" v-if="file">
    <!-- Toolbar - 与原LogViewer相同 -->
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

        <!-- <div class="log-stats">
          <span class="stat-item">总行数: {{ totalLines.toLocaleString() }}</span>
          <span class="stat-item">已加载: {{ logEntries.length.toLocaleString() }}</span>
          <span v-if="earliestLineNum && latestLineNum" class="stat-item range-info">
            行号范围: {{ earliestLineNum.toLocaleString() }} - {{ latestLineNum.toLocaleString() }}
          </span>
          <span v-if="internalSearchResults" class="stat-item search-info">
            搜索结果: {{ internalSearchResults.totalMatches.toLocaleString() }}
          </span>
          <span v-else-if="filterMode && searchQuery" class="stat-item filter-info">
            过滤显示: {{ filteredLogEntries.length.toLocaleString() }}
          </span>
          <span v-if="hasEarlierData" class="stat-item has-more">还有更早的数据...</span>
          <span v-if="isLoadingEarlier" class="stat-item loading-info">正在加载历史数据...</span>
          <span class="stat-item virtual-info">虚拟滚动已启用</span>
          <span class="stat-item mode-info">Tail -f 模式</span>
        </div> -->
      </div>

      <div class="toolbar-right">
        <el-tooltip content="滚动到顶部" placement="bottom">
          <el-button size="small" :icon="ArrowUp" @click="scrollToTop" />
        </el-tooltip>

        <el-tooltip content="滚动到底部（像tail -f）" placement="bottom">
          <el-button size="small" :icon="ArrowDown" @click="() => scrollToBottom(true)" />
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

    <!-- 虚拟滚动日志内容 -->
    <div
      ref="containerRef"
      class="virtual-log-container"
      @scroll="handleScroll"
      v-loading="loading && logEntries.length === 0"
    >
      <!-- 虚拟滚动内容 -->
      <div class="virtual-scroll-content" :style="{ height: visibleEntries.totalHeight + 'px' }">
        <div
          class="virtual-scroll-viewport"
          :style="{ transform: `translateY(${visibleEntries.offsetY}px)` }"
        >
          <div
            v-for="(entry, index) in visibleEntries.items"
            :key="visibleEntries.startIndex + index"
            :ref="el => { if (el) itemElements.set(visibleEntries.startIndex + index, el as HTMLElement) }"
            class="log-entry"
            :class="{
              'log-error': entry.level.toUpperCase() === 'ERROR',
              'log-warn': ['WARN', 'WARNING'].includes(entry.level.toUpperCase()),
              'log-info': entry.level.toUpperCase() === 'INFO',
              'log-debug': entry.level.toUpperCase() === 'DEBUG',
              'entry-hovered': hoveredEntryIndex === (visibleEntries.startIndex + index)
            }"
            @click="handleEntryClick(entry)"
            @mouseenter="hoveredEntryIndex = visibleEntries.startIndex + index"
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

              <el-button
                v-if="shouldShowFormattingButton(entry)"
                size="small"
                type="primary"
                :plain="!formattedEntries.has(visibleEntries.startIndex + index)"
                :icon="Document"
                @click.stop="toggleFormatting(visibleEntries.startIndex + index)"
                class="json-format-btn"
              >
                {{ formattedEntries.has(visibleEntries.startIndex + index) ? '原文' : '格式化' }}
              </el-button>

              <span class="entry-line-num" v-show="showLineNumbers">#{{ entry.lineNum }}</span>
            </div>

            <div class="entry-content">
              <div
                class="entry-message"
                :class="getDisplayType(entry, visibleEntries.startIndex + index)"
                v-html="getFormattedContent(entry, visibleEntries.startIndex + index)"
              />
            </div>
          </div>
        </div>
      </div>

      <!-- No results indicator -->
      <div v-if="filterMode && searchQuery && displayEntries.length === 0 && logEntries.length > 0" class="no-filter-results">
        <span v-if="internalSearchResults">没有找到匹配的搜索结果</span>
        <span v-else>没有匹配的日志条目</span>
      </div>

      <!-- Loading indicators - 重构支持新的加载模式 -->
      <div v-if="isLoadingEarlier" class="loading-more">
        <span>正在加载更早的数据...</span>
      </div>

      <div v-if="loading && logEntries.length > 0 && !isLoadingEarlier" class="loading-more">
        <span>正在加载...</span>
      </div>

      <!-- Data status indicators -->
      <div v-if="!hasEarlierData && logEntries.length > 0 && !loading" class="no-more-data">
        <span>已显示所有日志（从文件开头）</span>
      </div>
    </div>
  </div>

  <!-- Empty state -->
  <div v-else class="log-viewer-empty">
    <el-empty description="请选择一个日志文件开始查看" />
  </div>
</template>

<style scoped>
.virtual-log-viewer {
  height: 100%;
  display: flex;
  flex-direction: column;
  background-color: var(--color-background);
}

/* Toolbar styles - 与原LogViewer相同 */
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

.virtual-info {
  color: #67c23a;
  font-weight: 500;
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

/* 虚拟滚动容器 */
.virtual-log-container {
  flex: 1;
  overflow: auto;
  background-color: #323232;
  color: #ffffff;
  font-family: var(--log-font-family, 'Consolas', 'Monaco', 'Courier New', monospace);
  font-size: var(--log-font-size, 14px);
  line-height: var(--log-line-height, 1.5);
  position: relative;
}

.virtual-scroll-content {
  position: relative;
  will-change: transform;
}

.virtual-scroll-viewport {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  will-change: transform;
}

/* 日志条目样式 - 支持动态高度 */
.log-entry {
  padding: 12px 16px;
  border-bottom: 1px solid #2d2d2d;
  cursor: pointer;
  transition: background-color 0.2s;
  display: flex;
  flex-direction: column;
  gap: 8px;
  box-sizing: border-box;
  /* 移除固定高度，支持动态高度 */
  min-height: 60px;
  /* 内容适应高度 */
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
  min-height: 28px; /* 增加最小高度 */
  padding-bottom: 6px; /* 添加底部边距 */
}

.entry-timestamp {
  color: #858585;
  font-size: 12px;
  font-weight: 500;
  min-width: 140px;
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

.entry-hovered {
  background-color: rgba(255, 255, 255, 0.05);
  border-left: 3px solid #409eff;
  padding-left: 13px;
  transition: all 0.2s ease;
}

.entry-content {
  flex: 1;
  min-width: 0;
  min-height: 40px; /* 增加最小高度给内容区域 */
  display: flex;
  flex-direction: column;
}

.entry-message {
  color: #ffffff;
  word-break: var(--word-wrap, break-word);
  white-space: pre-wrap;
  line-height: 1.6; /* 增加行高 */
  font-size: var(--log-font-size, 14px);
  padding: 4px 0; /* 添加上下内边距 */
  min-height: 22px; /* 确保最小高度 */
}

/* JSON格式化按钮 */
.json-format-btn {
  font-size: 11px;
  height: 22px;
  padding: 0 10px;
  margin-left: 8px;
  min-width: 76px;
}

/* 格式化显示样式 - 与原LogViewer相同 */
.entry-message.raw-text {
  color: #ffffff;
  word-break: var(--word-wrap, break-word);
  white-space: pre-wrap;
  line-height: var(--log-line-height, 1.5);
  font-size: var(--log-font-size, 14px);
}

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

/* 状态指示器 - 使用绝对定位避免影响滚动高度 */
.loading-more,
.no-more-data,
.no-filter-results {
  position: absolute;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 12px 20px;
  color: #909399;
  background-color: rgba(45, 45, 45, 0.95);
  font-size: 12px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(4px);
  z-index: 10;
  pointer-events: none; /* 避免阻塞滚动操作 */
}

.range-info {
  color: #67c23a;
  font-weight: 500;
}

.loading-info {
  color: #e6a23c;
  font-weight: 500;
}

.mode-info {
  color: #f56c6c;
  font-weight: 600;
  background-color: rgba(245, 108, 108, 0.1);
  padding: 2px 6px;
  border-radius: 3px;
}

.no-filter-results {
  color: #e6a23c;
  background-color: rgba(230, 162, 60, 0.1);
  border-color: rgba(230, 162, 60, 0.3);
}

.log-viewer-empty {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 搜索高亮 */
:deep(.search-highlight) {
  background-color: #ffeb3b;
  color: #000;
  padding: 1px 2px;
  border-radius: 2px;
}

/* JSON语法高亮 */
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

/* 滚动条样式 */
.virtual-log-container::-webkit-scrollbar {
  width: 8px;
}

.virtual-log-container::-webkit-scrollbar-track {
  background: #2d2d2d;
}

.virtual-log-container::-webkit-scrollbar-thumb {
  background: #555;
  border-radius: 4px;
}

.virtual-log-container::-webkit-scrollbar-thumb:hover {
  background: #777;
}
</style>
