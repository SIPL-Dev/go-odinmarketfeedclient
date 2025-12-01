# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- Connection retry with exponential backoff
- More comprehensive unit tests
- Performance optimizations
- Better error handling and logging

## [1.0.0] - 2025-11-26

### Added
- Initial release of ODIN Market Feed Client
- WebSocket connectivity with SSL/TLS support
- ZLIB compression and decompression
- Message fragmentation and defragmentation handling
- Market data subscription support:
  - ChannelNum subscription/unsubscription
  - SnapQuote subscription/unsubscription
  - BestFive subscription/unsubscription
- Automatic heartbeat mechanism to keep connection alive
- Thread-safe operations with mutex locks
- Event handlers for connection lifecycle:
  - OnOpen - Connection established
  - OnMessage - Message received
  - OnError - Error occurred
  - OnClose - Connection closed
- Binary data parsing for market data
- Time conversion for market timestamps
- Comprehensive documentation and examples
- MIT License
- GitHub Actions CI/CD workflow
- Unit tests and benchmarks

### Features
- **Compression**: Efficient ZLIB compression for data transmission
- **Fragmentation**: Handles large messages split across multiple packets
- **Reconnection**: Manual disconnect and reconnect support
- **Multiple Markets**: Support for NSE, BSE, and other market segments
- **Type Safety**: Strongly typed structures and constants
- **Error Handling**: Comprehensive error handling throughout

### Documentation
- Detailed README with API reference
- Quick start guide
- Multiple usage examples
- Contributing guidelines
- Publishing guide for maintainers

[Unreleased]: https://github.com/yourusername/odin-market-feed-client/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/yourusername/odin-market-feed-client/releases/tag/v1.0.0
