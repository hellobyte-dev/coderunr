#!/bin/bash

# CodeRunr CLI Test Suite
# Usage: ./test.sh [basic|quick|full]

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

CLI="./coderunr-cli"
API_URL="http://localhost:2000"
MODE="${1:-quick}"
PASSED=0
FAILED=0

test_cmd() {
    echo -e "${BLUE}$1${NC}"
    if eval "$2" >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Passed${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ Failed${NC}"
        ((FAILED++))
    fi
    echo
}

echo "======================================"
echo "   CodeRunr CLI Tests ($MODE mode)"
echo "======================================"
echo

# Build if needed
[[ ! -f "$CLI" ]] && go build -o coderunr-cli

# Basic tests
test_cmd "Version command" "$CLI version"
test_cmd "Help command" "$CLI --help"

if [[ "$MODE" != "basic" ]]; then
    test_cmd "List runtimes" "$CLI --url='$API_URL' list"
    
    echo 'print("Hello!")' > /tmp/test.py
    test_cmd "Python execution" "$CLI --url='$API_URL' execute python /tmp/test.py"
    rm -f /tmp/test.py
    
    test_cmd "Package list" "$CLI --url='$API_URL' package list"
fi

if [[ "$MODE" == "full" ]]; then
    cat > /tmp/test.go << 'EOF'
package main
import "fmt"
func main() { fmt.Println("Go works!") }
EOF
    test_cmd "Go execution" "$CLI --url='$API_URL' execute go /tmp/test.go"
    rm -f /tmp/test.go
    
    test_cmd "Verbose mode" "$CLI --verbose version"
fi

echo "======================================"
echo -e "${GREEN}Passed: $PASSED${NC} | ${RED}Failed: $FAILED${NC}"
[[ $FAILED -eq 0 ]] && echo -e "${GREEN}All tests passed! 🎉${NC}" || echo -e "${RED}Some failed 😞${NC}"
exit $FAILED
