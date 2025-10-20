<script setup lang="ts">
import { ref, reactive, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Setting,
  Monitor,
  Refresh,
  Document,
  Timer,
  DataBoard
} from '@element-plus/icons-vue'
import { useSettings } from '../composables/useSettings'

// Settings interface
interface DisplaySettings {
  colorScheme: 'system' | 'blue-light' | 'green-light' | 'purple-light' | 'orange-light' | 'gray-light' | 'cyan-light' | 'amber-light'
  fontSize: number
  lineHeight: number
  fontFamily: string
  showLineNumbers: boolean
  wordWrap: boolean
}

interface PerformanceSettings {
  cacheSize: number
  maxLogLines: number
  enableVirtualScroll: boolean
  preloadLines: number
}

interface Settings {
  display: DisplaySettings
  performance: PerformanceSettings
}

// Default settings
const defaultSettings: Settings = {
  display: {
    colorScheme: 'system',
    fontSize: 14,
    lineHeight: 1.5,
    fontFamily: 'Consolas, Monaco, "Courier New", monospace',
    showLineNumbers: true,
    wordWrap: true
  },
  performance: {
    cacheSize: 50, // MB
    maxLogLines: 10000,
    enableVirtualScroll: true,
    preloadLines: 100
  }
}

// Reactive settings
const settings = reactive<Settings>(JSON.parse(JSON.stringify(defaultSettings)))

// Global settings state
const { isSettingsPanelVisible, hideSettings } = useSettings()

const activeTab = ref('display')

// Font family options
const fontFamilyOptions = [
  { label: 'Consolas', value: 'Consolas, Monaco, "Courier New", monospace' },
  { label: 'Monaco', value: 'Monaco, "Lucida Console", monospace' },
  { label: 'Courier New', value: '"Courier New", Courier, monospace' },
  { label: 'Source Code Pro', value: '"Source Code Pro", monospace' },
  { label: 'Fira Code', value: '"Fira Code", monospace' },
  { label: 'JetBrains Mono', value: '"JetBrains Mono", monospace' }
]

// Color scheme options
const colorSchemeOptions = [
  { id: 'system', name: '跟随系统', description: '自动适应系统设置' },
  { id: 'blue-light', name: '经典蓝色', description: '经典专业，稳重可靠' },
  { id: 'green-light', name: '森林绿色', description: '护眼清新，自然专业' },
  { id: 'purple-light', name: '深紫色', description: '现代科技，创新智慧' },
  { id: 'orange-light', name: '橙红色', description: '活力专注，醒目警觉' },
  { id: 'gray-light', name: '石墨灰', description: '极简高级，沉稳专业' },
  { id: 'cyan-light', name: '青绿色', description: '清新现代，冷静高效' },
  { id: 'amber-light', name: '琥珀金', description: '温暖专业，积极友好' }
]

// Emit events
const emit = defineEmits<{
  settingsChange: [settings: Settings]
}>()

// Load settings from localStorage
const loadSettings = () => {
  try {
    const saved = localStorage.getItem('logviewer-settings')
    if (saved) {
      const parsedSettings = JSON.parse(saved)
      Object.assign(settings, { ...defaultSettings, ...parsedSettings })
    }
  } catch (error) {
    console.error('Failed to load settings:', error)
    ElMessage.warning('设置加载失败，使用默认设置')
  }
}

// Save settings to localStorage
const saveSettings = () => {
  try {
    localStorage.setItem('logviewer-settings', JSON.stringify(settings))
    emit('settingsChange', settings)
    ElMessage.success('设置已保存')
  } catch (error) {
    console.error('Failed to save settings:', error)
    ElMessage.error('设置保存失败')
  }
}

// Auto-save settings with debounce
let saveTimer: number | null = null
const debouncedSave = () => {
  if (saveTimer) {
    clearTimeout(saveTimer)
  }
  saveTimer = setTimeout(() => {
    try {
      localStorage.setItem('logviewer-settings', JSON.stringify(settings))
      emit('settingsChange', settings)
    } catch (error) {
      console.error('Failed to auto-save settings:', error)
    }
  }, 500) as unknown as number
}

// Reset to default settings
const resetSettings = () => {
  Object.assign(settings, JSON.parse(JSON.stringify(defaultSettings)))
  saveSettings()
  ElMessage.success('设置已重置为默认值')
}

// Export settings
const exportSettings = () => {
  try {
    const dataStr = JSON.stringify(settings, null, 2)
    const dataBlob = new Blob([dataStr], { type: 'application/json' })
    const url = URL.createObjectURL(dataBlob)
    
    const link = document.createElement('a')
    link.href = url
    link.download = 'logviewer-settings.json'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    
    URL.revokeObjectURL(url)
    ElMessage.success('设置已导出')
  } catch (error) {
    console.error('Failed to export settings:', error)
    ElMessage.error('设置导出失败')
  }
}

