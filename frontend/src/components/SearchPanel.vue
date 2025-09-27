<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElInput, ElButton, ElCheckbox, ElSelect, ElOption, ElDatePicker, ElTag, ElPagination, ElMessage } from 'element-plus'
import { Search, Refresh, Close } from '@element-plus/icons-vue'
import apiService, { type SearchQuery, type SearchResult, type LogEntry } from '../services/api'

// Props
interface Props {
  currentFilePath?: string
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  currentFilePath: '',
  disabled: false
})

// Emits
const emit = defineEmits<{
  searchResults: [results: SearchResult]
  entrySelect: [entry: LogEntry]
  searchStateChange: [isSearching: boolean]
  searchQueryChange: [query: string]
}>()

// Reactive state
const searchQuery = ref('')
const isRegexMode = ref(false)
const selectedLevels = ref<string[]>([])
const startTime = ref<Date | null>(null)
const endTime = ref<Date | null>(null)
const currentPage = ref(1)
const pageSize = ref(50)
const loading = ref(false)
const searchResults = ref<SearchResult | null>(null)
const searchHistory = ref<string[]>([])

// Available log levels
const logLevels = [
  { label: 'ERROR', value: 'ERROR', color: '#f56c6c' },
  { label: 'WARN', value: 'WARN', color: '#e6a23c' },
  { label: 'INFO', value: 'INFO', color: '#409eff' },
  { label: 'DEBUG', value: 'DEBUG', color: '#909399' },
  { label: 'TRACE', value: 'TRACE', color: '#c0c4cc' }
]

// Computed properties
const hasSearchQuery = computed(() => searchQuery.value.trim().length > 0)
const hasFilters = computed(() => 
  selectedLevels.value.length > 0 || startTime.value || endTime.value
)
const hasResults = computed(() => searchResults.value && searchResults.value.entries.length > 0)
const totalPages = computed(() => {
  if (!searchResults.value) return 0
  return Math.ceil(searchResults.value.totalMatches / pageSize.value)
})

// Search functionality
const performSearch = async (page = 1) => {
  if (!props.currentFilePath || !hasSearchQuery.value) {
    ElMessage.warning('请选择日志文件并输入搜索关键词')
    return
  }

  loading.value = true
  emit('searchStateChange', true)
  emit('searchQueryChange', searchQuery.value.trim())

  try {
    const query: SearchQuery = {
      path: props.currentFilePath,
      query: searchQuery.value.trim(),
      isRegex: isRegexMode.value,
      startTime: startTime.value?.toISOString(),
      endTime: endTime.value?.toISOString(),
      levels: selectedLevels.value.length > 0 ? selectedLevels.value : undefined,
      offset: (page - 1) * pageSize.value,
      limit: pageSize.value
    }

    console.log('Performing search with query:', {
      path: query.path,
      query: query.query,
      isRegex: query.isRegex,
      startTime: query.startTime,
      endTime: query.endTime,
      levels: query.levels,
      offset: query.offset,
      limit: query.limit
    })

    const result = await apiService.searchLogs(query)
    searchResults.value = result
    currentPage.value = page

    // Add to search history
    if (!searchHistory.value.includes(searchQuery.value.trim())) {
      searchHistory.value.unshift(searchQuery.value.trim())
      if (searchHistory.value.length > 10) {
        searchHistory.value = searchHistory.value.slice(0, 10)
      }
    }

    emit('searchResults', result)

    if (result.entries.length === 0) {
      ElMessage.info('未找到匹配的日志条目')
    } else {
      ElMessage.success(`找到 ${result.totalMatches} 条匹配记录`)
    }

  } catch (error) {
    console.error('Search failed:', error)
    ElMessage.error('搜索失败，请检查搜索条件')
  } finally {
    loading.value = false
    emit('searchStateChange', false)
  }
}

// Clear search results
const clearSearch = () => {
  searchQuery.value = ''
  searchResults.value = null
  currentPage.value = 1
  emit('searchResults', { entries: [], totalMatches: 0, hasMore: false, offset: 0 })
  emit('searchQueryChange', '')
}

// Clear all filters
const clearFilters = () => {
  selectedLevels.value = []
  startTime.value = null
  endTime.value = null
  isRegexMode.value = false
}

// Handle filter changes (auto-apply if there's a search query)
const onFiltersChange = () => {
  if (hasSearchQuery.value) {
    // Auto-apply filters when there's an active search
    performSearch(1)
  }
}

// Handle page change
const handlePageChange = (page: number) => {
  performSearch(page)
}

// Handle entry click
const handleEntryClick = (entry: LogEntry) => {
  emit('entrySelect', entry)
}

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

// Get log level color
const getLogLevelColor = (level: string): string => {
  const levelConfig = logLevels.find(l => l.value === level.toUpperCase())
  return levelConfig?.color || '#909399'
}

// Highlight search terms in text
const highlightText = (text: string, query: string): string => {
  if (!query) return text
  
  try {
    const flags = isRegexMode.value ? 'gi' : 'gi'
    const searchPattern = isRegexMode.value ? query : query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const regex = new RegExp(`(${searchPattern})`, flags)
    return text.replace(regex, '<mark class="search-highlight">$1</mark>')
  } catch (error) {
    // If regex is invalid, return original text
    return text
  }
}

