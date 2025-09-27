# 本地日志查看器 - 前端

这是本地日志查看器的前端部分，使用 Vue.js 3 + Element Plus 构建。

## 技术栈

- **Vue.js 3** - 渐进式 JavaScript 框架
- **TypeScript** - 类型安全的 JavaScript
- **Element Plus** - Vue 3 UI 组件库
- **Vite** - 快速构建工具
- **Vue Router** - 路由管理
- **Pinia** - 状态管理

## 开发

### 安装依赖

```bash
npm install
```

### 开发模式

```bash
npm run dev
```

这将启动开发服务器在 `http://localhost:5173`，并自动代理 API 请求到后端服务器 `http://localhost:8080`。

### 构建

```bash
npm run build-for-go
```

这将构建前端应用并输出到 `../web` 目录，供 Go 应用嵌入使用。

### 测试

```bash
npm run test:unit
```

### 代码检查

```bash
npm run lint
```

## 项目结构

```
src/
├── components/          # 可复用组件
├── views/              # 页面组件
├── services/           # API 和 WebSocket 服务
├── router/             # 路由配置
├── stores/             # Pinia 状态管理
└── assets/             # 静态资源
```

## 主要功能

- **文件浏览器** - 显示日志文件树形结构
- **日志查看器** - 支持大文件虚拟滚动和实时更新
- **搜索面板** - 关键词搜索、正则表达式、时间过滤
- **WebSocket 连接** - 实时日志更新
- **响应式设计** - 适配不同屏幕尺寸

## API 集成

前端通过以下方式与后端通信：

- **REST API** - 获取文件列表、日志内容、搜索等
- **WebSocket** - 实时日志更新推送

API 服务位于 `src/services/api.ts`，WebSocket 服务位于 `src/services/websocket.ts`。