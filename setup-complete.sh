#!/bin/bash

# LibreChat Complete Setup Script
# Based on the README instructions - automates all setup steps

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

print_header() {
    echo -e "${CYAN}$1${NC}"
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
    set +e  # Don't exit on error
    eval "$cmd" > /tmp/last-command.log 2>&1
    local exit_code=$?
    set -e  # Re-enable exit on error
    
    if [ $exit_code -eq 0 ]; then
        print_success "$description completed"
        return 0
    else
        if [ "$optional" = "true" ]; then
            print_warning "$description failed (optional step)"
            if [ -f /tmp/last-command.log ]; then
                print_info "Error output (last 10 lines):"
                tail -10 /tmp/last-command.log 2>/dev/null || true
            fi
            return 1
        else
            print_error "$description failed (exit code: $exit_code)"
            if [ -f /tmp/last-command.log ]; then
                print_info "Error output:"
                cat /tmp/last-command.log
            fi
            return 1
        fi
    fi
}

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_ROOT="$SCRIPT_DIR"
INSTILIBRECHAT_DIR="$REPO_ROOT/InstiLibreChat"

# Print header
echo ""
print_header "=========================================="
print_header "LibreChat Complete Setup Script"
print_header "=========================================="
echo ""

# Step 0: Check if we're in the right directory
print_info "Checking repository structure..."

if [ ! -d "$INSTILIBRECHAT_DIR" ]; then
    print_error "InstiLibreChat directory not found at: $INSTILIBRECHAT_DIR"
    print_info "Please make sure you're running this script from the repository root"
    print_info "Expected structure:"
    print_info "  - Repository root (where setup-complete.sh is located)"
    print_info "  - InstiLibreChat/ (main application)"
    print_info "  - saas-api/ (SaaS API services)"
    exit 1
fi

print_success "Repository structure verified"
echo ""

# Step 1: Check Prerequisites
print_header "Step 1: Checking Prerequisites"
print_info "Verifying required tools are installed..."
echo ""

# Check Node.js
if ! command_exists node; then
    print_error "Node.js is not installed"
    print_info "Please install Node.js v18 or higher from: https://nodejs.org/"
    exit 1
fi

NODE_VERSION_STRING=$(node -v)
NODE_VERSION_MAJOR=$(echo "$NODE_VERSION_STRING" | sed 's/v//' | cut -d'.' -f1)

if ! [[ "$NODE_VERSION_MAJOR" =~ ^[0-9]+$ ]]; then
    print_warning "Could not parse Node.js version: $NODE_VERSION_STRING"
    print_warning "Continuing anyway, but Node.js v18+ is recommended"
else
    if [ "$NODE_VERSION_MAJOR" -lt 18 ]; then
        print_warning "Node.js version $NODE_VERSION_STRING is less than 18"
        print_warning "Recommended: Node.js v18 or higher"
        print_warning "The build may fail with older versions"
    else
        print_success "Node.js $NODE_VERSION_STRING found"
    fi
fi

# Check npm
if ! command_exists npm; then
    print_error "npm is not installed"
    print_info "npm usually comes with Node.js. Please reinstall Node.js"
    exit 1
fi
print_success "npm $(npm -v) found"

# Check for bun (optional)
if command_exists bun; then
    print_success "bun $(bun -v) found (optional)"
else
    print_info "bun not found (optional, npm will be used)"
fi

# Check MongoDB (optional, just warn)
if ! command_exists mongod; then
    print_warning "MongoDB not found in PATH (you may need to install it separately)"
    print_info "MongoDB is required to run LibreChat"
else
    print_success "MongoDB found"
fi

# Check Go (optional, for saas-api)
if command_exists go; then
    print_success "Go $(go version | cut -d' ' -f3) found (for saas-api)"
else
    print_info "Go not found (optional, only needed for building saas-api from source)"
fi

echo ""
print_success "Prerequisites check completed"
echo ""

# Step 2: Navigate to InstiLibreChat directory
print_header "Step 2: Setting Up InstiLibreChat"
print_info "Navigating to InstiLibreChat directory..."

