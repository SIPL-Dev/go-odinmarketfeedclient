# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-11-26

### Added
- Initial release of ODIN Market Feed Client
- WebSocket connectivity with SSL/TLS support
- ZLIB compression and decompression
- Message fragmentation and defragmentation handling
- Thread-safe operations with mutex locks
- Binary data parsing for market data
- Time conversion for market timestamps
- Comprehensive documentation and examples
- MIT License

### Features
- **Compression**: Efficient ZLIB compression for data transmission
- **Fragmentation**: Handles large messages split across multiple packets
- **Reconnection**: Manual disconnect and reconnect support
- **Multiple Markets**: Support for NSE, BSE, and other market segments
- **Type Safety**: Strongly typed structures and constants
- **Error Handling**: Comprehensive error handling throughout


[Unreleased]: https://github.com/SIPL-Dev/go-odinmarketfeedclient/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/SIPL-Dev/go-odinmarketfeedclient/releases/tag/v1.0.0
