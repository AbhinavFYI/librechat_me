#!/bin/bash

# LibreChat Setup Script
# This script automates the setup and build process for LibreChat after cloning

# Don't exit on error immediately - we want to show helpful messages
set +e

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

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_ROOT="$SCRIPT_DIR"
INSTILIBRECHAT_DIR="$REPO_ROOT/InstiLibreChat"

print_info "LibreChat Setup Script"
print_info "======================"
echo ""

# Check if InstiLibreChat directory exists
if [ ! -d "$INSTILIBRECHAT_DIR" ]; then
    print_error "InstiLibreChat directory not found at: $INSTILIBRECHAT_DIR"
    print_info "Please make sure you're running this script from the repository root"
    exit 1
fi

# Check for Node.js
print_info "Checking prerequisites..."
if ! command_exists node; then
    print_error "Node.js is not installed. Please install Node.js v18 or higher."
    print_info "Visit: https://nodejs.org/"
    exit 1
fi

# Get Node.js version more robustly
NODE_VERSION_STRING=$(node -v)
NODE_VERSION_MAJOR=$(echo "$NODE_VERSION_STRING" | sed 's/v//' | cut -d'.' -f1)

# Check if NODE_VERSION_MAJOR is a valid number
if ! [[ "$NODE_VERSION_MAJOR" =~ ^[0-9]+$ ]]; then
    print_warning "Could not parse Node.js version: $NODE_VERSION_STRING"
    print_warning "Continuing anyway, but Node.js v18+ is recommended"
else
    if [ "$NODE_VERSION_MAJOR" -lt 18 ]; then
        print_warning "Node.js version $NODE_VERSION_STRING is less than 18. Recommended: Node.js v18 or higher"
        print_warning "The build may fail with older versions"
    else
        print_success "Node.js $NODE_VERSION_STRING found"
    fi
fi

# Check for npm
if ! command_exists npm; then
    print_error "npm is not installed. Please install npm."
    exit 1
fi
print_success "npm $(npm -v) found"
echo ""

# Navigate to InstiLibreChat directory
print_info "Navigating to InstiLibreChat directory..."
cd "$INSTILIBRECHAT_DIR"
print_success "Current directory: $(pwd)"
echo ""

# Step 1: Install dependencies
print_info "Step 1: Installing dependencies..."
print_info "This may take several minutes..."
set -e  # Enable exit on error for actual commands
npm install
if [ $? -eq 0 ]; then
    print_success "Dependencies installed successfully"
else
    print_error "Failed to install dependencies"
    print_info "Try running: cd InstiLibreChat && npm install"
    exit 1
fi
echo ""

# Step 2: Build required packages
print_info "Step 2: Building required packages..."
print_info "Building all packages individually for first-time setup..."
echo ""

# Build data-schemas
print_info "Building @librechat/data-schemas..."
npm run build:data-schemas
if [ $? -eq 0 ]; then
    print_success "@librechat/data-schemas built successfully"
else
    print_error "Failed to build @librechat/data-schemas"
    exit 1
fi
echo ""

# Build data-provider
print_info "Building librechat-data-provider..."
npm run build:data-provider
if [ $? -eq 0 ]; then
    print_success "librechat-data-provider built successfully"
else
    print_error "Failed to build librechat-data-provider"
    exit 1
fi
echo ""

# Build API package
print_info "Building @librechat/api..."
npm run build:api
if [ $? -eq 0 ]; then
    print_success "@librechat/api built successfully"
else
    print_error "Failed to build @librechat/api"
    exit 1
fi
echo ""

# Build client package
print_info "Building @librechat/client package..."
npm run build:client-package
if [ $? -eq 0 ]; then
    print_success "@librechat/client package built successfully"
else
    print_error "Failed to build @librechat/client package"
    exit 1
fi
echo ""

print_success "All required packages built successfully!"
echo ""

# Step 3: Build client (optional, but recommended)
print_info "Step 3: Building client application..."
print_info "This may take a few minutes..."
set +e  # Don't exit on error for client build (it's optional)
npm run build:client
if [ $? -eq 0 ]; then
    print_success "Client built successfully"
else
    print_warning "Client build failed, but packages are built"
    print_info "You can still run the backend, but frontend may not work"
    print_info "Try running: cd InstiLibreChat && npm run build:client"
fi
set -e  # Re-enable exit on error
echo ""

# Summary
print_success "=========================================="
print_success "Setup completed successfully!"
print_success "=========================================="
echo ""
print_info "Next steps:"
echo "  1. Create a .env file in the InstiLibreChat directory"
echo "  2. Configure your environment variables (MONGO_URI, etc.)"
echo "  3. Start the backend with: npm run backend"
echo "  4. Or start in development mode: npm run backend:dev"
echo ""
print_info "For frontend development, run: npm run frontend:dev"
echo ""

