# CodeRunr Management Script

The `coderunr` script is inspired by Piston's management approach, providing a single entry point for all CodeRunr operations.

## Quick Start

```bash
# Show help
./coderunr help

# Start services in development mode
./coderunr start

# Check service status and health
./coderunr status
./coderunr health

# Build a specific package
./coderunr build-pkg python 3.12.0

# Use CLI passthrough (same as running coderunr-cli directly)
./coderunr execute python3 "print('Hello World')"
```

## Environment Management

```bash
# Switch to development environment (default)
./coderunr select dev

# Switch to production environment  
./coderunr select prod

# Check current environment
./coderunr help  # Shows current environment at the top
```

## Service Management

```bash
# Start all services
./coderunr start

# Stop all services
./coderunr stop

# Restart all services or specific service
./coderunr restart
./coderunr restart api

# Show logs for all services or specific service
./coderunr logs
./coderunr logs api

# Get shell access (default: api container)
./coderunr bash
./coderunr bash repo
```

## Development Commands

```bash
# List available packages
./coderunr list-pkgs

# Build specific package
./coderunr build-pkg <language> <version>

# Build all packages
./coderunr build-all-pkgs

# Clean operations
./coderunr clean-pkgs      # Clean package build artifacts
./coderunr clean-repo      # Clean repository data
./coderunr clean-all       # Clean everything (containers, images, volumes)

# Development tools
./coderunr lint           # Lint codebase
./coderunr format         # Format codebase
./coderunr test           # Run API tests
./coderunr test-e2e       # Run end-to-end tests

# API development
./coderunr dev-api        # Run API in development mode
./coderunr dev-setup      # Setup development environment
```

## Advanced Usage

```bash
# Direct docker-compose access
./coderunr docker_compose ps
./coderunr docker_compose build api

# Update system
./coderunr update         # Pull latest code and rebuild

# System maintenance
./coderunr rebuild        # Rebuild and restart containers
```

## CLI Passthrough

Any command not recognized by the management script is passed through to the CodeRunr CLI:

```bash
./coderunr list-runtimes
./coderunr execute go 'fmt.Println("Hello")'
./coderunr package list
```

## Environment Files

- `.coderunr_env` - Current environment setting (dev/prod)
- `docker-compose.yml` - Default compose file
- `docker-compose.dev.yml` - Development-specific compose file (if exists)
- `docker-compose.prod.yml` - Production-specific compose file (if exists)

## Comparison with Piston

| Feature | Piston | CodeRunr | Notes |
|---------|--------|----------|--------|
| Single entry point | ✅ | ✅ | Both provide unified management |
| Environment switching | ✅ | ✅ | dev/prod environments |
| CLI passthrough | ✅ | ✅ | Direct tool access |
| Package building | ✅ | ✅ | Build individual or all packages |
| Docker integration | ✅ | ✅ | Container management |
| Health checking | ❌ | ✅ | CodeRunr adds health monitoring |
| Development tools | ✅ | ✅ | Linting, formatting, testing |
| Update mechanism | ✅ | ✅ | Automated updates |
