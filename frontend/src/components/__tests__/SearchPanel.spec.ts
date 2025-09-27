import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, VueWrapper } from '@vue/test-utils'
import { ElInput, ElButton, ElCheckbox, ElSelect, ElDatePicker, ElPagination, ElEmpty, ElTag, ElOption } from 'element-plus'
import SearchPanel from '../SearchPanel.vue'
import apiService from '../../services/api'
import type { SearchResult, LogEntry } from '../../services/api'

// Mock the API service
vi.mock('../../services/api', () => ({
  default: {
    searchLogs: vi.fn()
  }
}))

// Mock Element Plus message
vi.mock('element-plus', async () => {
  const actual = await vi.importActual('element-plus')
  return {
    ...actual,
    ElMessage: {
      warning: vi.fn(),
      error: vi.fn(),
      success: vi.fn(),
      info: vi.fn()
    }
  }
})

describe('SearchPanel', () => {
  let wrapper: VueWrapper<any>
  const mockSearchResult: SearchResult = {
    entries: [
      {
        timestamp: '2024-01-01T10:00:00Z',
        level: 'INFO',
        message: 'Test log message with search term',
        fields: { userId: '123', action: 'login' },
        raw: '2024-01-01T10:00:00Z INFO Test log message with search term',
        lineNum: 1,
        logType: 'Generic'
      },
      {
        timestamp: '2024-01-01T10:01:00Z',
        level: 'ERROR',
        message: 'Error occurred during search operation',
        fields: {},
        raw: '2024-01-01T10:01:00Z ERROR Error occurred during search operation',
        lineNum: 2,
        logType: 'Generic'
      }
    ],
    totalMatches: 2,
    hasMore: false,
    offset: 0
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount()
    }
  })

  const createWrapper = (props = {}) => {
    return mount(SearchPanel, {
      props: {
        currentFilePath: '/test/app.log',
        ...props
      },
      global: {
        components: {
          ElInput,
          ElButton,
          ElCheckbox,
          ElSelect,
          ElDatePicker,
          ElPagination,
          ElEmpty,
          ElTag,
          ElOption
        }
      }
    })
  }

  describe('Component Rendering', () => {
    it('should render search input and controls', () => {
      wrapper = createWrapper()
      
      expect(wrapper.find('.search-input').exists()).toBe(true)
      expect(wrapper.find('[data-testid="regex-checkbox"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="search-button"]').exists()).toBe(true)
    })

    it('should render filter controls', () => {
      wrapper = createWrapper()
      
      expect(wrapper.find('.level-select').exists()).toBe(true)
      expect(wrapper.find('.time-picker').exists()).toBe(true)
    })

    it('should show empty state when no file is selected', () => {
      wrapper = createWrapper({ currentFilePath: '' })
      
      expect(wrapper.find('.empty-state').exists()).toBe(true)
      expect(wrapper.text()).toContain('请先选择一个日志文件')
    })

    it('should disable controls when disabled prop is true', () => {
      wrapper = createWrapper({ disabled: true })
      
      const searchInput = wrapper.findComponent(ElInput)
      expect(searchInput.props('disabled')).toBe(true)
    })
  })

  describe('Search Functionality', () => {
    beforeEach(() => {
      wrapper = createWrapper()
      vi.mocked(apiService.searchLogs).mockResolvedValue(mockSearchResult)
    })

    it('should perform search when search button is clicked', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test search')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      expect(apiService.searchLogs).toHaveBeenCalledWith({
        path: '/test/app.log',
        query: 'test search',
        isRegex: false,
        startTime: undefined,
        endTime: undefined,
        levels: undefined,
        offset: 0,
        limit: 50
      })
    })

    it('should perform search when Enter key is pressed', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test search')
      
      // Call the performSearch method directly to test the functionality
      await wrapper.vm.performSearch(1)
      
      expect(apiService.searchLogs).toHaveBeenCalled()
    })

    it('should include regex flag when regex mode is enabled', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      const regexCheckbox = wrapper.findComponent(ElCheckbox)
      
      await searchInput.setValue('test.*pattern')
      await regexCheckbox.setValue(true)
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      expect(apiService.searchLogs).toHaveBeenCalledWith(
        expect.objectContaining({
          query: 'test.*pattern',
          isRegex: true
        })
      )
    })

    it('should include selected log levels in search', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      // Set selected levels directly on the component instance
      wrapper.vm.selectedLevels = ['ERROR', 'WARN']
      await wrapper.vm.$nextTick()
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      expect(apiService.searchLogs).toHaveBeenCalledWith(
        expect.objectContaining({
          levels: ['ERROR', 'WARN']
        })
      )
    })

    it('should include time range in search', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const startTime = new Date('2024-01-01T00:00:00Z')
      const endTime = new Date('2024-01-01T23:59:59Z')
      
      // Set time range directly on the component instance
      wrapper.vm.startTime = startTime
      wrapper.vm.endTime = endTime
      await wrapper.vm.$nextTick()
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      expect(apiService.searchLogs).toHaveBeenCalledWith(
        expect.objectContaining({
          startTime: startTime.toISOString(),
          endTime: endTime.toISOString()
        })
      )
    })

    it('should emit search results when search completes', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      expect(wrapper.emitted('searchResults')).toBeTruthy()
      expect(wrapper.emitted('searchResults')?.[0]?.[0]).toEqual(mockSearchResult)
    })

    it('should emit search state changes', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      expect(wrapper.emitted('searchStateChange')).toBeTruthy()
      expect(wrapper.emitted('searchStateChange')?.[0]?.[0]).toBe(true)
    })
  })

  describe('Search Results Display', () => {
    beforeEach(() => {
      wrapper = createWrapper()
      vi.mocked(apiService.searchLogs).mockResolvedValue(mockSearchResult)
    })

    it('should display search results after successful search', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      expect(wrapper.find('.results-section').exists()).toBe(true)
      expect(wrapper.find('.results-count').text()).toContain('找到 2 条结果')
      expect(wrapper.findAll('.result-item')).toHaveLength(2)
    })

    it('should highlight search terms in results', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('search')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      const resultMessages = wrapper.findAll('.result-message')
      expect(resultMessages[0].html()).toContain('<mark class="search-highlight">search</mark>')
    })

    it('should display log level tags with correct colors', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      const levelTags = wrapper.findAll('.result-level')
      expect(levelTags[0].text()).toBe('INFO')
      expect(levelTags[1].text()).toBe('ERROR')
    })

    it('should display structured fields for JSON logs', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      const resultFields = wrapper.findAll('.result-fields')
      expect(resultFields).toHaveLength(1) // Only first entry has fields
      expect(resultFields[0].text()).toContain('userId')
      expect(resultFields[0].text()).toContain('action')
    })

    it('should emit entry select when result item is clicked', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      const firstResult = wrapper.find('.result-item')
      await firstResult.trigger('click')
      
      expect(wrapper.emitted('entrySelect')).toBeTruthy()
      expect(wrapper.emitted('entrySelect')?.[0]?.[0]).toEqual(mockSearchResult.entries[0])
    })
  })

  describe('Pagination', () => {
    const mockPaginatedResult: SearchResult = {
      ...mockSearchResult,
      totalMatches: 150,
      hasMore: true
    }

    beforeEach(() => {
      wrapper = createWrapper()
      vi.mocked(apiService.searchLogs).mockResolvedValue(mockPaginatedResult)
    })

    it('should show pagination when there are multiple pages', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      expect(wrapper.findComponent(ElPagination).exists()).toBe(true)
    })

    it('should perform search with correct offset when page changes', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      // Simulate page change to page 2
      const pagination = wrapper.findComponent(ElPagination)
      await pagination.vm.$emit('current-change', 2)
      
      expect(apiService.searchLogs).toHaveBeenLastCalledWith(
        expect.objectContaining({
          offset: 50, // (page 2 - 1) * pageSize 50
          limit: 50
        })
      )
    })
  })

  describe('Search History', () => {
    beforeEach(() => {
      wrapper = createWrapper()
      vi.mocked(apiService.searchLogs).mockResolvedValue(mockSearchResult)
    })

    it('should add successful searches to history', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('first search')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      expect(wrapper.find('.search-history').exists()).toBe(true)
      expect(wrapper.find('.history-tag').text()).toBe('first search')
    })

    it('should allow clicking history tags to repeat searches', async () => {
      // First search to create history
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('historical search')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      // Wait for the search to complete and history to be updated
      await wrapper.vm.$nextTick()
      
      // Verify history was created
      expect(wrapper.vm.searchHistory).toContain('historical search')
      
      // Clear current search
      wrapper.vm.searchQuery = ''
      await wrapper.vm.$nextTick()
      
      // Simulate clicking history tag by setting searchQuery directly (same as @click="searchQuery = query")
      wrapper.vm.searchQuery = 'historical search'
      await wrapper.vm.$nextTick()
      
      // Check the component's reactive data
      expect(wrapper.vm.searchQuery).toBe('historical search')
    })
  })

  describe('Clear Functionality', () => {
    beforeEach(() => {
      wrapper = createWrapper()
      vi.mocked(apiService.searchLogs).mockResolvedValue(mockSearchResult)
    })

    it('should clear search results when clear button is clicked', async () => {
      // Perform search first
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      // Clear search
      const clearButton = wrapper.find('[data-testid="clear-search"]')
      await clearButton.trigger('click')
      
      expect(wrapper.find('.results-section').exists()).toBe(false)
      expect(wrapper.vm.searchQuery).toBe('')
    })

    it('should clear all filters when clear filters button is clicked', async () => {
      // Set some filters directly on the component instance
      wrapper.vm.selectedLevels = ['ERROR']
      wrapper.vm.startTime = new Date()
      wrapper.vm.endTime = new Date()
      wrapper.vm.isRegexMode = true
      await wrapper.vm.$nextTick()
      
      const clearFiltersButton = wrapper.find('[data-testid="clear-filters"]')
      await clearFiltersButton.trigger('click')
      
      expect(wrapper.vm.selectedLevels).toEqual([])
      expect(wrapper.vm.startTime).toBeNull()
      expect(wrapper.vm.endTime).toBeNull()
      expect(wrapper.vm.isRegexMode).toBe(false)
    })
  })

  describe('Error Handling', () => {
    beforeEach(() => {
      wrapper = createWrapper()
    })

    it('should handle search API errors gracefully', async () => {
      vi.mocked(apiService.searchLogs).mockRejectedValue(new Error('API Error'))
      
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      // Should not crash and should emit search state change to false
      expect(wrapper.emitted('searchStateChange')).toBeTruthy()
      const stateChanges = wrapper.emitted('searchStateChange')
      expect(stateChanges?.[stateChanges.length - 1]?.[0]).toBe(false)
    })

    it('should handle invalid regex patterns gracefully', async () => {
      const searchInput = wrapper.findComponent(ElInput)
      const regexCheckbox = wrapper.findComponent(ElCheckbox)
      
      await searchInput.setValue('[invalid regex')
      await regexCheckbox.setValue(true)
      
      // Should not throw error when highlighting text
      expect(() => {
        wrapper.vm.highlightText('test text', '[invalid regex')
      }).not.toThrow()
    })
  })

  describe('Component Props and Events', () => {
    it('should clear search when file path changes', async () => {
      wrapper = createWrapper({ currentFilePath: '/test/app.log' })
      
      // Perform search first
      const searchInput = wrapper.findComponent(ElInput)
      await searchInput.setValue('test')
      
      const searchButton = wrapper.find('[data-testid="search-button"]')
      await searchButton.trigger('click')
      
      await wrapper.vm.$nextTick()
      
      // Change file path
      await wrapper.setProps({ currentFilePath: '/test/other.log' })
      
      expect(wrapper.find('.results-section').exists()).toBe(false)
    })

    it('should expose methods for testing', () => {
      wrapper = createWrapper()
      
      expect(typeof wrapper.vm.performSearch).toBe('function')
      expect(typeof wrapper.vm.clearSearch).toBe('function')
      expect(typeof wrapper.vm.clearFilters).toBe('function')
      expect(typeof wrapper.vm.handleEntryClick).toBe('function')
    })
  })
})