// Watch for file path changes
watch(() => props.currentFilePath, () => {
  if (searchResults.value) {
    clearSearch()
  }
})

// Expose methods for testing
defineExpose({
  performSearch,
  clearSearch,
  clearFilters,
  handleEntryClick
})
</script>

<template>
  <div class="search-panel">
    <!-- Search Input Section -->
    <div class="search-section">
      <div class="search-input-group">
        <el-input
          v-model="searchQuery"
          placeholder="输入搜索关键词..."
          class="search-input"
          :disabled="disabled || !currentFilePath"
          @keyup.enter="performSearch(1)"
          clearable
        >
          <template #append>
            <el-button
              :icon="Search"
              :loading="loading"
              :disabled="!hasSearchQuery || disabled"
              @click="performSearch(1)"
              data-testid="search-button"
            />
          </template>
        </el-input>
        
        <el-button
          v-if="searchResults"
          :icon="Close"
          size="small"
          @click="clearSearch"
          title="清除搜索"
          data-testid="clear-search"
        />
      </div>

      <!-- Search History -->
      <div v-if="searchHistory.length > 0" class="search-history">
        <span class="history-label">历史搜索:</span>
        <el-tag
          v-for="(query, index) in searchHistory.slice(0, 5)"
          :key="index"
          size="small"
          class="history-tag"
          @click="searchQuery = query"
        >
          {{ query }}
        </el-tag>
      </div>
    </div>

    <!-- Filters Section -->
    <div class="filters-section">
      <div class="filter-group">
        <label class="filter-label">过滤:</label>
        <el-checkbox v-model="isRegexMode" :disabled="disabled" data-testid="regex-checkbox">
          正则
        </el-checkbox>
      </div>

      <div class="filter-group">
        <label class="filter-label">日志级别:</label>
        <el-select
          v-model="selectedLevels"
          multiple
          placeholder="选择日志级别"
          class="level-select"
          :disabled="disabled"
          collapse-tags
          collapse-tags-tooltip
          @change="onFiltersChange"
        >
          <el-option
            v-for="level in logLevels"
            :key="level.value"
            :label="level.label"
            :value="level.value"
          >
            <span :style="{ color: level.color }">{{ level.label }}</span>
          </el-option>
        </el-select>
      </div>

      <div class="filter-group">
        <label class="filter-label">时间范围:</label>
        <el-date-picker
          v-model="startTime"
          type="datetime"
          placeholder="开始时间"
          class="time-picker"
          :disabled="disabled"
          format="YYYY-MM-DD HH:mm:ss"
          @change="onFiltersChange"
        />
        <span class="time-separator">至</span>
        <el-date-picker
          v-model="endTime"
          type="datetime"
          placeholder="结束时间"
          class="time-picker"
          :disabled="disabled"
          format="YYYY-MM-DD HH:mm:ss"
          @change="onFiltersChange"
        />
      </div>

      <div class="filter-actions">
        <el-button
          size="small"
          type="primary"
          :disabled="!hasSearchQuery || disabled"
          @click="performSearch(1)"
          data-testid="apply-filters"
        >
          应用筛选
        </el-button>

        <el-button
          size="small"
          :icon="Refresh"
          :disabled="!hasFilters || disabled"
          @click="clearFilters"
          data-testid="clear-filters"
        >
          清除筛选
        </el-button>
      </div>
    </div>

    <!-- Search Results Section -->
    <div v-if="searchResults" class="results-section">
      <div class="results-header">
        <div class="results-info">
          <span class="results-count">
            找到 {{ searchResults.totalMatches }} 条结果
          </span>
          <span v-if="searchResults.hasMore" class="has-more-indicator">
            (显示部分结果)
          </span>
        </div>
        
        <!-- Pagination -->
        <el-pagination
          v-if="totalPages > 1"
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="searchResults.totalMatches"
          layout="prev, pager, next, jumper"
          class="results-pagination"
          small
          @current-change="handlePageChange"
        />
      </div>

      <!-- Results List -->
      <div class="results-list">
        <div
          v-for="(entry, index) in searchResults.entries"
          :key="`${entry.lineNum}-${index}`"
          class="result-item"
          :class="{
            'result-error': entry.level.toUpperCase() === 'ERROR',
            'result-warn': ['WARN', 'WARNING'].includes(entry.level.toUpperCase()),
            'result-info': entry.level.toUpperCase() === 'INFO',
            'result-debug': entry.level.toUpperCase() === 'DEBUG'
          }"
          @click="handleEntryClick(entry)"
        >
          <div class="result-header">
            <span class="result-timestamp">{{ formatTimestamp(entry.timestamp) }}</span>
            <el-tag
              :color="getLogLevelColor(entry.level)"
              size="small"
              class="result-level"
              effect="dark"
            >
              {{ entry.level.toUpperCase() }}
            </el-tag>
            <span class="result-line-num">#{{ entry.lineNum }}</span>
          </div>
          
          <div class="result-content">
            <div
              class="result-message"
              v-html="highlightText(entry.message, searchQuery)"
            />
            
            <!-- Structured fields for JSON logs -->
            <div v-if="entry.fields && Object.keys(entry.fields).length > 0" class="result-fields">
              <div
                v-for="[key, value] in Object.entries(entry.fields).slice(0, 3)"
                :key="key"
                class="field-item"
              >
                <span class="field-key">{{ key }}:</span>
                <span class="field-value">{{ JSON.stringify(value) }}</span>
              </div>
              <div v-if="Object.keys(entry.fields).length > 3" class="field-more">
                +{{ Object.keys(entry.fields).length - 3 }} more fields
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Bottom Pagination -->
      <div v-if="totalPages > 1" class="results-footer">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="searchResults.totalMatches"
          layout="total, prev, pager, next, sizes"
          :page-sizes="[20, 50, 100, 200]"
          @current-change="handlePageChange"
          @size-change="(size) => { pageSize = size; performSearch(1) }"
        />
      </div>
    </div>

    <!-- Empty State -->
    <div v-else-if="!currentFilePath" class="empty-state">
      <el-empty description="请先选择一个日志文件" />
    </div>
  </div>
