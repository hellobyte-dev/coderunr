# CodeRunr CLI

A command-line interface for CodeRunr code execution engine.

## Quick Start

```bash
# Build
go build -o coderunr-cli .

# Execute code  
./coderunr-cli execute python script.py
./coderunr-cli execute go main.go -- arg1 arg2

# Interactive mode with real-time streaming
./coderunr-cli execute python script.py --interactive

# List available runtimes
./coderunr-cli list

# Package management
./coderunr-cli package list
./coderunr-cli package install python numpy
```

## Features

- **Multi-language**: Python, Go, Java, and more
- **Interactive mode**: Real-time WebSocket streaming  
- **Package management**: Install/uninstall packages
- **Flexible**: Stdin input, file arguments, timeouts
- **Compatible**: CodeRunr API v2 & Piston API

## Commands

| Command | Description | Example |
|---------|-------------|---------|
| `execute` | Run code files | `execute python script.py` |
| `list` | Show runtimes | `list --verbose` |
| `package` | Manage packages | `package list --language python` |
| `version` | Show version | `version` |

## Configuration

```bash
# Global flags
--url http://localhost:2000    # API server URL
--verbose                      # Detailed output  
--output json                  # Output format

# Execute flags  
--interactive                  # WebSocket mode
--language-version 3.9.4       # Specific version
--run-timeout 5000             # Timeout in ms
--files utils.py,config.json   # Additional files
```

## Testing

```bash
# Quick smoke test
./simple-test.sh

# Basic tests (no API needed)
./simple-test.sh basic

# Full comprehensive test  
./simple-test.sh full
```

## Development

**Requirements**: Go 1.21+, CodeRunr API server

**Dependencies**: Cobra, Gorilla WebSocket, Fatih Color  

**API Compatibility**: CodeRunr API v2, Piston API

## License

MIT
