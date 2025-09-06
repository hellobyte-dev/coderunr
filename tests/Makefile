# CodeRunr Tests Makefile

.PHONY: test test-e2e test-integration test-shell coverage clean help

# Default target
help:
	@echo "CodeRunr Test Commands:"
	@echo ""
	@echo "  test            - Run all tests"
	@echo "  test-e2e        - Run E2E tests only"
	@echo "  test-integration- Run integration tests only"
	@echo "  test-shell      - Run shell smoke tests only"
	@echo "  coverage        - Generate test coverage report"
	@echo "  clean           - Clean test artifacts"

# Run all tests
test:
	@echo "🧪 Running all tests..."
	go test ./... -v

# Run E2E tests only
test-e2e:
	@echo "⚡ Running E2E tests..."
	go test ./e2e -v

# Run integration tests only
test-integration:
	@echo "🔗 Running integration tests..."
	go test ./integration -v

# Run shell smoke tests only
test-shell:
	@echo "🐚 Running shell smoke tests..."
	./scripts/smoke-test.sh

# Generate coverage report
coverage:
	@echo "📊 Generating coverage report..."
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean test artifacts
clean:
	@echo "🧹 Cleaning test artifacts..."
	rm -f coverage.out coverage.html
