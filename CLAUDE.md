# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Information

This is a log viewer application built in Go with a Vue.js frontend. It provides a web-based interface for viewing, searching, and monitoring log files in real-time. The application embeds the frontend files directly into the Go binary for single-binary deployment.

## Common Commands

### Development
```bash
# Run in development mode with test logs
make dev

# Quick development build (skip frontend if exists)
make quick

# Run the application directly
make run
```

### Building
```bash
# Build the complete application (frontend + backend)
make build

# Build only frontend
make build-frontend

# Build for all platforms
make build-all

# Create release packages
make release
```

### Testing
```bash
# Run all tests
make test

# Run specific package tests
go test ./internal/parser -v

# Run tests with coverage
make test-coverage

# Run performance benchmarks
go test ./benchmark/... -bench=. -benchmem

# Run end-to-end tests
go test ./e2e/... -v
```

### Code Quality
```bash
# Format code
make fmt

# Run linter
make lint

# Generate mocks and code
make generate
```

### Dependencies
```bash
# Install/update Go dependencies
make deps
```

### Deployment Testing
```bash
# Test deployment scripts
make test-deployment

# Test all binary builds
make test-binaries
```

### Cleanup
```bash
# Clean build artifacts
make clean
```

## Architecture Overview

### Core Components

1. **LogManager** (`internal/manager/`) - Central component managing log file access, caching, and file watching
2. **FileWatcher** (`internal/watcher/`) - Monitors log files for changes using fsnotify
3. **WebSocketHub** (`internal/server/websocket.go`) - Manages real-time log updates via WebSocket connections
4. **LogParser** (`internal/parser/`) - Factory pattern for parsing different log formats (JSON, plain text, common formats)
5. **Cache System** (`internal/cache/`) - Memory-based caching for log content and search results
6. **HTTP Server** (`internal/server/`) - Gin-based REST API and WebSocket server

### Data Flow

1. **File Discovery**: LogManager scans configured log directories
2. **File Watching**: FileWatcher monitors files for changes
3. **Real-time Updates**: Changes trigger LogUpdate events broadcasted via WebSocketHub
4. **Caching**: Frequently accessed log content cached in memory
5. **Search**: Full-text search across log files with result caching

### Key Interfaces

All major components implement interfaces defined in `internal/interfaces/interfaces.go`:
- `LogManager` - Core log management operations
- `FileWatcher` - File system monitoring
- `LogParser` - Log format parsing
- `WebSocketHub` - Real-time communication
- `SearchEngine` - Log searching functionality
- `LogCache` - Caching abstraction

### Configuration

- Configuration loaded from YAML files or command-line flags
- Example config: `config.example.yaml`
- Supports security (auth, TLS), logging levels, file size limits, cache settings
- Command-line options override config file values

## Development Guidelines

### Adding New Log Parsers
1. Implement the `LogParser` interface in `internal/parser/`
2. Register in `internal/parser/factory.go`
3. Add comprehensive tests following existing patterns

### WebSocket Message Types
WebSocket communication uses typed messages defined in `internal/types/`:
- `LogUpdate` - Real-time log file changes
- `SearchResult` - Search query responses
- Error messages for connection issues

### Testing Strategy
- Unit tests for all packages
- Integration tests in `internal/`
- End-to-end tests in `e2e/`
- Performance benchmarks in `benchmark/`
- Manual testing scripts provided

### Frontend Development
The frontend is built separately and embedded into the Go binary:
- Source files in `frontend/` (Vue.js 3)
- Built files served from embedded `web/` directory
- Use `make build-frontend` to rebuild frontend assets

## Performance Considerations

- Memory usage monitored via `internal/monitor/`
- File pool management for concurrent access
- Search result caching to avoid repeated filesystem operations
- WebSocket connection management with automatic cleanup
- Configurable cache sizes and file size limits
