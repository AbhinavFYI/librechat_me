#!/bin/bash

# LibreChat Frontend Build Script
# This script builds only the frontend packages and client

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored messages
print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
INSTILIBRECHAT_DIR="$SCRIPT_DIR/InstiLibreChat"

print_info "LibreChat Frontend Build Script"
print_info "================================="
echo ""

# Check if InstiLibreChat directory exists
if [ ! -d "$INSTILIBRECHAT_DIR" ]; then
    print_error "InstiLibreChat directory not found at: $INSTILIBRECHAT_DIR"
    exit 1
fi

# Navigate to InstiLibreChat directory
cd "$INSTILIBRECHAT_DIR"

# Build all packages
print_info "Building all required packages..."
if npm run build:packages; then
    print_success "All packages built successfully"
else
    print_error "Failed to build packages"
    exit 1
fi
echo ""

# Build client
print_info "Building client application..."
if npm run build:client; then
    print_success "Client built successfully"
else
    print_error "Failed to build client"
    exit 1
fi
echo ""

print_success "Frontend build completed successfully!"
echo ""

