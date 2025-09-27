#!/bin/bash

# Build script for the frontend
set -e

echo "Building frontend..."

# Navigate to frontend directory
cd frontend

# Install dependencies if node_modules doesn't exist
if [ ! -d "node_modules" ]; then
    echo "Installing dependencies..."
    npm install
fi

# Build the frontend
echo "Building Vue.js application..."
npm run build-for-go

echo "Frontend build completed successfully!"
echo "Output directory: web/"

# Navigate back to root
cd ..

# List the contents of web directory
if [ -d "web" ]; then
    echo "Built files:"
    ls -la web/
fi