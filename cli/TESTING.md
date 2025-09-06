# Testing

Quick validation of CLI functionality.

## Usage

```bash
./simple-test.sh          # Default: essential tests
./simple-test.sh basic     # Version + help only (no API)  
./simple-test.sh full      # Comprehensive test suite
```

## Test Modes

| Mode | Tests | Requirements |
|------|-------|-------------|
| `basic` | Version, help | CLI binary only |
| `quick` | Core functionality | + API server |
| `full` | All languages | + API server |

## What's Tested

- ✅ Commands: version, help, list, execute, package
- ✅ Languages: Python, Go, Java  
- ✅ Features: interactive mode, arguments, error handling
- ✅ API: Runtime discovery, package management

## Requirements

**API Server**: `http://localhost:2000` (for quick/full modes)

**Auto-build**: Script builds CLI if missing

## CI/CD Integration

```bash
# Fast check  
./simple-test.sh basic

# Standard validation
./simple-test.sh  

# Pre-release
./simple-test.sh full
```
