export interface LogFile {
  path: string
  name: string
  size: number
  modTime: string
  isDirectory: boolean
  children?: LogFile[]
  isLeaf?: boolean
}

export interface LogContent {
  entries: LogEntry[]
  totalLines: number
  hasMore: boolean
  offset: number
}

export interface LogEntry {
  timestamp: string
  level: string
  message: string
  fields: Record<string, any>
  raw: string
  lineNum: number
  logType: string // JSON, WebServer, Generic
}

export interface SearchQuery {
  path: string
  query: string
  isRegex: boolean
  startTime?: string
  endTime?: string
  levels?: string[]
  offset: number
  limit: number
}

export interface SearchResult {
  entries: LogEntry[]
  totalMatches: number
  hasMore: boolean
  offset: number
}

class ApiService {
  private baseUrl = '/api'

  async getLogFiles(): Promise<LogFile[]> {
    const response = await fetch(`${this.baseUrl}/logs`)
    if (!response.ok) {
      throw new Error(`Failed to fetch log files: ${response.statusText}`)
    }
    const result = await response.json()
    return result.data || []
  }

  async getDirectoryFiles(path: string = ''): Promise<LogFile[]> {
    const params = new URLSearchParams()
    if (path) {
      params.append('path', path)
    }

    const response = await fetch(`${this.baseUrl}/logs/directory?${params}`)
    if (!response.ok) {
      throw new Error(`Failed to fetch directory files: ${response.statusText}`)
    }
    const result = await response.json()
    return result.data || []
  }

  async getLogContent(path: string, offset = 0, limit = 100): Promise<LogContent> {
    const params = new URLSearchParams({
      offset: offset.toString(),
      limit: limit.toString()
    })

    const response = await fetch(`${this.baseUrl}/logs/content/${encodeURIComponent(path)}?${params}`)
    if (!response.ok) {
      throw new Error(`Failed to fetch log content: ${response.statusText}`)
    }
    const result = await response.json()
    return result.data || { entries: [], totalLines: 0, hasMore: false, offset: 0 }
  }

  async getLogContentFromTail(path: string, lines = 100): Promise<LogContent> {
    const params = new URLSearchParams({
      lines: lines.toString()
    })

    const response = await fetch(`${this.baseUrl}/logs/tail/${encodeURIComponent(path)}?${params}`)
    if (!response.ok) {
      throw new Error(`Failed to fetch log content from tail: ${response.statusText}`)
    }
    const result = await response.json()
    return result.data || { entries: [], totalLines: 0, hasMore: false, offset: 0 }
  }

  async searchLogs(query: SearchQuery): Promise<SearchResult> {
    const params = new URLSearchParams({
      path: query.path,
      query: query.query,
      isRegex: query.isRegex.toString(),
      offset: query.offset.toString(),
      limit: query.limit.toString()
    })

    if (query.startTime) {
      params.append('startTime', query.startTime)
    }
    if (query.endTime) {
      params.append('endTime', query.endTime)
    }
    if (query.levels && query.levels.length > 0) {
      params.append('levels', query.levels.join(','))
    }

    const url = `${this.baseUrl}/search?${params}`
    console.log('Sending search request:', url)

    const response = await fetch(url)
    if (!response.ok) {
      const errorText = await response.text()
      console.error('Search API error:', response.status, errorText)
      throw new Error(`Failed to search logs: ${response.statusText}`)
    }
    const result = await response.json()
    console.log('Search API response:', result)

    // Map backend response to frontend interface
    const searchResult = result.data || { entries: [], totalCount: 0, hasMore: false, offset: 0 }
    return {
      entries: searchResult.entries || [],
      totalMatches: searchResult.totalCount || searchResult.totalMatches || 0,
      hasMore: searchResult.hasMore || false,
      offset: searchResult.offset || 0
    }
  }
}

export default new ApiService()