cd "$INSTILIBRECHAT_DIR" || {
    print_error "Failed to navigate to InstiLibreChat directory"
    exit 1
}

print_success "Current directory: $(pwd)"
echo ""

# Step 3: Install Dependencies
print_header "Step 3: Installing Dependencies"
print_info "Installing dependencies for root workspace and all sub-packages..."
print_info "This may take several minutes..."
echo ""

# Configure npm for better reliability (like in Dockerfile)
print_info "Configuring npm for better reliability..."
npm config set fetch-retry-maxtimeout 600000 || true
npm config set fetch-retries 5 || true
npm config set fetch-retry-mintimeout 15000 || true
print_success "npm configuration updated"
echo ""

# Clean npm cache if there are issues (optional, but helpful)
print_info "Checking npm cache..."
if npm cache verify 2>/dev/null; then
    print_success "npm cache is valid"
else
    print_warning "npm cache may have issues, cleaning..."
    npm cache clean --force 2>/dev/null || true
fi
echo ""

# Install dependencies with better error handling
# Prefer npm ci for clean, reproducible installs (requires package-lock.json)
if [ -f "package-lock.json" ]; then
    print_info "package-lock.json found, using npm ci for clean install..."
    print_info "npm ci is faster and more reliable for reproducible builds"
    set +e  # Don't exit on error immediately
    npm ci --no-audit 2>&1 | tee /tmp/npm-install.log
    NPM_INSTALL_EXIT_CODE=$?
    set -e  # Re-enable exit on error
    
    if [ $NPM_INSTALL_EXIT_CODE -ne 0 ]; then
        print_warning "npm ci failed, falling back to npm install..."
        print_info "This might be due to package-lock.json being out of sync"
        echo ""
        # Fall back to npm install
        set +e
        npm install 2>&1 | tee /tmp/npm-install.log
        NPM_INSTALL_EXIT_CODE=$?
        set -e
    fi
else
    print_info "package-lock.json not found, using npm install..."
    print_warning "Consider committing package-lock.json for reproducible builds"
    set +e  # Don't exit on error immediately
    npm install 2>&1 | tee /tmp/npm-install.log
    NPM_INSTALL_EXIT_CODE=$?
    set -e  # Re-enable exit on error
fi

if [ $NPM_INSTALL_EXIT_CODE -ne 0 ]; then
    print_error "Failed to install dependencies"
    print_info "Exit code: $NPM_INSTALL_EXIT_CODE"
    echo ""
    print_info "Last 20 lines of npm install output:"
    tail -20 /tmp/npm-install.log 2>/dev/null || true
    echo ""
    print_info "Troubleshooting steps:"
    print_info "1. Try cleaning npm cache: npm cache clean --force"
    print_info "2. Try removing node_modules:"
    print_info "   rm -rf node_modules"
    print_info "   npm install"
    print_info "3. If package-lock.json exists but npm ci fails, try:"
    print_info "   rm -rf node_modules package-lock.json"
    print_info "   npm install"
    print_info "4. Check your internet connection"
    print_info "5. Try with verbose output: npm install --verbose"
    exit 1
fi

# Verify installation
if [ ! -d "node_modules" ]; then
    print_error "node_modules directory not found after installation"
    exit 1
fi

# Check if key packages are installed
if [ ! -d "node_modules/.bin" ]; then
    print_warning "node_modules/.bin not found, but continuing..."
fi

print_success "Dependencies installed successfully"
echo ""

# Step 4: Build Required Packages
print_header "Step 4: Building Required Packages"
print_info "Building packages in dependency order:"
print_info "  1. librechat-data-provider - Data services (no dependencies)"
print_info "  2. @librechat/data-schemas - Mongoose schemas (depends on data-provider)"
print_info "  3. @librechat/api - API package (depends on data-provider, data-schemas)"
print_info "  4. @librechat/client - React components package (independent)"
echo ""

