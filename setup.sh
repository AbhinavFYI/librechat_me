#!/bin/bash

# LibreChat Setup Script
# This script automates the setup and build process for LibreChat after cloning

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

# Function to run command with error handling
run_command() {
    local cmd="$1"
    local description="$2"
    local optional="${3:-false}"
    
    print_info "$description"
    if eval "$cmd"; then
        print_success "$description completed"
        return 0
    else
        if [ "$optional" = "true" ]; then
            print_warning "$description failed (optional step)"
            return 1
        else
            print_error "$description failed"
            return 1
        fi
    fi
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
cd "$INSTILIBRECHAT_DIR" || {
    print_error "Failed to navigate to InstiLibreChat directory"
    exit 1
}
print_success "Current directory: $(pwd)"
echo ""

# Step 1: Install dependencies
print_info "Step 1: Installing dependencies..."
print_info "This may take several minutes..."
print_info "Installing root dependencies and all workspace packages..."

# Clean install to ensure everything is fresh
if ! run_command "npm install" "Installing dependencies"; then
    print_error "Failed to install dependencies"
    print_info "Try running manually: cd InstiLibreChat && npm install"
    print_info "If you continue to have issues, try: npm cache clean --force && npm install"
    exit 1
fi
echo ""

# Verify node_modules exist
if [ ! -d "node_modules" ]; then
    print_error "node_modules directory not found after installation"
    exit 1
fi
print_success "Dependencies installed successfully"
echo ""

# Step 2: Build required packages in correct order
print_info "Step 2: Building required packages..."
print_info "Building packages in dependency order:"
print_info "  1. data-provider (no dependencies)"
print_info "  2. data-schemas (depends on data-provider)"
print_info "  3. api (depends on data-provider, data-schemas)"
print_info "  4. client-package (independent)"
echo ""

# Build data-provider first (no dependencies)
if ! run_command "npm run build:data-provider" "Building librechat-data-provider"; then
    print_error "Failed to build librechat-data-provider"
    print_info "This is a critical package. Please check the error above."
    exit 1
fi

# Verify data-provider was built
if [ ! -d "packages/data-provider/dist" ]; then
    print_error "data-provider dist directory not found after build"
    exit 1
fi
echo ""

# Build data-schemas (depends on data-provider)
if ! run_command "npm run build:data-schemas" "Building @librechat/data-schemas"; then
    print_error "Failed to build @librechat/data-schemas"
    print_info "This package depends on data-provider. Make sure data-provider built successfully."
    exit 1
fi

# Verify data-schemas was built
if [ ! -d "packages/data-schemas/dist" ]; then
    print_error "data-schemas dist directory not found after build"
    exit 1
fi
echo ""

# Build API package (depends on data-provider and data-schemas)
if ! run_command "npm run build:api" "Building @librechat/api"; then
    print_error "Failed to build @librechat/api"
    print_info "This package depends on data-provider and data-schemas."
    exit 1
fi

# Verify API was built and api/server/index.js exists
if [ ! -f "api/server/index.js" ]; then
    print_error "api/server/index.js not found after build"
    print_info "The API build may have failed. Check the error messages above."
    exit 1
fi
echo ""

# Build client package (independent)
if ! run_command "npm run build:client-package" "Building @librechat/client package"; then
    print_error "Failed to build @librechat/client package"
    exit 1
fi

# Verify client package was built
if [ ! -d "packages/client/dist" ]; then
    print_error "client package dist directory not found after build"
    exit 1
fi
echo ""

print_success "All required packages built successfully!"
echo ""

# Step 3: Build client application (optional, but recommended)
print_info "Step 3: Building client application..."
print_info "This may take a few minutes..."

if run_command "npm run build:client" "Building client application" "true"; then
    # Verify client was built
    if [ -d "client/dist" ]; then
        print_success "Client application built successfully"
    else
        print_warning "Client dist directory not found, but build reported success"
    fi
else
    print_warning "Client build failed, but packages are built"
    print_info "You can still run the backend, but frontend may not work"
    print_info "Try running manually: cd InstiLibreChat && npm run build:client"
fi
echo ""

# Final verification
print_info "Verifying build artifacts..."
MISSING_FILES=0

if [ ! -d "packages/data-provider/dist" ]; then
    print_error "Missing: packages/data-provider/dist"
    MISSING_FILES=$((MISSING_FILES + 1))
fi

if [ ! -d "packages/data-schemas/dist" ]; then
    print_error "Missing: packages/data-schemas/dist"
    MISSING_FILES=$((MISSING_FILES + 1))
fi

if [ ! -f "api/server/index.js" ]; then
    print_error "Missing: api/server/index.js"
    MISSING_FILES=$((MISSING_FILES + 1))
fi

if [ ! -d "packages/client/dist" ]; then
    print_error "Missing: packages/client/dist"
    MISSING_FILES=$((MISSING_FILES + 1))
fi

if [ $MISSING_FILES -eq 0 ]; then
    print_success "All critical build artifacts verified"
else
    print_warning "$MISSING_FILES critical build artifact(s) missing"
fi
echo ""

# Summary
print_success "=========================================="
print_success "Setup completed!"
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
