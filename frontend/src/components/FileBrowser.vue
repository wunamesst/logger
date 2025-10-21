<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElTree, ElIcon, ElTag, ElTooltip, ElMessage } from 'element-plus'
import { Document, Folder, FolderOpened, Refresh, Search } from '@element-plus/icons-vue'
import apiService, { type LogFile } from '../services/api'
import wsService from '../services/websocket'

// Props and emits
const emit = defineEmits<{
  fileSelect: [file: LogFile]
}>()

// Reactive data
const logFiles = ref<LogFile[]>([])
const loading = ref(false)
const selectedPath = ref<string>('')
const expandedKeys = ref<string[]>([])
const filterText = ref<string>('')

// 懒加载模式标志
const lazyMode = ref(true)

// Tree props configuration
const treeProps = {
  label: 'name',
  children: 'children',
  isLeaf: (data: LogFile) => !data.isDirectory
}

// 递归过滤文件树节点
const filterTreeNode = (node: LogFile, keyword: string): LogFile | null => {
  const lowerKeyword = keyword.toLowerCase()
  const nameMatches = node.name.toLowerCase().includes(lowerKeyword)

  // 如果是文件且名称匹配,直接返回
  if (!node.isDirectory && nameMatches) {
    return { ...node }
  }

  // 如果是目录,递归检查子节点
  if (node.isDirectory && node.children) {
    const filteredChildren = node.children
      .map(child => filterTreeNode(child, keyword))
      .filter(child => child !== null) as LogFile[]

    // 如果有匹配的子节点,或目录名本身匹配,返回该目录
    if (filteredChildren.length > 0 || nameMatches) {
      return {
        ...node,
        children: filteredChildren
      }
    }
  }

  // 目录名匹配但没有子节点
  if (node.isDirectory && nameMatches) {
    return { ...node, children: [] }
  }

  return null
}

// 收集需要展开的节点路径
const collectExpandedKeys = (nodes: LogFile[], keys: string[] = []): string[] => {
  nodes.forEach(node => {
    if (node.isDirectory) {
      keys.push(node.path)
      if (node.children && node.children.length > 0) {
        collectExpandedKeys(node.children, keys)
      }
    }
  })
  return keys
}

// Computed properties
const isLazyMode = computed(() => {
  return lazyMode.value && !filterText.value.trim()
})

const treeData = computed(() => {
  if (!filterText.value.trim()) {
    return logFiles.value
  }

  // 搜索模式下,如果当前是懒加载模式,需要切换到完整模式
  if (lazyMode.value) {
    // 异步加载完整数据
    loadFullTreeForSearch()
    return logFiles.value
  }

  const filtered = logFiles.value
    .map(node => filterTreeNode(node, filterText.value.trim()))
    .filter(node => node !== null) as LogFile[]

  // 自动展开所有匹配结果的父目录
  if (filtered.length > 0) {
    expandedKeys.value = collectExpandedKeys(filtered)
  }

  return filtered
})

// 为搜索加载完整树
const loadFullTreeForSearch = async () => {
  if (!lazyMode.value) return // 已经是完整模式

  lazyMode.value = false
  await loadLogFiles()
}

// 统计匹配的文件数量
const matchedFileCount = computed(() => {
  if (!filterText.value.trim()) return 0

  const countFiles = (nodes: LogFile[]): number => {
    let count = 0
    nodes.forEach(node => {
      if (!node.isDirectory) {
        count++
      }
      if (node.children) {
        count += countFiles(node.children)
      }
    })
    return count
  }

  return countFiles(treeData.value)
})

// Format file size for display
const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const size = bytes / Math.pow(k, i)
  return (size % 1 === 0 ? size.toString() : size.toFixed(1)) + ' ' + sizes[i]
}

