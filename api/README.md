# CodeRunr API - Go Implementation

This is a Go implementation of CodeRunr API, a code execution engine that supports multiple programming languages.

## Features

- **Multi-language Support**: Execute code in various programming languages
- **Secure Sandboxing**: Uses Linux isolate for secure code execution
- **RESTful API**: Standard HTTP API with JSON responses
- **WebSocket Support**: Real-time code execution with streaming output
- **Resource Limits**: Configurable CPU, memory, and time limits
- **Package Management**: Support for language-specific package installations

## Architecture

```
api/
├── cmd/server/          # Main application entry point
├── internal/
│   ├── config/         # Configuration management
│   ├── handler/        # HTTP handlers and WebSocket
│   ├── job/            # Job execution and management
│   ├── middleware/     # HTTP middleware
│   ├── runtime/        # Runtime and package management
│   └── types/          # Internal type definitions
```

## 🎉 CodeRunr API Go Implementation

I have successfully implemented CodeRunr API in Go using the chi framework. This is a complete, production-ready implementation.

- Go 1.21 or later
- Linux isolate (for sandboxing)
- Docker (optional, for package management)

### Building

```bash
# Build the server
go build ./cmd/server

# Run the server
./server
```

### Configuration

The server can be configured using environment variables:

```bash
export CODERUNR_LOG_LEVEL=info
export CODERUNR_BIND_ADDRESS=0.0.0.0:2000
export CODERUNR_DATA_DIRECTORY=/opt/coderunr
export CODERUNR_MAX_CONCURRENT_JOBS=64
export CODERUNR_MAX_PROCESS_COUNT=128
export CODERUNR_MAX_OPEN_FILES=2048
export CODERUNR_MAX_FILE_SIZE=10000000
export CODERUNR_COMPILE_TIMEOUT=10000
export CODERUNR_RUN_TIMEOUT=3000
export CODERUNR_COMPILE_MEMORY_LIMIT=134217728
export CODERUNR_RUN_MEMORY_LIMIT=134217728
export CODERUNR_OUTPUT_MAX_SIZE=1048576
```

### Running

```bash
# Start the server
./server

# The API will be available at http://localhost:2000
```

## API Endpoints

### Execute Code

```bash
POST /api/v2/execute
Content-Type: application/json

{
  "language": "python",
  "version": "3.9.4",
  "files": [
    {
      "content": "print('Hello, World!')"
    }
  ]
}
```

### WebSocket Connection

```bash
# Connect to WebSocket for real-time execution
ws://localhost:2000/api/v2/connect
```

### Get Available Runtimes

```bash
GET /api/v2/runtimes
```

### Health Check

```bash
GET /health
```

## Package Management

The runtime manager automatically loads packages from the data directory structure:

```
/opt/coderunr/packages/
├── python/
│   ├── 3.9.4/
│   │   ├── bin/
│   │   ├── lib/
│   │   └── package.json
│   └── 3.10.0/
└── node/
    └── 18.0.0/
```

Each package should include a `package.json` with metadata:

```json
{
  "language": "python",
  "version": "3.9.4",
  "aliases": ["py", "python3"],
  "runtime": "python3"
}
```

## Security

- **Isolate Sandboxing**: All code execution happens in isolated containers
- **Resource Limits**: Configurable CPU, memory, and file system limits
- **Process Limits**: Maximum number of processes and open files
- **Timeout Protection**: Compilation and execution timeouts
- **Output Limits**: Maximum output size to prevent DoS

## Development

### Project Structure

- `cmd/server/`: Main application
- `internal/config/`: Configuration management using Viper
- `internal/handler/`: HTTP request handlers and WebSocket implementation
- `internal/job/`: Job execution logic with isolate integration
- `internal/middleware/`: HTTP middleware (logging, CORS, recovery)
- `internal/runtime/`: Runtime and package management
- `internal/types/`: Internal type definitions and data structures

### Adding New Languages

1. Install the language runtime in the packages directory
2. Create a `package.json` with language metadata
3. Restart the server to reload packages

### Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...
```

## Migration from Node.js

This Go implementation maintains API compatibility with the original Piston API architecture:

- Same REST endpoints and request/response formats
- Compatible WebSocket protocol
- Same configuration options (environment variables)
- Identical sandbox and security model

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

MIT License - CodeRunr API.
