export interface WSMessage {
  type: string
  path?: string
  data?: any
}

export interface LogUpdate {
  path: string
  entries: LogEntry[]
  type: 'append' | 'truncate' | 'delete'
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

export class WebSocketService {
  private ws: WebSocket | null = null
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private listeners: Map<string, ((data: any) => void)[]> = new Map()
  private reconnectTimer: number | null = null
  private heartbeatTimer: number | null = null
  private heartbeatInterval = 30000 // 30秒心跳

  constructor(private url: string) {
    // 暴露到 window 用于调试
    if (typeof window !== 'undefined') {
      (window as any).__wsService = this
    }
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      try {
        console.log('[WebSocket] Connecting to:', this.url)
        this.ws = new WebSocket(this.url)

        this.ws.onopen = () => {
          console.log('[WebSocket] Connected successfully')
          console.log('[WebSocket] ReadyState:', this.ws?.readyState)
          this.reconnectAttempts = 0
          this.startHeartbeat()
          resolve()
        }

        this.ws.onmessage = (event) => {
          console.log('[WebSocket] Message received:', event.data)
          try {
            const message: WSMessage = JSON.parse(event.data)
            console.log('[WebSocket] Parsed message:', message)
            this.handleMessage(message)
          } catch (error) {
            console.error('[WebSocket] Failed to parse message:', error)
          }
        }

        this.ws.onclose = (event) => {
          console.log('[WebSocket] Connection closed:', event.code, event.reason)
          this.stopHeartbeat()
          this.handleReconnect()
        }

        this.ws.onerror = (error) => {
          console.error('[WebSocket] Error occurred:', error)
          reject(error)
        }
      } catch (error) {
        console.error('[WebSocket] Failed to create connection:', error)
        reject(error)
      }
    })
  }

  private startHeartbeat(): void {
    console.log('[WebSocket] Starting heartbeat')
    this.stopHeartbeat()
    this.heartbeatTimer = window.setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        console.log('[WebSocket] Sending heartbeat ping')
        this.send({ type: 'ping' })
      }
    }, this.heartbeatInterval)
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer !== null) {
      console.log('[WebSocket] Stopping heartbeat')
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  disconnect(): void {
    console.log('[WebSocket] Disconnecting...')
    this.stopHeartbeat()
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
  }

  send(message: WSMessage): void {
    console.log('[WebSocket] Attempting to send message:', message)
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      console.log('[WebSocket] Sending message (readyState: OPEN)')
      this.ws.send(JSON.stringify(message))
    } else {
      console.warn('[WebSocket] Cannot send - not connected')
      console.warn('[WebSocket] Current readyState:', this.ws?.readyState)
      console.warn('[WebSocket] States: CONNECTING=0, OPEN=1, CLOSING=2, CLOSED=3')
    }
  }

  on(type: string, callback: (data: any) => void): void {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, [])
    }
    this.listeners.get(type)!.push(callback)
  }

  off(type: string, callback: (data: any) => void): void {
    const callbacks = this.listeners.get(type)
    if (callbacks) {
      const index = callbacks.indexOf(callback)
      if (index > -1) {
        callbacks.splice(index, 1)
      }
    }
  }

  private handleMessage(message: WSMessage): void {
    console.log('[WebSocket] Handling message type:', message.type)
    const callbacks = this.listeners.get(message.type)
    if (callbacks) {
      console.log('[WebSocket] Dispatching to', callbacks.length, 'listeners')
      callbacks.forEach(callback => {
        try {
          callback(message.data)
        } catch (error) {
          console.error('[WebSocket] Error in callback:', error)
        }
      })
    } else {
      console.warn('[WebSocket] No listeners for message type:', message.type)
    }
  }

  private handleReconnect(): void {
    if (this.reconnectTimer !== null) {
      return // 已经在重连中
    }

    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++
      const delay = this.reconnectDelay * this.reconnectAttempts
      console.log(`[WebSocket] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`)

      this.reconnectTimer = window.setTimeout(() => {
        this.reconnectTimer = null
        this.connect().catch(error => {
          console.error('[WebSocket] Reconnection failed:', error)
        })
      }, delay)
    } else {
      console.error('[WebSocket] Max reconnection attempts reached')
    }
  }

  get isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  // 调试方法
  getDebugInfo() {
    return {
      url: this.url,
      readyState: this.ws?.readyState,
      readyStateText: this.getReadyStateText(),
      reconnectAttempts: this.reconnectAttempts,
      maxReconnectAttempts: this.maxReconnectAttempts,
      hasHeartbeat: this.heartbeatTimer !== null,
      listeners: Array.from(this.listeners.keys()).map(type => ({
        type,
        count: this.listeners.get(type)?.length || 0
      })),
      wsInstance: this.ws
    }
  }

  private getReadyStateText(): string {
    if (!this.ws) return 'NULL'
    switch (this.ws.readyState) {
      case WebSocket.CONNECTING: return 'CONNECTING (0)'
      case WebSocket.OPEN: return 'OPEN (1)'
      case WebSocket.CLOSING: return 'CLOSING (2)'
      case WebSocket.CLOSED: return 'CLOSED (3)'
      default: return `UNKNOWN (${this.ws.readyState})`
    }
  }
}

// Create a singleton instance
const wsService = new WebSocketService(`ws://${window.location.host}/ws`)

// 移除自动连接 - 改由 App.vue 统一管理连接时机
// 这样避免了在组件 mount 之前就建立连接，导致事件监听器丢失的问题
console.log('[WebSocket] WebSocket service created (will connect when App mounts)')

// 暴露到全局用于调试
if (typeof window !== 'undefined') {
  (window as any).wsService = wsService
}

export default wsService