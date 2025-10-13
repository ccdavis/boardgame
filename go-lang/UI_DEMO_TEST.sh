#!/bin/bash

# UI Demo Test Script
# This script demonstrates the tutorial and expert modes

echo "=== Testing Tutorial Mode ==="
echo "This tests the basic UI in tutorial mode"
echo ""

echo -e "help\nmode\nhelp\nquit" | timeout 5 ./boardgame ../aaa.gdf 2>&1 | head -150

echo ""
echo ""
echo "=== Test Complete ==="
echo "The game now supports:"
echo "  - Tutorial mode (default) with detailed explanations"
echo "  - Expert mode with streamlined interface"
echo "  - Movement commands (move, cancel, show)"
echo "  - Combat commands (battles, view, resolve, auto)"
echo "  - Mode toggle with 'mode' command"
echo ""
