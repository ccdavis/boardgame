#!/bin/bash
# Install Playwright browsers for testing

set -e

echo "Installing Playwright browsers..."

# Install the browsers using the playwright CLI
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps chromium

echo "Playwright browsers installed successfully!"
echo ""
echo "To run browser tests:"
echo "  RUN_BROWSER_TESTS=1 go test ./webserver -v -run Browser"
