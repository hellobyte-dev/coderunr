# CodeRunr Development Makefile

.PHONY: help start stop build packages clean logs test

# Default target
help:
	@echo "CodeRunr Development Commands:"
	@echo ""
	@echo "  start     - Start local development environment"
	@echo "  stop      - Stop all services"
	@echo "  build     - Build all Docker images"
	@echo "  packages  - Build language packages"
	@echo "  clean     - Clean up containers and volumes"
	@echo "  logs      - Show service logs"
	@echo "  test      - Test API endpoints"

# Start local development environment
start:
	@echo "🚀 Starting CodeRunr local development..."
	./start-local.sh

# Stop all services
stop:
	@echo "🛑 Stopping CodeRunr services..."
	docker-compose down

# Build Docker images
build:
	@echo "🏗️  Building Docker images..."
	docker-compose build

# Build language packages only
packages:
	@echo "📦 Building packages..."
	cd packages && make build-all

# Clean up everything
clean:
	@echo "🧹 Cleaning up..."
	docker-compose down -v --remove-orphans
	docker system prune -f
	cd packages && make clean

# Show logs
logs:
	@echo "📋 Showing service logs..."
	docker-compose logs -f

# Test API
test:
	@echo "🧪 Testing API endpoints..."
	@echo "Testing health endpoint..."
	curl -s http://localhost:2000/api/v2/packages || echo "❌ API not responding"
	@echo ""
	@echo "Testing package list..."
	curl -s http://localhost:2000/api/v2/packages | jq '.' || echo "❌ Packages endpoint failed"
