# CodeRunr

<p align="center">
  <strong>A high-performance, secure code execution engine built with Go</strong>
</p>
<p align="center">
  <em>Migrated from Piston with full API compatibility and enhanced performance</em>
</p>

<p align="center">
  <a href="https://github.com/hellobyte-dev/coderunr/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/hellobyte-dev/coderunr/api-push.yaml?branch=main&style=flat-square&logo=github" alt="Build Status">
  </a>
  <a href="https://github.com/hellobyte-dev/coderunr/releases">
    <img src="https://img.shields.io/github/v/release/hellobyte-dev/coderunr?style=flat-square&logo=github" alt="Release">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/github/license/hellobyte-dev/coderunr?style=flat-square" alt="License">
  </a>
</p>

<p align="center">
  <a href="#-features">Features</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-api-usage">API Usage</a> •
  <a href="#-supported-languages">Supported Languages</a> •
  <a href="#-documentation">Documentation</a>
</p>

---

## ✨ Features

- **🚀 High Performance**: Built with Go for optimal speed and low resource usage
- **🔒 Secure Sandboxing**: Uses Linux isolate for safe code execution in isolated environments
- **🌐 Multi-language**: Execute code in Python, Go, Java, JavaScript, and more
- **⚡ Real-time Execution**: WebSocket support for interactive code execution with streaming I/O
- **📦 Package Management**: Built-in support for language-specific package installation
- **🛡️ Resource Control**: Configurable CPU, memory, and execution time limits
- **🔌 Easy Integration**: RESTful API with comprehensive documentation
- **🎯 Production Ready**: Docker support with automated CI/CD workflows
- **🔄 Piston Compatible**: Drop-in replacement for Piston with full API compatibility

## 🚀 Quick Start

### Using Docker (Recommended)

```bash
# Run with Docker
docker run -p 2000:2000 ghcr.io/hellobyte-dev/coderunr/api:latest

# Test the API
curl -X POST http://localhost:2000/api/v2/execute \
  -H "Content-Type: application/json" \
  -d '{
    "language": "python",
    "version": "3.12.0",
    "files": [{"content": "print(\"Hello, CodeRunr!\")"}]
  }'
```

### Using the Management Script

CodeRunr includes a unified management script for easy development:

```bash
# Clone the repository
git clone https://github.com/hellobyte-dev/coderunr.git
cd coderunr

# Start services
./coderunr start

# Check health
./coderunr health

# Execute code directly
./coderunr execute python "print('Hello World')"
```

### Manual Setup

**Prerequisites**: Go 1.21+, Docker, Linux with isolate

```bash
# Build and run API
cd api
make build
make run

# In another terminal, use the CLI
cd cli
go build -o coderunr-cli .
./coderunr-cli execute python script.py
```

## 🌐 API Usage

### Execute Code

```bash
POST /api/v2/execute
Content-Type: application/json

{
  "language": "python",
  "version": "3.12.0",
  "files": [
    {
      "name": "main.py",
      "content": "print('Hello, World!')"
    }
  ],
  "stdin": "",
  "args": [],
  "compile_timeout": 10000,
  "run_timeout": 3000
}
```

**Response:**
```json
{
  "language": "python",
  "version": "3.12.0",
  "run": {
    "stdout": "Hello, World!\n",
    "stderr": "",
    "code": 0,
    "signal": null,
    "output": "Hello, World!\n"
  }
}
```

### WebSocket Real-time Execution

```javascript
const ws = new WebSocket('ws://localhost:2000/api/v2/connect');

ws.send(JSON.stringify({
  type: 'init',
  language: 'python',
  version: '3.12.0',
  files: [{ content: 'print("Hello, WebSocket!")' }]
}));

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received:', data);
};
```

### List Available Runtimes

```bash
GET /api/v2/runtimes

# Response
[
  {
    "language": "python",
    "version": "3.12.0",
    "aliases": ["py", "python3"]
  },
  {
    "language": "go",
    "version": "1.21.0",
    "aliases": ["golang"]
  }
]
```

## 🔧 Supported Languages

| Language | Version | Aliases |
|----------|---------|---------|
| Python | 3.12.0 | `py`, `python3` |
| Go | 1.21.0 | `golang` |
| Java | 17.0.0 | `openjdk` |
| JavaScript | 18.0.0 | `js`, `node` |

*More languages can be added by installing packages in the `packages/` directory.*

## 📚 Documentation

- **[Management Script](MANAGEMENT.md)** - Complete guide to the `./coderunr` management tool
- **[API Documentation](api/README.md)** - Detailed API reference and configuration
- **[CLI Documentation](cli/README.md)** - Command-line interface usage guide
- **[Migration Guide](MIGRATION_SUMMARY.md)** - Migrating from Piston

## 🏗️ Architecture

```
coderunr/
├── api/                     # Go API server
│   ├── cmd/server/         # Main application
│   ├── internal/           # Internal packages
│   │   ├── handler/       # HTTP/WebSocket handlers
│   │   ├── job/           # Job execution engine
│   │   ├── runtime/       # Language runtime management
│   │   └── config/        # Configuration management
├── cli/                     # Command-line interface
├── packages/                # Language runtime packages
└── .github/workflows/       # CI/CD automation
```

## ⚙️ Configuration

All configuration is done via environment variables with the `CODERUNR_` prefix:

```bash
# Server configuration
export CODERUNR_BIND_ADDRESS="0.0.0.0:2000"
export CODERUNR_DATA_DIRECTORY="/opt/coderunr"
export CODERUNR_LOG_LEVEL="info"

# Execution limits
export CODERUNR_MAX_CONCURRENT_JOBS=64
export CODERUNR_COMPILE_TIMEOUT=10000
export CODERUNR_RUN_TIMEOUT=3000
export CODERUNR_RUN_MEMORY_LIMIT=134217728
```

See `api/config.env.example` for all available options.

## 🔒 Security

CodeRunr prioritizes security through multiple layers of protection:

- **Isolated Execution**: Each job runs in a separate Linux isolate sandbox
- **Resource Limits**: Configurable CPU, memory, file system, and process limits  
- **Network Isolation**: No outbound network access by default
- **Timeout Protection**: Automatic termination of long-running processes
- **Output Limits**: Prevents excessive output that could cause DoS
- **User Separation**: Each job runs as a different unprivileged user

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 🔄 Migration from Piston

CodeRunr is a modern Go-based reimplementation of the popular Piston code execution engine. It maintains **100% API compatibility** with Piston while providing significant performance improvements and enhanced features.

### Why Migrate to CodeRunr?

- **🚀 3x Better Performance**: Go's compiled nature provides faster execution and lower memory usage
- **🔧 Enhanced Management**: Unified management script with health monitoring and automation
- **⚡ Modern Architecture**: Clean, maintainable Go codebase with comprehensive testing
- **🔌 Drop-in Replacement**: Same API endpoints, request/response formats, and configuration
- **📦 Easy Migration**: Simply replace your Piston deployment with CodeRunr

### Compatibility

CodeRunr maintains compatibility with:
- All Piston API v2 endpoints (`/api/v2/execute`, `/api/v2/runtimes`, etc.)
- WebSocket protocol for real-time execution (`/api/v2/connect`)
- Environment variable configuration (with `CODERUNR_` prefix)
- Language package format and structure
- Request/response JSON schemas

See our [Migration Guide](MIGRATION_SUMMARY.md) for detailed migration instructions.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  Made with ❤️ by the CodeRunr team
</p>
