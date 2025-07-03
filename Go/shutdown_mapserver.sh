#!/bin/bash

# Shutdown script for the mapserver
# This script finds and terminates all running mapserver processes

echo "Searching for running mapserver processes..."

# Find all mapserver processes
# Look for both 'mapserver' binary and 'go run cmd/mapserver/main.go' processes
PIDS=$(pgrep -f "mapserver|cmd/mapserver/main.go" | grep -v "shutdown_mapserver.sh")

if [ -z "$PIDS" ]; then
    echo "No mapserver processes found."
    exit 0
fi

echo "Found mapserver process(es) with PID(s): $PIDS"

# Kill the processes
for PID in $PIDS; do
    echo "Terminating process $PID..."
    kill -TERM $PID 2>/dev/null
    
    # Wait a moment for graceful shutdown
    sleep 1
    
    # Check if process is still running
    if kill -0 $PID 2>/dev/null; then
        echo "Process $PID didn't terminate gracefully, forcing shutdown..."
        kill -KILL $PID 2>/dev/null
    else
        echo "Process $PID terminated successfully."
    fi
done

# Final check
REMAINING=$(pgrep -f "mapserver|cmd/mapserver/main.go" | grep -v "shutdown_mapserver.sh")
if [ -z "$REMAINING" ]; then
    echo "All mapserver processes have been terminated."
else
    echo "Warning: Some processes may still be running: $REMAINING"
    exit 1
fi

exit 0