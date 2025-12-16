#!/bin/bash

# LibreChat Dependency Installation Script
# This script installs all dependencies for the LibreChat workspace

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
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

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBRECHAT_DIR="${SCRIPT_DIR}/InstiLibreChat"

# Check if InstiLibreChat directory exists
if [ ! -d "$LIBRECHAT_DIR" ]; then
    print_error "InstiLibreChat directory not found at: $LIBRECHAT_DIR"
    exit 1
fi

print_info "=========================================="
print_info "LibreChat Dependency Installation Script"
print_info "=========================================="
echo ""

# Step 1: Check prerequisites
print_info "Step 1: Checking prerequisites..."

if command -v node &> /dev/null; then
    NODE_VERSION=$(node --version)
    print_success "Node.js $NODE_VERSION found"
else
    print_error "Node.js not found. Please install Node.js first."
    exit 1
fi

if command -v npm &> /dev/null; then
    NPM_VERSION=$(npm --version)
    print_success "npm $NPM_VERSION found"
else
    print_error "npm not found. Please install npm first."
    exit 1
fi

echo ""

# Step 2: Navigate to InstiLibreChat directory
print_info "Step 2: Navigating to InstiLibreChat directory..."
cd "$LIBRECHAT_DIR"
print_success "Current directory: $(pwd)"
echo ""

# Step 3: Install root dependencies
print_info "Step 3: Installing root workspace dependencies..."
print_info "This may take several minutes..."

# Configure npm for better reliability
npm config set fetch-retry-maxtimeout 600000 || true
npm config set fetch-retries 5 || true
npm config set fetch-retry-mintimeout 15000 || true

# Use npm ci if package-lock.json exists, otherwise npm install
if [ -f "package-lock.json" ]; then
    print_info "Using npm ci for clean, reproducible install..."
    if npm ci --no-audit; then
        print_success "Root dependencies installed successfully"
    else
        print_warning "npm ci failed, falling back to npm install..."
        npm install
        print_success "Root dependencies installed successfully"
    fi
else
    print_info "package-lock.json not found, using npm install..."
    if npm install; then
        print_success "Root dependencies installed successfully"
    else
        print_error "Failed to install root dependencies"
        exit 1
    fi
fi

echo ""

# Step 4: Install dependencies for each workspace package
print_info "Step 4: Installing dependencies for workspace packages..."
print_info "Note: Workspace dependencies are usually handled by root npm install"
print_info "This step ensures all packages have their dependencies installed..."

WORKSPACES=(
    "packages/data-provider"
    "packages/data-schemas"
    "packages/api"
    "packages/client"
    "client"
    "api"
)

for workspace in "${WORKSPACES[@]}"; do
    if [ -d "$workspace" ] && [ -f "$workspace/package.json" ]; then
        print_info "Checking dependencies for $workspace..."
        cd "$workspace"
        
        # Only install if node_modules doesn't exist or is incomplete
        if [ ! -d "node_modules" ] || [ ! -f "node_modules/.package-lock.json" ]; then
            if npm install; then
                print_success "Dependencies verified for $workspace"
            else
                print_warning "Failed to install dependencies for $workspace (continuing...)"
            fi
        else
            print_success "Dependencies already installed for $workspace"
        fi
        
        cd "$LIBRECHAT_DIR"
    else
        print_warning "Skipping $workspace (directory or package.json not found)"
    fi
done

echo ""

# Step 5: Build packages (optional, but recommended)
print_info "Step 5: Building workspace packages..."
print_info "This step is optional but recommended for development"
echo ""

# Build data-provider
print_info "Building librechat-data-provider..."
set +e  # Don't exit on error for builds
if npm run build:data-provider 2>&1 | tee /tmp/build-data-provider.log; then
    print_success "librechat-data-provider built successfully"
else
    print_warning "Failed to build librechat-data-provider"
    print_info "Check /tmp/build-data-provider.log for details"
fi
set -e
echo ""

# Build data-schemas
print_info "Building @librechat/data-schemas..."
set +e
if npm run build:data-schemas 2>&1 | tee /tmp/build-data-schemas.log; then
    print_success "@librechat/data-schemas built successfully"
else
    print_warning "Failed to build @librechat/data-schemas"
    print_info "Check /tmp/build-data-schemas.log for details"
fi
set -e
echo ""

# Build client package
print_info "Building @librechat/client..."
set +e
if npm run build:client-package 2>&1 | tee /tmp/build-client-package.log; then
    print_success "@librechat/client built successfully"
else
    print_warning "Failed to build @librechat/client"
    print_info "Check /tmp/build-client-package.log for details"
fi
set -e
echo ""

# Build API package
print_info "Building @librechat/api..."
set +e
if npm run build:api 2>&1 | tee /tmp/build-api.log; then
    print_success "@librechat/api built successfully"
    
    # Check if memory module exports are in the bundle (rollup bundles everything into index.js)
    if [ -f "packages/api/dist/index.js" ]; then
        if grep -q "loadMemoryConfig\|isMemoryEnabled" packages/api/dist/index.js 2>/dev/null; then
            print_success "Memory module exports verified in bundle"
        else
            print_warning "Memory module exports not found in bundle"
            print_info "This might cause runtime issues with memory features"
        fi
    else
        print_warning "packages/api/dist/index.js not found after build"
    fi
else
    print_error "Failed to build @librechat/api"
    print_info "Check /tmp/build-api.log for details"
    print_info "Common issues:"
    print_info "1. Missing dependencies - ensure data-provider and data-schemas are built first"
    print_info "2. TypeScript/rollup errors - check the log file above"
    print_info "3. Memory module path alias issues - check rollup.config.js"
    exit 1
fi
set -e
echo ""

# Step 6: Summary
print_info "=========================================="
print_info "Installation Summary"
print_info "=========================================="
print_success "Dependency installation completed!"
echo ""

print_info "Next steps:"
print_info "1. If you need to fix the API build, check packages/api/src/memory/"
print_info "2. Run 'npm run backend' to start the backend server"
print_info "3. Run 'npm run frontend:dev' to start the frontend dev server"
echo ""

print_success "All done! 🎉"