// Import settings
const importSettings = (uploadFile: any) => {
  const file = uploadFile.raw

  if (!file) return

  const reader = new FileReader()
  reader.onload = (e) => {
    try {
      const importedSettings = JSON.parse(e.target?.result as string)

      // Validate the imported settings structure
      if (importedSettings && typeof importedSettings === 'object') {
        // Merge with default settings to ensure all required properties exist
        const mergedSettings = JSON.parse(JSON.stringify(defaultSettings))

        // Deep merge display settings
        if (importedSettings.display) {
          Object.assign(mergedSettings.display, importedSettings.display)
        }

        // Deep merge performance settings
        if (importedSettings.performance) {
          Object.assign(mergedSettings.performance, importedSettings.performance)
        }

        Object.assign(settings, mergedSettings)
        saveSettings()
        ElMessage.success('设置已导入')
      } else {
        throw new Error('Invalid settings format')
      }
    } catch (error) {
      console.error('Failed to import settings:', error)
      ElMessage.error('设置导入失败，请检查文件格式')
    }
  }
  reader.readAsText(file)

  return false // Prevent upload
}

// Show/hide panel - using global state now
// Functions removed, controlled by global useSettings composable

// Watch for settings changes and auto-save with debounce
watch(settings, () => {
  debouncedSave()
}, { deep: true })

// Apply color scheme changes
watch(() => settings.display.colorScheme, (newColorScheme) => {
  applyColorScheme(newColorScheme)
})

const applyColorScheme = (colorScheme: string) => {
  document.documentElement.setAttribute('data-theme', colorScheme)
  // Save to localStorage for themes.css
  localStorage.setItem('logviewer-theme', colorScheme)
}

// Initialize
onMounted(() => {
  loadSettings()
  applyColorScheme(settings.display.colorScheme)

  // Listen for system theme changes when in system mode
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  mediaQuery.addEventListener('change', () => {
    if (settings.display.colorScheme === 'system') {
      applyColorScheme('system')
    }
  })
})

// No need to expose methods anymore - using global state
// defineExpose removed
</script>