// Format modification time
const formatModTime = (modTime: string): string => {
  const date = new Date(modTime)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

// Get file status for styling
const getFileStatus = (file: LogFile): string => {
  const now = Date.now()
  const fileTime = new Date(file.modTime).getTime()
  const hourAgo = now - (60 * 60 * 1000)
  const dayAgo = now - (24 * 60 * 60 * 1000)

  // Recent file (modified within last hour)
  if (fileTime > hourAgo) {
    return 'recent'
  }

  // Large file (>10MB)
  if (file.size > 10 * 1024 * 1024) {
    return 'large'
  }

  return 'normal'
}

// Add isLeaf property to file data
const addIsLeafProperty = (files: LogFile[]): LogFile[] => {
  return files.map(file => ({
    ...file,
    isLeaf: !file.isDirectory,
    children: file.children ? addIsLeafProperty(file.children) : undefined
  }))
}

// Load log files from API
const loadLogFiles = async () => {
  loading.value = true
  try {
    if (lazyMode.value) {
      // 懒加载模式: 只加载根目录
      const files = await apiService.getDirectoryFiles('')
      logFiles.value = files
    } else {
      // 完整加载模式: 加载整个树(用于搜索)
      const files = await apiService.getLogFiles()
      logFiles.value = addIsLeafProperty(files)
    }

    // Don't auto-expand directories - let user manually expand them
    expandedKeys.value = []

  } catch (error) {
    console.error('Failed to load log files:', error)
    ElMessage.error('加载日志文件失败')
  } finally {
    loading.value = false
  }
}

// 懒加载子节点
const loadNode = async (node: any, resolve: (data: LogFile[]) => void) => {
  if (node.level === 0) {
    // 根节点,返回已加载的根目录列表
    return resolve(logFiles.value)
  }

  if (node.data.isDirectory) {
    // 加载子目录
    try {
      const children = await apiService.getDirectoryFiles(node.data.path)
      resolve(children)
    } catch (error) {
      console.error('Failed to load directory:', error)
      ElMessage.error(`加载目录失败: ${node.data.name}`)
      resolve([])
    }
  } else {
    // 文件节点没有子节点
    resolve([])
  }
}

// Handle tree node click
const handleNodeClick = (data: LogFile) => {
  // Only handle file selection for non-directories
  // Directory expansion is handled automatically by el-tree when expand-on-click-node is true
  if (!data.isDirectory) {
    selectedPath.value = data.path
    emit('fileSelect', data)
  }
}

// Handle refresh button click
const handleRefresh = () => {
  loadLogFiles()
}

// Handle search clear
const handleClearSearch = async () => {
  filterText.value = ''
  expandedKeys.value = []

  // 切换回懒加载模式
  if (!lazyMode.value) {
    lazyMode.value = true
    await loadLogFiles()
  }
}

// Listen for WebSocket file updates
const handleFileUpdate = (message: any) => {
  if (message.type === 'file_update') {
    // Refresh file list when files are updated
    loadLogFiles()
  }
}

// Lifecycle hooks
onMounted(() => {
  loadLogFiles()
  
  // Listen for WebSocket updates
  wsService.on('message', handleFileUpdate)
})

// Expose methods for testing
defineExpose({
  handleNodeClick,
  handleRefresh,
  formatFileSize,
  formatModTime
})
</script>

<template>
  <div class="file-browser elevated-surface">
    <div class="browser-header glass-effect">
      <div class="header-top">
        <div class="header-title">
          <div class="title-icon">
            <el-icon><Folder /></el-icon>
          </div>
          <div class="title-content">
            <h3 class="title-text">日志文件</h3>
            <p class="title-subtitle">浏览和选择日志文件</p>
          </div>
        </div>

        <div class="header-actions">
          <el-tooltip content="刷新文件列表" placement="bottom">
            <el-button
              class="refresh-btn"
              size="small"
              :icon="Refresh"
              :loading="loading"
              @click="handleRefresh"
              circle
            />
          </el-tooltip>
        </div>
      </div>

      <div class="header-search">
        <el-input
          v-model="filterText"
          placeholder="搜索文件名..."
          class="search-input"
          size="small"
          clearable
          @clear="handleClearSearch"
        >
          <template #prefix>
            <el-icon class="search-icon">
              <Search />
            </el-icon>
          </template>
        </el-input>
      </div>
    </div>

    <div class="browser-content">
      <div class="content-wrapper">
        <el-tree
          :key="lazyMode ? 'lazy' : 'full'"
          v-loading="loading"
          :data="isLazyMode ? logFiles : treeData"
          :props="treeProps"
          :load="isLazyMode ? loadNode : undefined"
          :lazy="isLazyMode"
          :indent="20"
          :expand-on-click-node="true"
          :highlight-current="true"
          :current-node-key="selectedPath"
          :expanded-keys="!isLazyMode ? expandedKeys : undefined"
          node-key="path"
          @node-click="handleNodeClick"
          class="file-tree"
          element-loading-text="正在加载文件列表..."
          element-loading-spinner="el-icon-loading"
        >
          <template #default="{ node, data }">
            <div class="tree-node" :class="{ 'is-file': !data.isDirectory, 'is-selected': selectedPath === data.path }">
              <div class="node-content">
                <div class="node-icon-wrapper">
                  <el-icon class="node-icon" :class="{ 'folder-open': data.isDirectory && node.expanded }">
                    <FolderOpened v-if="data.isDirectory && node.expanded" />
                    <Folder v-else-if="data.isDirectory" />
                    <Document v-else />
                  </el-icon>
                </div>

                <div class="node-details">
                  <span class="node-label" :title="data.name">
                    {{ data.name }}
                  </span>
                  <div v-if="!data.isDirectory" class="file-size-info">
                    <span class="file-size">{{ formatFileSize(data.size) }}</span>
                  </div>
                </div>
              </div>

              <!-- File status indicator -->
              <div v-if="!data.isDirectory" class="node-status">
                <div class="status-indicator" :class="getFileStatus(data)"></div>
              </div>
            </div>
          </template>
        </el-tree>
      </div>
    </div>
  </div>
</template>

<style scoped>
.file-browser {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--app-bg-secondary);
  border-radius: var(--radius-lg);
  overflow: hidden;
  border: 1px solid var(--app-border-color);
  box-shadow: var(--shadow-sm);
  transition: all var(--transition-normal);
}

