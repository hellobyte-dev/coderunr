#!/bin/bash

# Test CodeRunr Local Repository Setup

set -e

echo "🧪 Testing CodeRunr Local Setup"

# Check if services are running
echo "1️⃣ Checking if services are running..."

if curl -s http://localhost:8000 > /dev/null; then
    echo "✅ Package repository is running at http://localhost:8000"
else
    echo "❌ Package repository is not accessible"
fi

if curl -s http://localhost:2000/api/v2/packages > /dev/null; then
    echo "✅ CodeRunr API is running at http://localhost:2000"
else
    echo "❌ CodeRunr API is not accessible"
fi

# Test package repository index
echo ""
echo "2️⃣ Testing package repository index..."
if curl -s http://localhost:8000/index; then
    echo "✅ Package index is accessible"
else
    echo "❌ Package index is not accessible"
fi

# Test API endpoints
echo ""
echo "3️⃣ Testing API endpoints..."

echo "Testing /api/v2/packages:"
curl -s http://localhost:2000/api/v2/packages | jq '.[0:3]' || echo "❌ Packages endpoint failed"

echo ""
echo "Testing /api/v2/runtimes:"
curl -s http://localhost:2000/api/v2/runtimes | jq '.[0:3]' || echo "❌ Runtimes endpoint failed"

# Test code execution
echo ""
echo "4️⃣ Testing code execution..."

# Python test
echo "Testing Python execution:"
curl -s -X POST http://localhost:2000/api/v2/execute \
  -H "Content-Type: application/json" \
  -d '{
    "language": "python", 
    "version": "3.12.0",
    "files": [{"content": "print(\"Hello from CodeRunr Python!\")"}]
  }' | jq '.run.stdout' || echo "❌ Python execution failed"

echo ""
echo "✅ Local setup testing completed!"
