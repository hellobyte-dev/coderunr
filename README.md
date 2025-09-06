# CodeRunr

CodeRunr is a modern code execution engine that provides secure, sandboxed execution of code in multiple programming languages.

## Project Structure

```
coderunr/
├── api/                     # Go API implementation
│   ├── cmd/server/         # Main application entry point
│   ├── internal/           # Internal Go packages
│   │   ├── config/        # Configuration management
│   │   ├── handler/       # HTTP handlers and WebSocket
│   │   ├── job/           # Job execution and management
│   │   ├── middleware/    # HTTP middleware
│   │   ├── runtime/       # Runtime and package management
│   │   └── types/         # Internal type definitions
│   ├── Dockerfile         # Container configuration
│   ├── Makefile          # Build and development tools
│   └── README.md         # Detailed API documentation
└── README.md             # This file
```

## Quick Start

### Using the Management Script (Recommended)

CodeRunr provides a unified management script similar to Piston's approach:

```bash
# Show all available commands
./coderunr help

# Start development environment
./coderunr start

# Check service health
./coderunr health

# Use CLI directly through the script
./coderunr execute python3 "print('Hello World')"
```

See [MANAGEMENT.md](MANAGEMENT.md) for complete management script documentation.

### Manual Setup

### Prerequisites

- Go 1.21 or later
- Linux isolate (for sandboxing)
- Docker (optional)

### Running the API Server

```bash
cd api
make build
make run
```

### Development

```bash
cd api
make dev
```

### Testing

```bash
cd api
make test
```

### Docker

```bash
cd api
make docker-run
```

## API Documentation

For detailed API documentation, configuration options, and usage examples, see [api/README.md](api/README.md).

## Features

- **Multi-language Support**: Execute code in various programming languages
- **Secure Sandboxing**: Uses Linux isolate for secure code execution
- **RESTful API**: Standard HTTP API with JSON responses
- **WebSocket Support**: Real-time code execution with streaming output
- **Resource Limits**: Configurable CPU, memory, and time limits
- **Package Management**: Support for language-specific package installations

## Environment Variables

All configuration is done via environment variables with the `CODERUNR_` prefix:

- `CODERUNR_BIND_ADDRESS`: Server bind address (default: `0.0.0.0:2000`)
- `CODERUNR_DATA_DIRECTORY`: Data directory for packages (default: `/coderunr`)
- `CODERUNR_LOG_LEVEL`: Log level (default: `info`)

See `api/config.env.example` for all available configuration options.

## License

MIT License - see [api/LICENSE](api/LICENSE) for details.
