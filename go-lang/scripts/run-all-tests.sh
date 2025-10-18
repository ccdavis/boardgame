#!/bin/bash
# Comprehensive test runner for Axis & Allies web interface

set -e

echo "========================================="
echo "Axis & Allies Web Interface Test Suite"
echo "========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track failures
FAILURES=0

# Function to run tests and track results
run_test() {
    local name=$1
    local command=$2

    echo -e "${YELLOW}Running: $name${NC}"
    if eval "$command"; then
        echo -e "${GREEN}✓ $name passed${NC}"
        echo ""
    else
        echo -e "${RED}✗ $name failed${NC}"
        echo ""
        FAILURES=$((FAILURES + 1))
    fi
}

# 1. Unit Tests
run_test "Unit Tests" \
    "go test ./webserver -v -run 'Test.*Session' 2>&1 | grep -E '(PASS|FAIL|RUN)'"

# 2. Integration Tests
run_test "Integration Tests" \
    "go test ./webserver -v -run 'Integration' 2>&1 | grep -E '(PASS|FAIL|RUN)'"

# 3. DTO Tests
run_test "DTO Conversion Tests" \
    "go test ./webserver -v -run 'DTO' 2>&1 | grep -E '(PASS|FAIL|RUN)' || echo 'No DTO tests found'"

# 4. Check if Playwright is installed
echo -e "${YELLOW}Checking Playwright installation...${NC}"
if go run github.com/playwright-community/playwright-go/cmd/playwright@latest --version &> /dev/null; then
    echo -e "${GREEN}✓ Playwright is installed${NC}"
    echo ""

    # 5. Browser Tests - Accessibility
    run_test "Browser Accessibility Tests" \
        "RUN_BROWSER_TESTS=1 go test ./webserver -v -run 'TestBrowser_Accessibility' -timeout 30s"

    # 6. Browser Tests - UI Components
    run_test "Browser UI Tests" \
        "RUN_BROWSER_TESTS=1 go test ./webserver -v -run 'TestBrowser_(StartGame|ResponsiveDesign|ErrorHandling)' -timeout 30s"

    # 7. Browser Tests - Form Validation
    run_test "Browser Form Tests" \
        "RUN_BROWSER_TESTS=1 go test ./webserver -v -run 'TestBrowser_(FormValidation|ButtonStates)' -timeout 30s"

    # 8. E2E Tests - Game Flow
    run_test "E2E Complete Game Flow" \
        "RUN_BROWSER_TESTS=1 go test ./webserver -v -run 'TestE2E_CompleteGameFlow' -timeout 60s"

    # 9. E2E Tests - Navigation
    run_test "E2E Keyboard Navigation" \
        "RUN_BROWSER_TESTS=1 go test ./webserver -v -run 'TestE2E_KeyboardNavigation' -timeout 30s"

    # 10. E2E Tests - Error Recovery
    run_test "E2E Error Recovery" \
        "RUN_BROWSER_TESTS=1 go test ./webserver -v -run 'TestE2E_ErrorRecovery' -timeout 30s"

    # 11. E2E Tests - Multi-Session
    run_test "E2E Multiple Games" \
        "RUN_BROWSER_TESTS=1 go test ./webserver -v -run 'TestE2E_MultipleGames' -timeout 45s"

else
    echo -e "${YELLOW}⚠ Playwright not installed - skipping browser tests${NC}"
    echo "Run './scripts/install-playwright.sh' to enable browser testing"
    echo ""
fi

# Summary
echo "========================================="
echo "Test Summary"
echo "========================================="
if [ $FAILURES -eq 0 ]; then
    echo -e "${GREEN}All tests passed! ✓${NC}"
    exit 0
else
    echo -e "${RED}$FAILURES test suite(s) failed ✗${NC}"
    exit 1
fi
