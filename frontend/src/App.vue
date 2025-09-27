<script setup lang="ts">
import { RouterView } from 'vue-router'
import { ref, onMounted, onUnmounted } from 'vue'
import { Document, Setting } from '@element-plus/icons-vue'
import wsService from './services/websocket'
import { useSettings } from './composables/useSettings'
import SettingsPanel from './components/SettingsPanel.vue'

// WebSocket connection state
const wsConnected = ref(false)

// Settings composable
const { showSettings } = useSettings()

// Handle settings button click
const handleShowSettings = () => {
  console.log('handleShowSettings called - using composable')
  showSettings()
}

// Settings change handler - moved from LogViewerView.vue
const handleSettingsChange = (settings: any) => {
  console.log('Settings changed:', settings)

  // Apply font settings to document root
  const root = document.documentElement
  if (settings.display?.fontSize) {
    root.style.setProperty('--log-font-size', `${settings.display.fontSize}px`)
  }
  if (settings.display?.lineHeight) {
    root.style.setProperty('--log-line-height', settings.display.lineHeight.toString())
  }
  if (settings.display?.fontFamily) {
    root.style.setProperty('--log-font-family', settings.display.fontFamily)
  }

  // Apply display settings
  root.style.setProperty('--show-line-numbers', settings.display?.showLineNumbers ? 'block' : 'none')
  root.style.setProperty('--word-wrap', settings.display?.wordWrap ? 'break-word' : 'normal')
}


// Load and apply initial settings
const loadInitialSettings = () => {
  try {
    const saved = localStorage.getItem('logviewer-settings')
    if (saved) {
      const settings = JSON.parse(saved)
      handleSettingsChange(settings)
    }
  } catch (error) {
    console.warn('Failed to load initial settings:', error)
  }
}

onMounted(async () => {
  // Load and apply initial settings first
  loadInitialSettings()

  // Connect to WebSocket only once (避免重复连接)
  if (!wsService.isConnected) {
    try {
      console.log('[App] Initializing WebSocket connection...')
      await wsService.connect()
      wsConnected.value = wsService.isConnected

      // Listen for connection status changes
      wsService.on('connect', () => {
        wsConnected.value = true
        console.log('[App] WebSocket connected')
      })

      wsService.on('disconnect', () => {
        wsConnected.value = false
        console.log('[App] WebSocket disconnected')
      })

      console.log('[App] WebSocket service initialized successfully')
      console.log('[App] Debug info:', wsService.getDebugInfo())
    } catch (error) {
      console.error('[App] Failed to connect to WebSocket:', error)
      wsConnected.value = false
    }
  } else {
    console.log('[App] WebSocket already connected, skipping initialization')
    wsConnected.value = true
  }
})

onUnmounted(() => {
  wsService.disconnect()
})
</script>

<template>
  <el-container class="app-container">
    <el-header class="app-header glass-effect">
      <div class="header-content">
        <div class="brand-section">
          <div class="logo-container">
            <el-icon class="app-icon"><Document /></el-icon>
          </div>
          <div class="title-section">
            <h1 class="app-title">本地日志查看器</h1>
            <p class="app-subtitle">实时日志监控与分析平台</p>
          </div>
        </div>

        <div class="header-actions">
          <div class="connection-status">
            <span class="status-text">
              {{ wsConnected ? '已连接' : '未连接' }}
            </span>
          </div>

          <el-button
            class="settings-btn"
            @click="handleShowSettings"
          >
            <el-icon><Setting /></el-icon>
          </el-button>
        </div>
      </div>
    </el-header>

    <el-main class="app-main">
      <RouterView />
    </el-main>

    <!-- Global Settings Panel -->
    <SettingsPanel @settings-change="handleSettingsChange" />
  </el-container>
</template>

<style scoped>
.app-container {
  height: 100vh;
  background: var(--app-bg-color);
}

.app-header {
  background: var(--app-header-bg);
  color: var(--app-header-text);
  border: none;
  box-shadow: var(--shadow-lg);
  position: relative;
  z-index: var(--z-sticky);
  padding: 0 var(--space-xl);
  height: 80px !important;
  transition: all var(--transition-normal);
}

.app-header::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--app-header-bg);
  opacity: 0.9;
  backdrop-filter: var(--app-backdrop-blur);
  -webkit-backdrop-filter: var(--app-backdrop-blur);
  z-index: -1;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  height: 100%;
  position: relative;
}

.brand-section {
  display: flex;
  align-items: center;
  gap: var(--space-md);
}

.logo-container {
  width: 48px;
  height: 48px;
  background: rgba(255, 255, 255, 0.15);
  border-radius: var(--radius-xl);
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(10px);
  border: 1px solid rgba(255, 255, 255, 0.2);
  transition: all var(--transition-normal);
}

.logo-container:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg);
  background: rgba(255, 255, 255, 0.2);
}

.app-icon {
  font-size: 24px;
  color: var(--app-header-text);
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.1));
}

.title-section {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
}

.app-title {
  margin: 0;
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-bold);
  line-height: var(--line-height-tight);
  letter-spacing: -0.02em;
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  color: white;
}

.app-subtitle {
  margin: 0;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  opacity: 0.8;
  line-height: var(--line-height-normal);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
}

.connection-status {
  display: flex;
  align-items: center;
  padding: var(--space-sm) var(--space-md);
  transition: all var(--transition-normal);
}

.status-text {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--app-header-text);
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
}

.settings-btn {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: var(--app-header-text);
  backdrop-filter: blur(10px);
  border-radius: 50%;
  font-weight: var(--font-weight-medium);
  transition: all var(--transition-normal);
  width: 32px;
}

.settings-btn:hover {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.3);
  transform: translateY(-1px);
  box-shadow: var(--shadow-sm);
}

.app-main {
  padding: 0;
  height: calc(100vh - 80px);
  overflow: hidden;
  background: var(--app-bg-color);
  transition: background-color var(--transition-normal);
}

/* Animations */
@keyframes pulse {
  0%, 100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.7;
    transform: scale(1.1);
  }
}

/* Responsive Design */
@media (max-width: 768px) {
  .app-header {
    padding: 0 var(--space-md);
    height: 70px !important;
  }

  .brand-section {
    gap: var(--space-sm);
  }

  .logo-container {
    width: 40px;
    height: 40px;
  }

  .app-icon {
    font-size: 20px;
  }

  .app-title {
    font-size: var(--font-size-xl);
  }

  .app-subtitle {
    display: none;
  }

  .app-main {
    height: calc(100vh - 70px);
  }
}

@media (max-width: 480px) {
  .header-content {
    gap: var(--space-sm);
  }

  .connection-indicator {
    padding: var(--space-xs) var(--space-sm);
  }

  .status-text {
    display: none;
  }
}
</style>
