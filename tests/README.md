# CodeRunr E2E Tests

End-to-end tests for the CodeRunr code execution platform.

## Quick Start

```bash
# Start services and run tests
cd ../ && ./start-local.sh
cd tests && go test ./e2e -v
```

## Commands

```bash
# All tests
go test ./e2e -v

# Smoke test (fast)
./scripts/smoke-test.sh

# Specific test
go test ./e2e -run TestCodeExecution -v
```

## What's Tested

API health, package management, Python/Go/Java execution, performance limits.

## Prerequisites

Services must be running on:
- API: http://localhost:2000  
- Repository: http://localhost:8000

Start with `../start-local.sh` before running tests.