<template>
  <el-drawer
    v-model="isSettingsPanelVisible"
    title="设置"
    direction="rtl"
    size="500px"
    :before-close="hideSettings"
  >
    <template #header>
      <div class="settings-header">
        <el-icon><Setting /></el-icon>
        <span>设置</span>
      </div>
    </template>

    <div class="settings-content">
      <el-tabs v-model="activeTab" class="settings-tabs">
        <!-- Display Settings Tab -->
        <el-tab-pane label="显示" name="display">
          <template #label>
            <span class="tab-label">
              <el-icon><Monitor /></el-icon>
              显示
            </span>
          </template>

          <div class="settings-section">
            <h3 class="section-title">主题设置</h3>

            <el-form-item label="主题色彩">
              <div class="color-scheme-grid">
                <div
                  v-for="scheme in colorSchemeOptions"
                  :key="scheme.id"
                  :class="['color-scheme-item', { active: settings.display.colorScheme === scheme.id }]"
                  @click="settings.display.colorScheme = scheme.id as typeof settings.display.colorScheme"
                >
                  <div :data-theme="scheme.id" class="color-scheme-preview">
                    <div class="color-scheme-preview-content">
                      <div class="color-scheme-preview-header"></div>
                      <div class="color-scheme-preview-body">
                        <div class="color-scheme-preview-accent"></div>
                      </div>
                    </div>
                  </div>
                  <div class="color-scheme-info">
                    <div class="color-scheme-name">{{ scheme.name }}</div>
                    <div class="color-scheme-description">{{ scheme.description }}</div>
                  </div>
                </div>
              </div>
            </el-form-item>
          </div>

          <div class="settings-section">
            <h3 class="section-title">字体设置</h3>
            
            <el-form-item label="字体大小">
              <el-slider
                v-model="settings.display.fontSize"
                :min="10"
                :max="24"
                :step="1"
                show-input
                :show-input-controls="false"
              />
            </el-form-item>

            <el-form-item label="行高">
              <el-slider
                v-model="settings.display.lineHeight"
                :min="1.0"
                :max="2.5"
                :step="0.1"
                show-input
                :show-input-controls="false"
              />
            </el-form-item>

            <el-form-item label="字体族">
              <el-select v-model="settings.display.fontFamily" style="width: 100%">
                <el-option
                  v-for="option in fontFamilyOptions"
                  :key="option.value"
                  :label="option.label"
                  :value="option.value"
                />
              </el-select>
            </el-form-item>
          </div>

          <div class="settings-section">
            <h3 class="section-title">显示选项</h3>
            
            <el-form-item>
              <el-checkbox v-model="settings.display.showLineNumbers">
                显示行号
              </el-checkbox>
            </el-form-item>

            <el-form-item>
              <el-checkbox v-model="settings.display.wordWrap">
                自动换行
              </el-checkbox>
            </el-form-item>
          </div>
        </el-tab-pane>

        <!-- Performance Settings Tab -->
        <el-tab-pane label="性能" name="performance">
          <template #label>
            <span class="tab-label">
              <el-icon><DataBoard /></el-icon>
              性能
            </span>
          </template>

          <div class="settings-section">
            <h3 class="section-title">缓存设置</h3>
            
            <el-form-item label="缓存大小 (MB)">
              <el-slider
                v-model="settings.performance.cacheSize"
                :min="10"
                :max="500"
                :step="10"
                show-input
                :show-input-controls="false"
              />
            </el-form-item>

            <el-form-item label="最大日志行数">
              <el-slider
                v-model="settings.performance.maxLogLines"
                :min="1000"
                :max="50000"
                :step="1000"
                show-input
                :show-input-controls="false"
              />
            </el-form-item>

            <el-form-item label="预加载行数">
              <el-slider
                v-model="settings.performance.preloadLines"
                :min="50"
                :max="1000"
                :step="50"
                show-input
                :show-input-controls="false"
              />
            </el-form-item>
          </div>

          <div class="settings-section">
            <h3 class="section-title">性能选项</h3>

            <el-form-item>
              <el-checkbox v-model="settings.performance.enableVirtualScroll">
                启用虚拟滚动
              </el-checkbox>
            </el-form-item>
          </div>
        </el-tab-pane>
      </el-tabs>

      <!-- Action Buttons -->
      <div class="settings-actions">
        <el-button @click="resetSettings" type="warning" plain>
          <el-icon><Refresh /></el-icon>
          重置默认
        </el-button>
        
        <el-button @click="exportSettings" type="primary" plain>
          <el-icon><Document /></el-icon>
          导出设置
        </el-button>
        
        <el-upload
          :show-file-list="false"
          :before-upload="importSettings"
          accept=".json"
          :limit="1"
        >
          <el-button type="success" plain>
            <el-icon><Document /></el-icon>
            导入设置
          </el-button>
        </el-upload>
      </div>
    </div>
  </el-drawer>
</template>

<style scoped>
.settings-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 500;
}

.settings-content {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.settings-tabs {
  flex: 1;
}

.tab-label {
  display: flex;
  align-items: center;
  gap: 6px;
}

.settings-section {
  margin-bottom: 24px;
}

.section-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--app-text-color, #303133);
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--app-border-color, #e4e7ed);
}

.el-form-item {
  margin-bottom: 16px;
}

.settings-actions {
  display: flex;
  gap: 12px;
  padding: 16px 0;
  border-top: 1px solid var(--app-border-color, #e4e7ed);
  margin-top: auto;
}

.settings-actions .el-button {
  flex: 1;
}

/* Color Scheme Styles */
.color-scheme-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 12px;
  margin-top: 8px;
}

.color-scheme-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 2px solid #e4e7ed;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.color-scheme-item:hover {
  border-color: #c0c4cc;
  background: #f5f7fa;
  transform: translateY(-2px);
}

.color-scheme-item.active {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.color-scheme-preview {
  width: 40px;
  height: 28px;
  border-radius: 4px;
  border: 1px solid #e4e7ed;
  overflow: hidden;
  flex-shrink: 0;
}

.color-scheme-preview-content {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.color-scheme-preview-header {
  height: 40%;
  background: var(--primary-500);
}

.color-scheme-preview-body {
  height: 60%;
  background: var(--app-bg-secondary, #f9f9f9);
  position: relative;
}

.color-scheme-preview-accent {
  position: absolute;
  top: 2px;
  left: 2px;
  right: 2px;
  height: 2px;
  background: var(--primary-400);
  border-radius: 1px;
}

.color-scheme-info {
  flex: 1;
  min-width: 0;
}

.color-scheme-name {
  font-size: 14px;
  font-weight: 500;
  color: #303133;
  margin-bottom: 4px;
}

.color-scheme-description {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
}

/* Responsive design */
@media (max-width: 768px) {
  .settings-actions {
    flex-direction: column;
  }
  
  .settings-actions .el-button {
    width: 100%;
  }
}
</style>