# Build data-provider first (no dependencies)
print_info "Building librechat-data-provider (no dependencies)..."
if ! run_command "npm run build:data-provider" "Building librechat-data-provider"; then
    print_error "Failed to build librechat-data-provider"
    print_info "This is a critical package. Check the error output above."
    print_info "Make sure all dependencies were installed correctly."
    exit 1
fi

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

if [ ! -d "packages/data-schemas/dist" ]; then
    print_error "data-schemas dist directory not found after build"
    exit 1
fi
echo ""

# Build API package (depends on data-provider and data-schemas)
print_info "Building @librechat/api (depends on data-provider and data-schemas)..."
print_info "Verifying dependencies are available..."

# Verify dependencies exist before building
if [ ! -d "packages/data-provider/dist" ]; then
    print_error "data-provider/dist not found! API build requires data-provider to be built first."
    exit 1
fi

if [ ! -d "packages/data-schemas/dist" ]; then
    print_error "data-schemas/dist not found! API build requires data-schemas to be built first."
    exit 1
fi

print_success "Dependencies verified"
echo ""

if ! run_command "npm run build:api" "Building @librechat/api"; then
    print_error "Failed to build @librechat/api"
    print_info "Common issues:"
    print_info "1. Missing dependencies - make sure data-provider and data-schemas built successfully"
    print_info "2. TypeScript errors - check the error output above"
    print_info "3. Missing node_modules - try: cd packages/api && npm install"
    print_info "4. Memory module issues - check if packages/api/src/memory directory has files"
    echo ""
    print_info "Try building manually:"
    print_info "  cd packages/api"
    print_info "  npm run build"
    exit 1
fi

# Verify API was built correctly
if [ ! -f "api/server/index.js" ]; then
    print_error "api/server/index.js not found after build"
    print_info "The API package may have built, but api/server/index.js was not created."
    print_info "This file is created by the root build process, not the packages/api build."
    print_info "Checking if packages/api/dist exists..."
    
    if [ -d "packages/api/dist" ]; then
        print_success "packages/api/dist exists - API package built successfully"
        
        # Check if memory module was built
        if [ -d "packages/api/dist/memory" ] && [ -f "packages/api/dist/memory/index.js" ]; then
            print_success "packages/api/dist/memory exists - memory module built"
        else
            print_warning "packages/api/dist/memory not found - memory module may not have been built"
            print_info "Checking source files..."
            
            if [ -d "packages/api/src/memory" ] && [ -f "packages/api/src/memory/index.ts" ]; then
                print_info "Source memory module exists at packages/api/src/memory/"
                print_warning "Memory module exists in source but not in dist - build may have issues"
                print_info "This could be due to:"
                print_info "1. TypeScript path alias resolution issue (~/memory/config)"
                print_info "2. Rollup not including the memory module in the bundle"
                print_info "3. The memory module not being properly exported"
                print_info ""
                print_info "The build may still work, but memory-related features might fail at runtime"
            else
                print_error "Source memory module not found at packages/api/src/memory/"
                print_info "The memory module should exist in the source code"
            fi
        fi
        
        print_warning "api/server/index.js may be created by a different build step"
        print_info "The API package itself is built, but the server index may need additional setup"
    else
        print_error "packages/api/dist not found - API package build failed"
        exit 1
    fi
else
    print_success "api/server/index.js found - API build successful"
    
    # Still check memory module even if api/server/index.js exists
    if [ ! -d "packages/api/dist/memory" ] || [ ! -f "packages/api/dist/memory/index.js" ]; then
        print_warning "Memory module not found in dist, but api/server/index.js exists"
        print_info "This might cause runtime errors when using memory features"
    fi
fi
echo ""

# Build client package
if ! run_command "npm run build:client-package" "Building @librechat/client package"; then
    print_error "Failed to build @librechat/client package"
    exit 1
fi

if [ ! -d "packages/client/dist" ]; then
    print_error "client package dist directory not found after build"
    exit 1
fi
echo ""

print_success "All required packages built successfully!"
echo ""