</template>

<style scoped>
.search-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  background-color: #ffffff;
  border-radius: 4px;
  overflow: hidden;
}

/* Search Section */
.search-section {
  padding: 16px;
  border-bottom: 1px solid #e4e7ed;
  background-color: #f8f9fa;
}

.search-input-group {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.search-input {
  flex: 1;
}

.search-history {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.history-label {
  font-size: 12px;
  color: #606266;
  white-space: nowrap;
}

.history-tag {
  cursor: pointer;
  transition: all 0.2s;
}

.history-tag:hover {
  background-color: #409eff;
  color: white;
}

/* Filters Section */
.filters-section {
  padding: 16px;
  border-bottom: 1px solid #e4e7ed;
  background-color: #fafafa;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.filter-group:last-child {
  margin-bottom: 0;
}

.filter-label {
  font-size: 13px;
  color: #606266;
  white-space: nowrap;
  min-width: 60px;
}

.level-select {
  width: 200px;
}

.time-picker {
  width: 160px;
}

.time-separator {
  font-size: 12px;
  color: #909399;
  margin: 0 4px;
}

.filter-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}

/* Results Section */
.results-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.results-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #e4e7ed;
  background-color: #f8f9fa;
}

.results-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.results-count {
  font-size: 13px;
  font-weight: 500;
  color: #303133;
}

.has-more-indicator {
  font-size: 12px;
  color: #909399;
}

.results-pagination {
  margin: 0;
}

/* Results List */
.results-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.result-item {
  padding: 8px 12px;
  margin-bottom: 8px;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
  background-color: #ffffff;
}

.result-item:hover {
  border-color: #409eff;
  box-shadow: 0 2px 4px rgba(64, 158, 255, 0.1);
}

.result-item.result-error {
  border-left: 3px solid #f56c6c;
}

.result-item.result-warn {
  border-left: 3px solid #e6a23c;
}

.result-item.result-info {
  border-left: 3px solid #409eff;
}

.result-item.result-debug {
  border-left: 3px solid #909399;
}

.result-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.result-timestamp {
  font-size: 11px;
  color: #909399;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
}

.result-level {
  font-size: 10px;
  height: 16px;
  line-height: 14px;
  padding: 0 4px;
  min-width: 45px;
  text-align: center;
}

.result-line-num {
  font-size: 10px;
  color: #c0c4cc;
  margin-left: auto;
}

.result-content {
  margin-left: 4px;
}

.result-message {
  font-size: 13px;
  line-height: 1.4;
  color: #303133;
  word-break: break-all;
  margin-bottom: 4px;
}

.result-fields {
  font-size: 11px;
  color: #606266;
  background-color: #f8f9fa;
  padding: 4px 8px;
  border-radius: 2px;
  margin-top: 4px;
}

.field-item {
  display: inline-block;
  margin-right: 12px;
  margin-bottom: 2px;
}

.field-key {
  font-weight: 500;
  color: #409eff;
}

.field-value {
  margin-left: 2px;
  color: #606266;
}

.field-more {
  color: #909399;
  font-style: italic;
}

/* Results Footer */
.results-footer {
  padding: 12px 16px;
  border-top: 1px solid #e4e7ed;
  background-color: #f8f9fa;
  display: flex;
  justify-content: center;
}

/* Empty State */
.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Search Highlight */
:deep(.search-highlight) {
  background-color: #ffeb3b;
  color: #000;
  padding: 1px 2px;
  border-radius: 2px;
  font-weight: 500;
}

/* Scrollbar styling */
.results-list::-webkit-scrollbar {
  width: 6px;
}

.results-list::-webkit-scrollbar-track {
  background: #f1f1f1;
}

.results-list::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 3px;
}

.results-list::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

/* Responsive design */
@media (max-width: 768px) {
  .filter-group {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .results-header {
    flex-direction: column;
    gap: 8px;
    align-items: flex-start;
  }
  
  .time-picker {
    width: 140px;
  }
}
</style>