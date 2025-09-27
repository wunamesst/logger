import { ref } from 'vue'

// Global state for settings panel
const isSettingsPanelVisible = ref(false)

export function useSettings() {
  const showSettings = () => {
    isSettingsPanelVisible.value = true
  }

  const hideSettings = () => {
    isSettingsPanelVisible.value = false
  }

  return {
    isSettingsPanelVisible,
    showSettings,
    hideSettings
  }
}