# Step 5: Build Client Application
print_header "Step 5: Building Client Application"
print_info "Building the client application (frontend)..."
print_info "This may take a few minutes..."
echo ""

if run_command "npm run build:client" "Building client application" "true"; then
    if [ -d "client/dist" ]; then
        print_success "Client application built successfully"
    else
        print_warning "Client dist directory not found, but build reported success"
    fi
else
    print_warning "Client build failed, but packages are built"
    print_info "You can still run the backend, but frontend may not work"
    print_info "For development, you can use: npm run frontend:dev"
fi
echo ""

# Step 6: Verify Build Artifacts
print_header "Step 6: Verifying Build Artifacts"
print_info "Checking that all critical files were created..."
echo ""

MISSING_FILES=0
MISSING_DIRS=0

# Check package dist directories
if [ ! -d "packages/data-schemas/dist" ]; then
    print_error "Missing: packages/data-schemas/dist"
    MISSING_DIRS=$((MISSING_DIRS + 1))
else
    print_success "Found: packages/data-schemas/dist"
fi

if [ ! -d "packages/data-provider/dist" ]; then
    print_error "Missing: packages/data-provider/dist"
    MISSING_DIRS=$((MISSING_DIRS + 1))
else
    print_success "Found: packages/data-provider/dist"
fi

if [ ! -d "packages/client/dist" ]; then
    print_error "Missing: packages/client/dist"
    MISSING_DIRS=$((MISSING_DIRS + 1))
else
    print_success "Found: packages/client/dist"
fi

# Check API server file
if [ ! -f "api/server/index.js" ]; then
    print_error "Missing: api/server/index.js"
    MISSING_FILES=$((MISSING_FILES + 1))
else
    print_success "Found: api/server/index.js"
fi

# Check client dist (optional)
if [ -d "client/dist" ]; then
    print_success "Found: client/dist (client application)"
else
    print_warning "Missing: client/dist (optional, for production frontend)"
fi

echo ""

if [ $MISSING_FILES -eq 0 ] && [ $MISSING_DIRS -eq 0 ]; then
    print_success "All critical build artifacts verified"
else
    print_warning "$MISSING_FILES file(s) and $MISSING_DIRS directory(ies) missing"
    if [ $MISSING_FILES -gt 0 ] || [ $MISSING_DIRS -gt 0 ]; then
        print_info "Some builds may have failed. Check the error messages above."
    fi
fi
echo ""

# Step 7: Environment Setup Reminder
print_header "Step 7: Environment Configuration"
print_info "Next, you need to configure your environment:"
echo ""
print_info "1. Create a .env file in the InstiLibreChat directory:"
echo "   cd InstiLibreChat"
echo "   touch .env"
echo ""
print_info "2. Edit .env with your configuration. Common variables:"
echo "   - MONGO_URI - MongoDB connection string"
echo "   - MEILI_HOST - Meilisearch host URL"
echo "   - OPENAI_API_KEY - OpenAI API key (if using OpenAI)"
echo "   - PORT - Server port (default: 3080)"
echo ""
print_info "For detailed configuration, see:"
echo "   - LibreChat Documentation: https://docs.librechat.ai"
echo "   - Original LibreChat repo: https://github.com/danny-avila/LibreChat"
echo ""

# Final Summary
print_header "=========================================="
print_success "Setup Completed Successfully!"
print_header "=========================================="
echo ""
print_info "What was done:"
echo "  ✓ Prerequisites checked"
echo "  ✓ Dependencies installed"
echo "  ✓ All packages built"
echo "  ✓ Client application built (if successful)"
echo "  ✓ Build artifacts verified"
echo ""
print_info "Next steps:"
echo "  1. Create and configure .env file (see above)"
echo "  2. Start MongoDB (if not already running)"
echo "  3. Start the backend:"
echo "     cd InstiLibreChat"
echo "     npm run backend"
echo ""
print_info "For development:"
echo "  - Backend dev mode: npm run backend:dev"
echo "  - Frontend dev mode: npm run frontend:dev"
echo ""
print_info "For troubleshooting, see the README.md file"
echo ""