.file-browser:hover {
  box-shadow: var(--shadow-md);
  border-color: var(--app-border-hover);
}

.browser-header {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  padding: var(--space-lg) var(--space-lg) var(--space-md);
  border-bottom: 1px solid var(--app-border-color);
  background: var(--app-bg-secondary);
  gap: var(--space-md);
}

.header-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-title {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-shrink: 0;
}

.title-icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--primary-500);
  border-radius: var(--radius-lg);
  color: white;
  box-shadow: var(--shadow-sm);
  transition: all var(--transition-normal);
}

.title-icon:hover {
  transform: translateY(-1px);
  box-shadow: var(--shadow-md);
  background: var(--primary-600);
}

.title-content {
  flex: 1;
}

.title-text {
  margin: 0;
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-semibold);
  color: var(--app-text-color);
  line-height: var(--line-height-tight);
}

.title-subtitle {
  margin: 0;
  font-size: var(--font-size-xs);
  color: var(--app-text-muted);
  line-height: var(--line-height-normal);
  margin-top: var(--space-xs);
}

.header-search {
  width: 100%;
}

.search-input {
  width: 100%;
}

:deep(.search-input .el-input__wrapper) {
  background: var(--app-bg-tertiary);
  border: 1px solid var(--app-border-color);
  border-radius: var(--radius-md);
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
  font-size: 14px;
}

.header-actions {
  display: flex;
  gap: var(--space-sm);
  flex-shrink: 0;
}

.refresh-btn {
  background: var(--app-bg-tertiary);
  border: 1px solid var(--app-border-color);
  color: var(--app-text-color);
  transition: all var(--transition-normal);
}

.refresh-btn:hover {
  background: var(--primary-50);
  border-color: var(--primary-200);
  color: var(--primary-600);
  transform: translateY(-1px);
  box-shadow: var(--shadow-sm);
}

