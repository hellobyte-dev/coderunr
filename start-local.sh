#!/bin/bash

# CodeRunr Local Development Setup
# This script builds packages and starts the local repository

set -e

echo "🚀 Starting CodeRunr Local Development Environment"

# Check if packages are already built
echo "📦 Checking packages..."
cd packages

PACKAGES_EXIST=true
if [[ ! -f "python-3.12.0.pkg.tar.gz" ]] || [[ ! -f "go-1.16.2.pkg.tar.gz" ]] || [[ ! -f "java-15.0.2.pkg.tar.gz" ]]; then
    PACKAGES_EXIST=false
fi

if [[ "$PACKAGES_EXIST" == "false" ]]; then
    echo "🔨 Building packages (this may take a few minutes)..."
    make build-all || echo "⚠️  Some package builds failed, continuing..."
else
    echo "✅ Packages already exist, skipping build"
fi

cd ..

# Generate package index for repo
echo "📑 Generating package index..."
cd repo
./mkindex.sh || echo "⚠️  Index generation failed"
cd ..

# Start services with Docker Compose (force rebuild images)
echo "🐳 Starting services (rebuilding images)..."
docker compose up -d --build

echo "✅ CodeRunr services started!"
echo ""
echo "🌐 Services:"
echo "  - API Server: http://localhost:2000"
echo "  - Package Repo: http://localhost:8000"
echo ""
echo "📝 Test the API:"
echo "  curl http://localhost:2000/api/v2/packages"
echo ""
echo "🛑 Stop services with: docker compose down"
