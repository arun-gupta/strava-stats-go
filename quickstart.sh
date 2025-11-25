#!/bin/bash

# Start the application in the background
echo "Starting Strava Stats..."
go run cmd/strava-stats/main.go &
SERVER_PID=$!

# Wait for a moment to ensure the server starts
sleep 2

# Open the browser
echo "Opening browser..."
open "http://localhost:8080"

# Wait for the server process to finish (so the script doesn't exit immediately killing the server)
wait $SERVER_PID