.browser-content {
  flex: 1;
  overflow: hidden;
  position: relative;
  background: var(--app-bg-secondary);
}

.content-wrapper {
  height: 100%;
  overflow: auto;
  padding: var(--space-sm);
}

.file-tree {
  background-color: transparent;
}

.tree-node {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  border-radius: var(--radius-md);
  transition: all var(--transition-fast);
  margin-bottom: var(--space-xs);
  border: 1px solid transparent;
}




.node-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  flex: 1;
  min-width: 0;
}

.node-icon-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  transition: all var(--transition-fast);
}

.node-icon {
  color: var(--app-text-muted);
  transition: all var(--transition-fast);
  font-size: 18px;
}

.node-icon.folder-open {
  color: var(--primary-500);
}

.tree-node:hover .node-icon {
  color: var(--app-text-color);
}

.tree-node.is-selected .node-icon {
  color: var(--primary-600);
}

.node-details {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
}

.node-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--app-text-color);
  line-height: var(--line-height-tight);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color var(--transition-fast);
}

.tree-node:hover .node-label {
  color: var(--primary-600);
}

.tree-node.is-selected .node-label {
  color: var(--primary-700);
  font-weight: var(--font-weight-semibold);
}


.file-size-info {
  display: flex;
  align-items: center;
}

.file-size {
  font-size: var(--font-size-xs);
  color: var(--app-text-muted);
  font-weight: var(--font-weight-medium);
}

.node-status {
  display: flex;
  align-items: center;
  margin-left: var(--space-sm);
}

.status-indicator {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--success-400);
  box-shadow: 0 0 4px rgba(34, 197, 94, 0.3);
}

.status-indicator.large {
  background: var(--warning-400);
  box-shadow: 0 0 4px rgba(245, 158, 11, 0.3);
}

.status-indicator.recent {
  background: var(--info-400);
  box-shadow: 0 0 4px rgba(59, 130, 246, 0.3);
  animation: pulse 2s infinite;
}

/* Tree component deep styling */
:deep(.el-tree-node__content) {
  padding-top: 0 !important;
  padding-bottom: 0 !important;
  background: transparent !important;
  border-radius: var(--radius-md);
  transition: all var(--transition-fast);
}

:deep(.el-tree-node__content:hover) {
  background: transparent !important;
}

:deep(.el-tree-node__content.is-current) {
  background: transparent !important;
}

:deep(.el-tree-node__expand-icon) {
  color: var(--app-text-muted);
  transition: all var(--transition-fast);
  border-radius: var(--radius-sm);
  padding: 2px;
}

:deep(.el-tree-node__expand-icon:hover) {
  background: var(--app-bg-tertiary);
  color: var(--primary-500);
}

:deep(.el-tree-node__expand-icon.is-leaf) {
  color: transparent;
}

/* Loading state enhancement */
:deep(.el-loading-mask) {
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(4px);
  border-radius: var(--radius-lg);
}


:deep(.el-loading-text) {
  color: var(--app-text-color);
  font-weight: var(--font-weight-medium);
}

/* Scrollbar styling */
.content-wrapper::-webkit-scrollbar {
  width: 6px;
}

.content-wrapper::-webkit-scrollbar-track {
  background: var(--app-bg-tertiary);
  border-radius: var(--radius-sm);
}

.content-wrapper::-webkit-scrollbar-thumb {
  background: var(--app-border-hover);
  border-radius: var(--radius-sm);
  transition: all var(--transition-fast);
}

.content-wrapper::-webkit-scrollbar-thumb:hover {
  background: var(--primary-400);
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .browser-header {
    padding: var(--space-md);
  }

  .title-icon {
    width: 32px;
    height: 32px;
  }

  .title-text {
    font-size: var(--font-size-base);
  }

  .title-subtitle {
    display: none;
  }

  .content-wrapper {
    padding: var(--space-sm);
  }
}

/* Animation for recent files */
@keyframes pulse {
  0%, 100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.7;
    transform: scale(1.2);
  }
}
</style>