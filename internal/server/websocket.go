package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/local-log-viewer/internal/interfaces"
	"github.com/local-log-viewer/internal/types"
)

const (
	// WebSocket配置常量
	writeWait      = 10 * time.Second    // 写入超时时间
	pongWait       = 60 * time.Second    // pong等待时间
	pingPeriod     = (pongWait * 9) / 10 // ping发送周期
	maxMessageSize = 4096                // 最大消息大小
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源的连接（生产环境中应该更严格）
		return true
	},
}

// WebSocketHub WebSocket中心实现
type WebSocketHub struct {
	// 注册的客户端
	clients map[interfaces.WebSocketClient]bool

	// 广播消息通道
	broadcast chan types.WSMessage

	// 注册客户端通道
	register chan interfaces.WebSocketClient

	// 注销客户端通道
	unregister chan interfaces.WebSocketClient

	// 停止通道
	stopCh chan struct{}

	// 运行状态
	running bool
	mutex   sync.RWMutex

	// 性能监控指标
	totalConnections int64
	messagesSent     int64
	messagesDropped  int64
	metricsMutex     sync.RWMutex
}

// NewWebSocketHub 创建新的WebSocket中心
func NewWebSocketHub() interfaces.WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[interfaces.WebSocketClient]bool),
		broadcast:  make(chan types.WSMessage, 256),
		register:   make(chan interfaces.WebSocketClient, 100),
		unregister: make(chan interfaces.WebSocketClient, 100),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动WebSocket中心
func (h *WebSocketHub) Start() error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.running {
		return fmt.Errorf("WebSocket hub is already running")
	}

	// 重新创建通道（如果之前被关闭）
	h.stopCh = make(chan struct{})
	h.clients = make(map[interfaces.WebSocketClient]bool)
	h.broadcast = make(chan types.WSMessage, 256)
	h.register = make(chan interfaces.WebSocketClient, 100)
	h.unregister = make(chan interfaces.WebSocketClient, 100)

	h.running = true
	go h.Run()
	return nil
}

// Stop 停止WebSocket中心
func (h *WebSocketHub) Stop() error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if !h.running {
		return nil
	}

	h.running = false

	// 关闭所有客户端连接
	for client := range h.clients {
		client.Close()
	}

	// 清空客户端映射
	h.clients = make(map[interfaces.WebSocketClient]bool)

	// 关闭停止通道
	select {
	case <-h.stopCh:
		// 通道已经关闭
	default:
		close(h.stopCh)
	}

	return nil
}

// Run 运行WebSocket中心
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.metricsMutex.Lock()
			h.totalConnections++
			h.metricsMutex.Unlock()
			log.Printf("WebSocket client registered: %s (total: %d)", client.GetID(), len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
				log.Printf("WebSocket client unregistered: %s (remaining: %d)", client.GetID(), len(h.clients))
			}

		case message := <-h.broadcast:
			// 广播消息给所有客户端
			for client := range h.clients {
				if err := client.Send(message); err != nil {
					log.Printf("Error sending message to client %s: %v", client.GetID(), err)
					// 发送失败，移除客户端
					delete(h.clients, client)
					client.Close()
				} else {
					h.metricsMutex.Lock()
					h.messagesSent++
					h.metricsMutex.Unlock()
				}
			}

		case <-h.stopCh:
			return
		}
	}
}

// BroadcastLogUpdate 广播日志更新
func (h *WebSocketHub) BroadcastLogUpdate(update types.LogUpdate) {
	message := types.WSMessage{
		Type: "log_update",
		Data: update,
	}

	select {
	case h.broadcast <- message:
		// 发送成功
	default:
		// 通道满，记录指标并发出警告
		h.metricsMutex.Lock()
		h.messagesDropped++
		h.metricsMutex.Unlock()
		log.Printf("Warning: Broadcast channel is full (size: %d/%d), message dropped. Consider increasing buffer size or clients are too slow.",
			len(h.broadcast), cap(h.broadcast))
	}
}

// GetMetrics 获取性能指标
func (h *WebSocketHub) GetMetrics() map[string]interface{} {
	h.metricsMutex.RLock()
	defer h.metricsMutex.RUnlock()

	return map[string]interface{}{
		"total_connections":     h.totalConnections,
		"active_connections":    len(h.clients),
		"messages_sent":         h.messagesSent,
		"messages_dropped":      h.messagesDropped,
		"broadcast_capacity":    cap(h.broadcast),
		"broadcast_queue_size":  len(h.broadcast),
		"broadcast_utilization": float64(len(h.broadcast)) / float64(cap(h.broadcast)) * 100,
	}
}

