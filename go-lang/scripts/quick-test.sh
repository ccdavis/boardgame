#!/bin/bash
# Quick test runner - runs only unit and integration tests (no browser)

set -e

echo "Running quick tests (unit + integration)..."
echo ""

# Run all non-browser tests
go test ./webserver -v -run 'Test' | grep -v 'Browser\|E2E'

echo ""
echo "✓ Quick tests complete!"
echo ""
echo "For full test suite including browser tests, run:"
echo "  ./scripts/run-all-tests.sh"
