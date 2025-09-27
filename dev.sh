#!/bin/bash

# Development script to run both frontend and backend
set -e

echo "Starting development environment..."

# Function to cleanup background processes
cleanup() {
    echo "Stopping development servers..."
    if [ ! -z "$FRONTEND_PID" ]; then
        kill $FRONTEND_PID 2>/dev/null || true
    fi
    if [ ! -z "$BACKEND_PID" ]; then
        kill $BACKEND_PID 2>/dev/null || true
    fi
    exit 0
}

# Set up signal handlers
trap cleanup SIGINT SIGTERM

# Start backend server
echo "Starting Go backend server..."
go run cmd/logviewer/main.go -logs "./logs" -port 8080 &
BACKEND_PID=$!

# Wait a moment for backend to start
sleep 2

# Start frontend development server
echo "Starting Vue.js frontend development server..."
cd frontend
npm run dev &
FRONTEND_PID=$!
cd ..

echo "Development servers started:"
echo "  Backend:  http://localhost:8080"
echo "  Frontend: http://localhost:5173"
echo "  API Proxy: Frontend will proxy API calls to backend"
echo ""
echo "Press Ctrl+C to stop both servers"

# Wait for either process to exit
wait