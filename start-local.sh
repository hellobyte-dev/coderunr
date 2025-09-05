#!/bin/bash

# CodeRunr Local Development Setup
# This script builds packages and starts the local repository

set -e

echo "🚀 Starting CodeRunr Local Development Environment"

# Build packages
echo "📦 Building packages..."
cd packages

# Build available packages
if command -v jq >/dev/null 2>&1; then
    echo "Building Python 3.12.0..."
    make python-3.12.0.pkg.tar.gz || echo "⚠️  Python build failed, continuing..."
    
    echo "Building Go 1.16.2..."
    make go-1.16.2.pkg.tar.gz || echo "⚠️  Go build failed, continuing..."
    
    echo "Building Java 15.0.2..."
    make java-15.0.2.pkg.tar.gz || echo "⚠️  Java build failed, continuing..."
else
    echo "⚠️  jq not found, skipping package builds"
fi

cd ..

# Generate package index for repo
echo "📑 Generating package index..."
cd repo
./mkindex.sh || echo "⚠️  Index generation failed"
cd ..

# Start services with Docker Compose
echo "🐳 Starting services..."
docker-compose up -d

echo "✅ CodeRunr services started!"
echo ""
echo "🌐 Services:"
echo "  - API Server: http://localhost:2000"
echo "  - Package Repo: http://localhost:8000"
echo ""
echo "📝 Test the API:"
echo "  curl http://localhost:2000/api/v2/packages"
echo ""
echo "🛑 Stop services with: docker-compose down"