// RegisterClient 注册客户端
func (h *WebSocketHub) RegisterClient(client interfaces.WebSocketClient) {
	select {
	case h.register <- client:
	default:
		log.Printf("Register channel is full, dropping client registration")
	}
}

// UnregisterClient 注销客户端
func (h *WebSocketHub) UnregisterClient(client interfaces.WebSocketClient) {
	select {
	case h.unregister <- client:
	default:
		log.Printf("Unregister channel is full, dropping client unregistration")
	}
}

// WebSocketMessage WebSocket消息
type WebSocketMessage struct {
	Type string      `json:"type"`
	Path string      `json:"path,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// WebSocketClient WebSocket客户端实现
type WebSocketClient struct {
	// WebSocket连接
	conn *websocket.Conn

	// 发送消息通道
	send chan types.WSMessage

	// WebSocket中心引用
	hub interfaces.WebSocketHub

	// 客户端ID
	id string

	// 日志管理器
	logManager interfaces.LogManager

	// 订阅管理
	subscriptions map[string]context.CancelFunc
	subMutex      sync.Mutex

	// 关闭状态
	closed bool
	mutex  sync.RWMutex
}

// NewWebSocketClient 创建新的WebSocket客户端
func NewWebSocketClient(conn *websocket.Conn, hub interfaces.WebSocketHub, id string, logManager interfaces.LogManager) interfaces.WebSocketClient {
	return &WebSocketClient{
		conn:          conn,
		send:          make(chan types.WSMessage, 256),
		hub:           hub,
		id:            id,
		logManager:    logManager,
		subscriptions: make(map[string]context.CancelFunc),
	}
}

// GetID 获取客户端ID
func (c *WebSocketClient) GetID() string {
	return c.id
}

// Send 发送消息
func (c *WebSocketClient) Send(message types.WSMessage) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.closed {
		return fmt.Errorf("client connection is closed")
	}

	select {
	case c.send <- message:
		return nil
	default:
		return fmt.Errorf("send channel is full")
	}
}

// Close 关闭连接
func (c *WebSocketClient) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	// 取消所有订阅
	c.subMutex.Lock()
	for path, cancel := range c.subscriptions {
		cancel()
		log.Printf("Cancelled subscription for: %s", path)
	}
	c.subscriptions = make(map[string]context.CancelFunc)
	c.subMutex.Unlock()

	close(c.send)
	return c.conn.Close()
}

// readPump 读取消息泵
func (c *WebSocketClient) readPump() {
	defer func() {
		c.hub.UnregisterClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		log.Printf("WebSocket message received: type=%d, length=%d", messageType, len(message))

		// 处理客户端发送的消息
		c.handleMessage(message)
	}
}

// handleMessage 处理WebSocket消息
func (c *WebSocketClient) handleMessage(message []byte) {
	log.Printf("Raw WebSocket message received: %s", string(message))

	var msg WebSocketMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Failed to parse WebSocket message: %v", err)
		c.sendError("PARSE_ERROR", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	log.Printf("Parsed WebSocket message: type=%s, path=%s", msg.Type, msg.Path)

	switch msg.Type {
	case "subscribe":
		c.handleSubscribe(msg.Path)

	case "unsubscribe":
		c.handleUnsubscribe(msg.Path)

	case "ping":
		// 响应 ping 消息，发送 pong 回复
		log.Printf("Received ping from client %s", c.GetID())
		c.Send(types.WSMessage{
			Type: "pong",
			Data: map[string]interface{}{
				"timestamp": time.Now().Unix(),
			},
		})

	default:
		log.Printf("Unknown WebSocket message type: %s", msg.Type)
		c.sendError("UNKNOWN_TYPE", fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

// normalizePath 规范化路径
func (c *WebSocketClient) normalizePath(path string) (string, error) {
	// 如果是绝对路径，直接返回
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return path, nil
	}

	// 如果是相对路径，尝试从配置的日志目录中查找
	logPaths := c.logManager.GetLogPaths()
	for _, logDir := range logPaths {
		fullPath := filepath.Join(logDir, path)
		if _, err := os.Stat(fullPath); err == nil {
			log.Printf("Normalized path: %s -> %s", path, fullPath)
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("file not found in any log directory: %s", path)
}

// handleSubscribe 处理订阅请求
func (c *WebSocketClient) handleSubscribe(path string) {
	if path == "" {
		c.sendError("INVALID_PATH", "Path is required for subscribe")
		return
	}

	log.Printf("Starting file watch for: %s", path)

	// 规范化路径
	normalizedPath, err := c.normalizePath(path)
	if err != nil {
		log.Printf("Failed to normalize path %s: %v", path, err)
		c.sendError("FILE_NOT_FOUND", err.Error())
		return
	}

	// 开始监控指定文件
	updateCh, err := c.logManager.WatchFile(normalizedPath)
	if err != nil {
		log.Printf("Failed to watch file %s: %v", normalizedPath, err)
		c.sendError("WATCH_FAILED", fmt.Sprintf("Failed to watch file: %v", err))
		return
	}

	log.Printf("File watch started successfully for: %s", normalizedPath)

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())

	c.subMutex.Lock()
	// 取消旧订阅（如果存在）
	if oldCancel, exists := c.subscriptions[normalizedPath]; exists {
		log.Printf("Cancelling existing subscription for: %s", normalizedPath)
		oldCancel()
	}
	c.subscriptions[normalizedPath] = cancel
	c.subMutex.Unlock()

	// 发送订阅成功确认
	c.Send(types.WSMessage{
		Type: "subscribed",
		Data: map[string]string{"path": normalizedPath},
	})

	// 在新的 goroutine 中监听文件更新
	go func() {
		log.Printf("Starting update listener goroutine for: %s", normalizedPath)
		defer log.Printf("Update listener goroutine ended for: %s", normalizedPath)

		for {
			select {
			case update, ok := <-updateCh:
				if !ok {
					log.Printf("Update channel closed for: %s", normalizedPath)
					return
				}

				log.Printf("Received file update for %s: %d entries", update.Path, len(update.Entries))

				// 将更新发送给这个客户端
				wsMessage := types.WSMessage{
					Type: "log_update",
					Data: update,
				}

				if err := c.Send(wsMessage); err != nil {
					log.Printf("Failed to send log update: %v", err)
					return
				}

				log.Printf("Successfully sent log update to client")

			case <-ctx.Done():
				log.Printf("Subscription cancelled for: %s", normalizedPath)
				return
			}
		}
	}()
}

// handleUnsubscribe 处理取消订阅请求
func (c *WebSocketClient) handleUnsubscribe(path string) {
	if path == "" {
		c.sendError("INVALID_PATH", "Path is required for unsubscribe")
		return
	}

	// 规范化路径
	normalizedPath, err := c.normalizePath(path)
	if err != nil {
		// 如果路径不存在，尝试直接使用原始路径
		normalizedPath = path
	}

	c.subMutex.Lock()
	if cancel, exists := c.subscriptions[normalizedPath]; exists {
		cancel()
		delete(c.subscriptions, normalizedPath)
		log.Printf("Unsubscribed from file: %s", normalizedPath)
		c.subMutex.Unlock()

		// 发送取消订阅确认
		c.Send(types.WSMessage{
			Type: "unsubscribed",
			Data: map[string]string{"path": normalizedPath},
		})
	} else {
		c.subMutex.Unlock()
		log.Printf("No active subscription for: %s", normalizedPath)
		c.sendError("NOT_SUBSCRIBED", fmt.Sprintf("No active subscription for: %s", normalizedPath))
	}
}

// sendError 发送错误消息
func (c *WebSocketClient) sendError(code, message string) {
	c.Send(types.WSMessage{
		Type: "error",
		Data: map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// writePump 写入消息泵
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// 发送通道已关闭
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 发送JSON消息
			if err := c.conn.WriteJSON(message); err != nil {
				log.Printf("Error writing JSON message: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// startPumps 启动读写泵
func (c *WebSocketClient) startPumps() {
	go c.writePump()
	go c.readPump()
}

// HandleWebSocketConnection 处理WebSocket连接
func HandleWebSocketConnection(c *gin.Context, hub interfaces.WebSocketHub, logManager interfaces.LogManager) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	// 生成客户端ID
	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())

	// 创建客户端
	client := NewWebSocketClient(conn, hub, clientID, logManager)

	// 注册客户端
	hub.RegisterClient(client)

	// 启动消息泵
	if wsClient, ok := client.(*WebSocketClient); ok {
		wsClient.startPumps()
	}
}
