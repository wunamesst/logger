import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import FileBrowser from '../FileBrowser.vue'
import apiService from '../../services/api'
import wsService from '../../services/websocket'

// Mock the API service
vi.mock('../../services/api', () => ({
  default: {
    getLogFiles: vi.fn()
  }
}))

// Mock the WebSocket service
vi.mock('../../services/websocket', () => ({
  default: {
    on: vi.fn(),
    off: vi.fn()
  }
}))

// Mock Element Plus Message
vi.mock('element-plus', async () => {
  const actual = await vi.importActual('element-plus')
  return {
    ...actual,
    ElMessage: {
      error: vi.fn()
    }
  }
})

describe('FileBrowser', () => {
  const mockLogFiles = [
    {
      path: '/logs/app.log',
      name: 'app.log',
      size: 1024,
      modTime: new Date().toISOString(),
      isDirectory: false
    },
    {
      path: '/logs/error.log',
      name: 'error.log',
      size: 2048,
      modTime: new Date(Date.now() - 10 * 60 * 1000).toISOString(), // 10 minutes ago
      isDirectory: false
    },
    {
      path: '/logs/archive',
      name: 'archive',
      size: 0,
      modTime: new Date().toISOString(),
      isDirectory: true,
      children: [
        {
          path: '/logs/archive/old.log',
          name: 'old.log',
          size: 512,
          modTime: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(), // 1 day ago
          isDirectory: false
        }
      ]
    }
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(apiService.getLogFiles).mockResolvedValue(mockLogFiles)
  })

  it('renders correctly', () => {
    const wrapper = mount(FileBrowser, {
      global: {
        stubs: {
          'el-tree': true,
          'el-button': true,
          'el-tag': true,
          'el-tooltip': true,
          'el-icon': true
        }
      }
    })

    expect(wrapper.find('.file-browser').exists()).toBe(true)
    expect(wrapper.find('.browser-header').exists()).toBe(true)
    expect(wrapper.find('.browser-content').exists()).toBe(true)
  })

  it('loads log files on mount', async () => {
    mount(FileBrowser, {
      global: {
        stubs: {
          'el-tree': true,
          'el-button': true,
          'el-tag': true,
          'el-tooltip': true,
          'el-icon': true
        }
      }
    })

    // Wait for the component to mount and load files
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(apiService.getLogFiles).toHaveBeenCalled()
  })

  it('emits file-select event when a file is clicked', async () => {
    const wrapper = mount(FileBrowser, {
      global: {
        stubs: {
          'el-tree': true,
          'el-button': true,
          'el-tag': true,
          'el-tooltip': true,
          'el-icon': true
        }
      }
    })

    // Wait for files to load
    await new Promise(resolve => setTimeout(resolve, 0))

    // Simulate clicking on a file (not directory)
    const fileData = mockLogFiles[0] // app.log
    await wrapper.vm.handleNodeClick(fileData)

    expect(wrapper.emitted('fileSelect')).toBeTruthy()
    expect(wrapper.emitted('fileSelect')?.[0]).toEqual([fileData])
  })

  it('does not emit file-select event when a directory is clicked', async () => {
    const wrapper = mount(FileBrowser, {
      global: {
        stubs: {
          'el-tree': true,
          'el-button': true,
          'el-tag': true,
          'el-tooltip': true,
          'el-icon': true
        }
      }
    })

    // Wait for files to load
    await new Promise(resolve => setTimeout(resolve, 0))

    // Simulate clicking on a directory
    const directoryData = mockLogFiles[2] // archive directory
    await wrapper.vm.handleNodeClick(directoryData)

    expect(wrapper.emitted('fileSelect')).toBeFalsy()
  })

  it('formats file size correctly', () => {
    const wrapper = mount(FileBrowser, {
      global: {
        stubs: {
          'el-tree': true,
          'el-button': true,
          'el-tag': true,
          'el-tooltip': true,
          'el-icon': true
        }
      }
    })

    expect(wrapper.vm.formatFileSize(0)).toBe('0 B')
    expect(wrapper.vm.formatFileSize(1024)).toBe('1 KB')
    expect(wrapper.vm.formatFileSize(1048576)).toBe('1 MB')
    expect(wrapper.vm.formatFileSize(1073741824)).toBe('1 GB')
  })

  it('handles refresh button click', async () => {
    const wrapper = mount(FileBrowser, {
      global: {
        stubs: {
          'el-tree': true,
          'el-button': true,
          'el-tag': true,
          'el-tooltip': true,
          'el-icon': true
        }
      }
    })

    // Clear the initial call
    vi.clearAllMocks()

    // Simulate refresh button click
    await wrapper.vm.handleRefresh()

    expect(apiService.getLogFiles).toHaveBeenCalled()
  })

  it('registers WebSocket event listener on mount', () => {
    mount(FileBrowser, {
      global: {
        stubs: {
          'el-tree': true,
          'el-button': true,
          'el-tag': true,
          'el-tooltip': true,
          'el-icon': true
        }
      }
    })

    expect(wsService.on).toHaveBeenCalledWith('message', expect.any(Function))
  })

  it('formats modification time correctly', () => {
    const wrapper = mount(FileBrowser, {
      global: {
        stubs: {
          'el-tree': true,
          'el-button': true,
          'el-tag': true,
          'el-tooltip': true,
          'el-icon': true
        }
      }
    })

    const testDate = new Date('2024-01-15T10:30:00Z')
    const formatted = wrapper.vm.formatModTime(testDate.toISOString())
    
    // The exact format depends on locale, but it should contain date and time
    expect(formatted).toMatch(/2024/)
    expect(formatted).toMatch(/01/)
    expect(formatted).toMatch(/15/)
  })
})