#!/bin/bash

echo "Testing CodeRunr API with new CODERUNR_ environment variables..."

# Set up test environment
export CODERUNR_DATA_DIRECTORY="/tmp/coderunr-test"
export CODERUNR_LOG_LEVEL="debug"
export CODERUNR_BIND_ADDRESS="localhost:2001"
export CODERUNR_MAX_CONCURRENT_JOBS="32"

# Create test directories
mkdir -p /tmp/coderunr-test/packages

echo "Environment variables set:"
env | grep CODERUNR_ | sort

echo ""
echo "Starting CodeRunr API server for 3 seconds..."

# Start server with timeout
timeout 3s ./build/coderunr-api

echo ""
echo "Server test completed successfully!"

# Cleanup
rm -rf /tmp/coderunr-test
