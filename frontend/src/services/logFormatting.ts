import type { LogEntry } from './api'

// 日志格式化策略接口
export interface LogFormattingStrategy {
  // 检查是否可以处理此类型的日志
  canHandle(entry: LogEntry): boolean

  // 检查是否应该显示格式化按钮
  shouldShowButton(entry: LogEntry): boolean

  // 格式化日志内容
  format(entry: LogEntry): string

  // 获取格式化后的显示类型（用于CSS样式）
  getDisplayType(): string

  // 获取策略名称
  getName(): string
}

// JSON格式化策略
export class JSONFormattingStrategy implements LogFormattingStrategy {
  canHandle(entry: LogEntry): boolean {
    return this.isValidJson(entry.message) || this.isValidJson(entry.raw)
  }

  shouldShowButton(entry: LogEntry): boolean {
    return this.canHandle(entry)
  }

  format(entry: LogEntry): string {
    const jsonContent = this.getJsonContent(entry)
    if (!jsonContent) return entry.raw

    try {
      const parsed = JSON.parse(jsonContent)
      const formatted = JSON.stringify(parsed, null, 2)
      return this.highlightJson(formatted)
    } catch {
      return entry.raw
    }
  }

  getDisplayType(): string {
    return 'json-formatted'
  }

  getName(): string {
    return 'JSON'
  }

  private isValidJson(str: string): boolean {
    if (!str || typeof str !== 'string') return false

    str = str.trim()
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

  private getJsonContent(entry: LogEntry): string {
    if (this.isValidJson(entry.message)) return entry.message
    if (this.isValidJson(entry.raw)) return entry.raw
    return ''
  }

  private highlightJson(jsonStr: string): string {
    return jsonStr
      .replace(/"([^"]+)":/g, '<span class="json-key">"$1":</span>')
      .replace(/:\s*"([^"]*)"/g, ': <span class="json-string">"$1"</span>')
      .replace(/:\s*(-?\d+\.?\d*)/g, ': <span class="json-number">$1</span>')
      .replace(/:\s*(true|false)/g, ': <span class="json-boolean">$1</span>')
      .replace(/:\s*(null)/g, ': <span class="json-null">$1</span>')
  }
}

// Web服务器日志格式化策略
export class WebServerFormattingStrategy implements LogFormattingStrategy {
  canHandle(entry: LogEntry): boolean {
    // 检查是否有Web服务器相关的字段
    return entry.fields && Object.keys(entry.fields).some(key =>
      ['remote_addr', 'status', 'request', 'ip_address', 'status_code'].includes(key)
    )
  }

  shouldShowButton(entry: LogEntry): boolean {
    return this.canHandle(entry)
  }

  format(entry: LogEntry): string {
    if (!entry.fields) return entry.raw

    const formatted: string[] = []

    // 按重要性排序显示字段
    const fieldOrder = [
      'remote_addr', 'ip_address', 'request', 'status', 'status_code',
      'body_bytes_sent', 'size', 'http_user_agent', 'http_referer'
    ]

    fieldOrder.forEach(key => {
      if (entry.fields[key] !== undefined) {
        const value = entry.fields[key]
        formatted.push(`<div class="field-row"><span class="field-label">${this.getFieldLabel(key)}:</span><span class="field-value">${this.escapeHtml(String(value))}</span></div>`)
      }
    })

    // 添加其他字段
    Object.entries(entry.fields).forEach(([key, value]) => {
      if (!fieldOrder.includes(key)) {
        formatted.push(`<div class="field-row"><span class="field-label">${key}:</span><span class="field-value">${this.escapeHtml(String(value))}</span></div>`)
      }
    })

    return formatted.join('')
  }

  getDisplayType(): string {
    return 'webserver-formatted'
  }

  getName(): string {
    return 'WebServer'
  }

  private getFieldLabel(key: string): string {
    const labels: Record<string, string> = {
      'remote_addr': '客户端IP',
      'ip_address': 'IP地址',
      'request': '请求',
      'status': '状态码',
      'status_code': '状态码',
      'body_bytes_sent': '响应大小',
      'size': '大小',
      'http_user_agent': '用户代理',
      'http_referer': '来源页面'
    }
    return labels[key] || key
  }

  private escapeHtml(text: string): string {
    const div = document.createElement('div')
    div.textContent = text
    return div.innerHTML
  }
}

// 原文显示策略（默认策略）
export class RawTextFormattingStrategy implements LogFormattingStrategy {
  canHandle(entry: LogEntry): boolean {
    return true // 可以处理任何日志
  }

  shouldShowButton(entry: LogEntry): boolean {
    return false // 原文策略不需要格式化按钮
  }

  format(entry: LogEntry): string {
    return entry.raw
  }

  getDisplayType(): string {
    return 'raw-text'
  }

  getName(): string {
    return 'RawText'
  }
}

// 格式化策略管理器
export class LogFormattingManager {
  private strategies: LogFormattingStrategy[] = []

  constructor() {
    // 按优先级注册策略（优先级高的放前面）
    this.strategies = [
      new JSONFormattingStrategy(),
      new WebServerFormattingStrategy(),
      new RawTextFormattingStrategy() // 默认策略放最后
    ]
  }

  // 获取适合的格式化策略
  getStrategy(entry: LogEntry): LogFormattingStrategy {
    return this.strategies.find(strategy => strategy.canHandle(entry)) ||
           new RawTextFormattingStrategy()
  }

  // 检查是否应该显示格式化按钮
  shouldShowFormattingButton(entry: LogEntry): boolean {
    const strategy = this.getStrategy(entry)
    return strategy.shouldShowButton(entry)
  }

  // 格式化日志内容
  formatLog(entry: LogEntry): { content: string; displayType: string; strategyName: string } {
    const strategy = this.getStrategy(entry)
    return {
      content: strategy.format(entry),
      displayType: strategy.getDisplayType(),
      strategyName: strategy.getName()
    }
  }
}