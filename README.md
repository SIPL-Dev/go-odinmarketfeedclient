# ODIN Market Feed Client (Go Library)

A robust Go Library for connecting to ODIN Market Feed WebSocket API. This library provides real-time market data streaming with built-in compression, fragmentation handling, and subscription management.

## Features

- 🚀 **WebSocket Connectivity** - Reliable WebSocket connection with automatic reconnection
- 📦 **Compression Support** - Built-in ZLIB compression/decompression
- 🔀 **Message Fragmentation** - Automatic handling of fragmented messages
- 📊 **Market Data Subscriptions** - Subscribe to ChannelNum, SnapQuote, and BestFive feeds
- ❤️ **Heartbeat Management** - Automatic heartbeat to keep connection alive
- 🔒 **Thread-Safe** - Concurrent-safe operations with mutex locks
- ⚡ **High Performance** - Efficient binary data parsing

## Installation

```bash
go get github.com/SIPL-Dev/go-odinmarketfeedclient
```

## Quick Start

```go
package main

import (
    "fmt"
    "time"
    
    odin "github.com/SIPL-Dev/go-odinmarketfeedclient"
)

func main() {
    // Create a new client
    client := odin.ODINMarketFeedClient()
    
    // Set up event handlers
    client.OnOpen = func() {
        fmt.Println("✓ Connected to ODIN Market Feed")
    }
    
    client.OnMessage = func(message string) {
        fmt.Println("📨 Message:", message)
    }
    
    client.OnError = func(err string) {
        fmt.Println("❌ Error:", err)
    }
    
    client.OnClose = func(code int, reason string) {
        fmt.Printf("🔌 Connection closed: %d - %s\n", code, reason)
    }
    
    // Connect to the server
    err := client.Connect("your-host.com", 8080, true, "your_user_id")
    if err != nil {
        panic(err)
    }
    
    // Subscribe to market data
    client.SubscribeLTPTouchline("token1,token2", 1)
    
    // Keep connection alive
    time.Sleep(60 * time.Second)
    
    // Disconnect when done
    client.Disconnect()
}
```

## API Reference

### Client Creation

#### `NewODINMarketFeedClient() *ODINMarketFeedClient`
Creates a new instance of the ODIN Market Feed client.

```go
client := odin.ODINMarketFeedClient()
```

### Connection Management

#### `Connect(host string, port int, ssl bool, userID string) error`
Establishes a WebSocket connection to the ODIN Market Feed server.

**Parameters:**
- `host` - Server hostname (e.g., "market.example.com")
- `port` - Server port (e.g., 8080)
- `ssl` - Use secure WebSocket (true for wss://, false for ws://)
- `userID` - Your user identifier

**Example:**
```go
err := client.Connect("market.example.com", 8080, true, "USER123")
```

#### `Disconnect()`
Gracefully closes the WebSocket connection.

```go
client.Disconnect()
```


## Requirements

- Go 1.21 or higher
- `github.com/gorilla/websocket` v1.5.1

## Examples

See the [examples](examples/) directory for more detailed usage examples:

- [basic_usage.go](examples/basic_usage.go) - Simple connection and subscription


## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For issues, questions, or contributions, please open an issue on GitHub.

## Disclaimer

This SDK is provided as-is. Please ensure you comply with ODIN Market Feed's terms of service and usage policies. The authors are not responsible for any financial losses or damages resulting from the use of this library.

## Changelog

### v1.0.0 (Initial Release)
- WebSocket connectivity with SSL support
- ZLIB compression/decompression
- Message fragmentation handling
- ChannelNum, SnapQuote, and BestFive subscriptions
- Automatic heartbeat management
- Thread-safe operations
