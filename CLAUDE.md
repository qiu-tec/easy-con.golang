# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is **easy-con**, a Go-based MQTT communication library that simplifies inter-module communication over MQTT protocols. The library provides a clean abstraction layer with support for clients, monitoring, and mqttProxy functionality.

## Development Commands

### Building
```bash
# Build the library
go build .

# Build specific applications
go build ./app/mqttProxy
```

### Testing
```bash
# Run all tests
go test ./...

# Run specific test suites
go test ./unitTest/expressTest
go test ./unitTest/monitor
go test ./unitTest/mqttProxy
go test ./unitTest/versionAndExit

# Run tests with verbose output
go test -v ./...

# Run a specific test
go test -run TestName ./unitTest/expressTest
```

### Dependencies
```bash
# Download dependencies
go mod download

# Tidy up go.mod
go mod tidy

# Verify dependencies
go mod verify
```

## Architecture Overview

### Core Components

#### 1. **Interface Layer** (`interface.go`)
- **IAdapter**: Basic MQTT communication interface
- **IMonitor**: Extended interface for monitoring (inherits from IAdapter)
- **IProxy**: Proxy functionality interface
- **iLogger**: Logging interface
- Setting structures for configuration

#### 2. **Protocol Layer** (`protocol.go`)
- Message pack/unpack utilities (`PackReq`, `PackResp`, `PackNotice`, `PackLog`)
- Topic generation functions (automatic topic naming based on clients)
- Atomic ID generation for unique message IDs
- HTTP-style status codes for responses

#### 3. **Implementation Layer**
- **mqttAdapter.go**: Core MQTT client implementation
- **mqttMonitor.go**: Monitoring and module discovery functionality
- **mqttProxy.go**: Forward and reverse mqttProxy functionality

#### 4. **Supporting Components**
- **cgo/**: C language bindings for non-Go applications
- **app/mqttProxy/**: Ready-to-use mqttProxy application
- **unitTest/**: Comprehensive test suites

### Key Design Patterns

#### **Interface-Based Architecture**
All major components implement well-defined interfaces, making the system highly testable and extensible.

#### **Channel-Based Communication**
Uses buffered channels for async message handling with proper correlation of requests/responses.

#### **Topic-Based Routing**
Automatic topic generation: `Request_{module}` and `Response_{module}` with optional prefixes.

#### **Message Protocol**
Structured message packs with consistent patterns:
- **PackReq**: Request messages with From/To/Route/Content
- **PackResp**: Response messages with status codes
- **PackNotice**: Notification messages
- **PackLog**: Log messages with levels

## Configuration Patterns

### Basic Module Setup
```go
setting := easyCon.NewDefaultMqttSetting("ModuleName", "ws://broker:port/mqtt", onRequest, onStatusChanged)
setting.UID = "username"
setting.PWD = "password"
adapter := easyCon.NewMqttAdapter(setting)
```

### Monitor Setup
```go
monitorSetting := easyCon.NewMonitorSetting("MonitorName", brokerAddr, onReqRec, onStatusChanged)
monitorSetting.OnNoticeRec = handleNotices
monitorSetting.OnLogRec = handleLogs
monitor := easyCon.NewMqttMonitor(monitorSetting)
```

### Proxy Setup
```go
proxySetting := easyCon.NewProxySetting("ProxyName", brokerAddr, onReqRec, onStatusChanged)
proxySetting.ForwardTo = "target-module"
proxySetting.Reverse = true/false
mqttProxy := easyCon.NewMqttProxy(proxySetting)
```

## Testing Strategy

### Test Categories
1. **Express Tests** (`unitTest/expressTest/`): High-performance concurrency testing
2. **Monitor Tests** (`unitTest/monitor/`): Monitoring functionality validation
3. **Proxy Tests** (`unitTest/mqttProxy/`): Forward and reverse mqttProxy testing
4. **Integration Tests**: CGO bindings and version handling

### Test Patterns
- Uses standard Go testing framework with `testing.T`
- Concurrent testing with goroutines and `sync.WaitGroup`
- Comprehensive callback function testing
- Error condition and edge case testing

## Development Guidelines

### Adding New Features
1. Define interfaces first in `interface.go`
2. Implement protocol logic in `protocol.go` if needed
3. Create implementation in appropriate file (adapter/monitor/mqttProxy)
4. Add comprehensive tests in `unitTest/`
5. Update examples in README.md if applicable

### Error Handling
- Use HTTP-style status codes from `EResp` enum
- Log errors using the `PackLog` structure
- Implement proper error propagation through callbacks
- Handle connection loss gracefully with retry logic

### Thread Safety
- Use atomic operations for counters and IDs
- Protect shared state with mutexes
- Use buffered channels for message passing
- Ensure proper cleanup in `Stop()` methods

## Module Structure

### Core Library Files
- `interface.go`: Interface definitions and settings
- `protocol.go`: Message protocol and utilities
- `mqttAdapter.go`: Main MQTT implementation
- `mqttMonitor.go`: Monitoring functionality
- `mqttProxy.go`: Proxy functionality

### Supporting Directories
- `cgo/`: C language bindings
- `app/`: Example applications
- `unitTest/`: Test suites organized by functionality

## Dependencies

Main dependencies:
- `github.com/eclipse/paho.mqtt.golang` v1.4.3 - MQTT client
- `github.com/spf13/viper` v1.19.0 - Configuration management

## Build Requirements

- Go 1.20 or higher
- Standard Go toolchain (no special build tools required)
- MQTT broker for testing (e.g., Mosquitto, EMQX)
