# API 参考文档

本文档描述了本地日志查看工具提供的 REST API 和 WebSocket API。

## 目录

- [REST API](#rest-api)
- [WebSocket API](#websocket-api)
- [数据模型](#数据模型)
- [错误处理](#错误处理)
- [示例代码](#示例代码)

## REST API

### 基础信息

- **Base URL**: `http://localhost:8080/api`
- **Content-Type**: `application/json`
- **字符编码**: UTF-8

### 端点列表

#### 1. 健康检查

检查服务状态。

```http
GET /api/health
```

**响应**:
```json
{
  "status": "ok",
  "timestamp": "2024-01-01T10:00:00Z",
  "uptime": "1h30m45s"
}
```

#### 2. 版本信息

获取应用版本信息。

```http
GET /api/version
```

**响应**:
```json
{
  "version": "1.0.0",
  "commit": "abc123def",
  "buildTime": "2024-01-01T10:00:00Z",
  "goVersion": "go1.21.0"
}
```

#### 3. 获取日志文件列表

获取可用的日志文件树形结构。

```http
GET /api/logs
```

**响应**:
```json
[
  {
    "path": "app.log",
    "name": "app.log",
    "size": 1024,
    "modTime": "2024-01-01T10:00:00Z",
    "isDirectory": false
  },
  {
    "path": "logs",
    "name": "logs",
    "size": 0,
    "modTime": "2024-01-01T09:00:00Z",
    "isDirectory": true,
    "children": [
      {
        "path": "logs/error.log",
        "name": "error.log",
        "size": 512,
        "modTime": "2024-01-01T10:00:00Z",
        "isDirectory": false
      }
    ]
  }
]
```

#### 4. 获取日志文件内容

读取指定日志文件的内容。

```http
GET /api/logs/{path}
```

**路径参数**:
- `path`: 日志文件路径（URL 编码）

**查询参数**:
- `offset` (int): 起始行号，默认 0
- `limit` (int): 返回行数，默认 100，最大 1000
- `reverse` (bool): 是否倒序返回，默认 false

**示例**:
```http
GET /api/logs/app.log?offset=0&limit=50
GET /api/logs/logs%2Ferror.log?offset=100&limit=20&reverse=true
```

**响应**:
```json
{
  "entries": [
    {
      "timestamp": "2024-01-01T10:00:00Z",
      "level": "INFO",
      "message": "Application started",
      "fields": {
        "service": "logviewer"
      },
      "raw": "2024-01-01 10:00:00 INFO Application started",
      "lineNum": 1
    }
  ],
  "totalLines": 1000,
  "hasMore": true,
  "offset": 0
}
```

#### 5. 搜索日志

在指定文件中搜索日志内容。

```http
GET /api/search
```

**查询参数**:
- `path` (string, 必需): 日志文件路径
- `query` (string): 搜索关键词或正则表达式
- `isRegex` (bool): 是否使用正则表达式，默认 false
- `startTime` (string): 开始时间 (RFC3339 格式)
- `endTime` (string): 结束时间 (RFC3339 格式)
- `levels` (string): 日志级别，多个用逗号分隔 (ERROR,WARN,INFO,DEBUG)
- `offset` (int): 分页偏移，默认 0
- `limit` (int): 返回条数，默认 50，最大 500

**示例**:
```http
GET /api/search?path=app.log&query=error&limit=20
GET /api/search?path=app.log&query=\d+\.\d+\.\d+\.\d+&isRegex=true
GET /api/search?path=app.log&levels=ERROR,WARN&startTime=2024-01-01T10:00:00Z
```

**响应**:
```json
{
  "entries": [
    {
      "timestamp": "2024-01-01T10:00:05Z",
      "level": "ERROR",
      "message": "Database connection failed",
      "fields": {},
      "raw": "2024-01-01 10:00:05 ERROR Database connection failed",
      "lineNum": 5,
      "highlights": [
        {
          "start": 20,
          "end": 25,
          "text": "ERROR"
        }
      ]
    }
  ],
  "total": 15,
  "hasMore": false,
  "query": "error",
  "executionTime": "15ms"
}
```

## WebSocket API

### 连接

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
```

### 消息格式

所有 WebSocket 消息使用 JSON 格式：

```json
{
  "type": "message_type",
  "data": { /* 消息数据 */ }
}
```

### 客户端消息

#### 1. 订阅文件更新

```json
{
  "type": "subscribe",
  "data": {
    "path": "app.log"
  }
}
```

#### 2. 取消订阅

```json
{
  "type": "unsubscribe",
  "data": {
    "path": "app.log"
  }
}
```

#### 3. 心跳

```json
{
  "type": "ping",
  "data": {}
}
```

### 服务器消息

#### 1. 订阅确认

```json
{
  "type": "subscribed",
  "data": {
    "path": "app.log",
    "message": "Successfully subscribed to app.log"
  }
}
```

#### 2. 取消订阅确认

```json
{
  "type": "unsubscribed",
  "data": {
    "path": "app.log",
    "message": "Successfully unsubscribed from app.log"
  }
}
```

#### 3. 日志更新

```json
{
  "type": "log_update",
  "data": {
    "path": "app.log",
    "updateType": "append",
    "entries": [
      {
        "timestamp": "2024-01-01T10:00:00Z",
        "level": "INFO",
        "message": "New log entry",
        "fields": {},
        "raw": "2024-01-01 10:00:00 INFO New log entry",
        "lineNum": 1001
      }
    ]
  }
}
```

#### 4. 心跳响应

```json
{
  "type": "pong",
  "data": {
    "timestamp": "2024-01-01T10:00:00Z"
  }
}
```

#### 5. 错误消息

```json
{
  "type": "error",
  "data": {
    "code": "INVALID_PATH",
    "message": "File not found: nonexistent.log",
    "details": "The specified file path does not exist or is not accessible"
  }
}
```

## 数据模型

### LogFile

```typescript
interface LogFile {
  path: string;        // 文件路径
  name: string;        // 文件名
  size: number;        // 文件大小（字节）
  modTime: string;     // 修改时间 (RFC3339)
  isDirectory: boolean; // 是否为目录
  children?: LogFile[]; // 子文件（仅目录）
}
```

### LogEntry

```typescript
interface LogEntry {
  timestamp: string;              // 时间戳 (RFC3339)
  level: string;                  // 日志级别
  message: string;                // 日志消息
  fields: Record<string, any>;    // 结构化字段
  raw: string;                    // 原始日志行
  lineNum: number;                // 行号
  highlights?: Highlight[];       // 搜索高亮（仅搜索结果）
}
```

### LogContent

```typescript
interface LogContent {
  entries: LogEntry[];    // 日志条目
  totalLines: number;     // 文件总行数
  hasMore: boolean;       // 是否有更多内容
  offset: number;         // 当前偏移
}
```

### SearchResult

```typescript
interface SearchResult {
  entries: LogEntry[];    // 搜索结果
  total: number;          // 总匹配数
  hasMore: boolean;       // 是否有更多结果
  query: string;          // 搜索查询
  executionTime: string;  // 执行时间
}
```

### Highlight

```typescript
interface Highlight {
  start: number;    // 高亮开始位置
  end: number;      // 高亮结束位置
  text: string;     // 高亮文本
}
```

## 错误处理

### HTTP 状态码

- `200 OK`: 请求成功
- `400 Bad Request`: 请求参数错误
- `404 Not Found`: 资源不存在
- `500 Internal Server Error`: 服务器内部错误

### 错误响应格式

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable error message",
    "details": "Additional error details"
  }
}
```

### 常见错误码

- `INVALID_PATH`: 无效的文件路径
- `FILE_NOT_FOUND`: 文件不存在
- `PERMISSION_DENIED`: 权限不足
- `INVALID_REGEX`: 无效的正则表达式
- `INVALID_TIME_FORMAT`: 无效的时间格式
- `FILE_TOO_LARGE`: 文件过大
- `RATE_LIMIT_EXCEEDED`: 请求频率超限

## 示例代码

### JavaScript/TypeScript

#### 获取日志文件列表

```javascript
async function getLogFiles() {
  try {
    const response = await fetch('/api/logs');
    const files = await response.json();
    return files;
  } catch (error) {
    console.error('获取文件列表失败:', error);
  }
}
```

#### 读取日志内容

```javascript
async function readLogFile(path, offset = 0, limit = 100) {
  try {
    const params = new URLSearchParams({
      offset: offset.toString(),
      limit: limit.toString()
    });
    
    const response = await fetch(`/api/logs/${encodeURIComponent(path)}?${params}`);
    const content = await response.json();
    return content;
  } catch (error) {
    console.error('读取日志失败:', error);
  }
}
```

#### 搜索日志

```javascript
async function searchLogs(path, query, options = {}) {
  try {
    const params = new URLSearchParams({
      path,
      query,
      ...options
    });
    
    const response = await fetch(`/api/search?${params}`);
    const result = await response.json();
    return result;
  } catch (error) {
    console.error('搜索失败:', error);
  }
}
```

#### WebSocket 连接

```javascript
class LogViewerWebSocket {
  constructor(url) {
    this.ws = new WebSocket(url);
    this.setupEventHandlers();
  }
  
  setupEventHandlers() {
    this.ws.onopen = () => {
      console.log('WebSocket 连接已建立');
    };
    
    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      this.handleMessage(message);
    };
    
    this.ws.onclose = () => {
      console.log('WebSocket 连接已关闭');
    };
    
    this.ws.onerror = (error) => {
      console.error('WebSocket 错误:', error);
    };
  }
  
  handleMessage(message) {
    switch (message.type) {
      case 'log_update':
        this.onLogUpdate(message.data);
        break;
      case 'error':
        this.onError(message.data);
        break;
    }
  }
  
  subscribe(path) {
    this.send('subscribe', { path });
  }
  
  unsubscribe(path) {
    this.send('unsubscribe', { path });
  }
  
  send(type, data) {
    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, data }));
    }
  }
  
  onLogUpdate(data) {
    // 处理日志更新
    console.log('收到日志更新:', data);
  }
  
  onError(error) {
    // 处理错误
    console.error('WebSocket 错误:', error);
  }
}

// 使用示例
const ws = new LogViewerWebSocket('ws://localhost:8080/ws');
ws.subscribe('app.log');
```

### Python

#### 使用 requests 库

```python
import requests
import json

class LogViewerClient:
    def __init__(self, base_url='http://localhost:8080'):
        self.base_url = base_url
        
    def get_log_files(self):
        """获取日志文件列表"""
        response = requests.get(f'{self.base_url}/api/logs')
        response.raise_for_status()
        return response.json()
    
    def read_log_file(self, path, offset=0, limit=100):
        """读取日志文件内容"""
        params = {'offset': offset, 'limit': limit}
        response = requests.get(
            f'{self.base_url}/api/logs/{path}',
            params=params
        )
        response.raise_for_status()
        return response.json()
    
    def search_logs(self, path, query, **options):
        """搜索日志"""
        params = {'path': path, 'query': query, **options}
        response = requests.get(
            f'{self.base_url}/api/search',
            params=params
        )
        response.raise_for_status()
        return response.json()

# 使用示例
client = LogViewerClient()
files = client.get_log_files()
content = client.read_log_file('app.log', limit=50)
results = client.search_logs('app.log', 'ERROR', levels='ERROR,WARN')
```

### Go

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
)

type LogViewerClient struct {
    BaseURL string
    Client  *http.Client
}

func NewLogViewerClient(baseURL string) *LogViewerClient {
    return &LogViewerClient{
        BaseURL: baseURL,
        Client:  &http.Client{},
    }
}

func (c *LogViewerClient) GetLogFiles() ([]LogFile, error) {
    resp, err := c.Client.Get(c.BaseURL + "/api/logs")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var files []LogFile
    err = json.NewDecoder(resp.Body).Decode(&files)
    return files, err
}

func (c *LogViewerClient) SearchLogs(path, query string, options map[string]string) (*SearchResult, error) {
    params := url.Values{}
    params.Set("path", path)
    params.Set("query", query)
    
    for k, v := range options {
        params.Set(k, v)
    }
    
    resp, err := c.Client.Get(c.BaseURL + "/api/search?" + params.Encode())
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result SearchResult
    err = json.NewDecoder(resp.Body).Decode(&result)
    return &result, err
}

// 使用示例
func main() {
    client := NewLogViewerClient("http://localhost:8080")
    
    files, err := client.GetLogFiles()
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("找到 %d 个文件\n", len(files))
}
```

## 速率限制

为了保护服务器性能，API 实施了速率限制：

- **搜索 API**: 每分钟最多 60 次请求
- **文件读取 API**: 每分钟最多 120 次请求
- **其他 API**: 每分钟最多 300 次请求

超出限制时会返回 `429 Too Many Requests` 状态码。

## 缓存

API 使用智能缓存提高性能：

- **文件列表**: 缓存 30 秒
- **文件内容**: 缓存 5 分钟
- **搜索结果**: 缓存 2 分钟

可以通过 `Cache-Control: no-cache` 头部跳过缓存。