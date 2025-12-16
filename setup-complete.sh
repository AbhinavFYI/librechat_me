#!/usr/bin/env bash

# ==========================================
# LibreChat Complete Setup & Build Script
# ==========================================
# This script handles the complete setup from a fresh clone:
# 1. Prerequisites checking
# 2. Dependency installation
# 3. Package builds in correct order
# 4. Verification
# 5. Ready to run frontend and backend
# ==========================================

set -euo pipefail  # Exit on error, but we handle errors gracefully in build functions

# ---------- Colors & Output ----------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()    { echo -e "${BLUE}ℹ${NC} $1"; }
success() { echo -e "${GREEN}✓${NC} $1"; }
warn()    { echo -e "${YELLOW}⚠${NC} $1"; }
error()   { echo -e "${RED}✗${NC} $1"; exit 1; }
step()    { echo -e "\n${CYAN}${BOLD}▶${NC} ${BOLD}$1${NC}"; }
substep() { echo -e "  ${BLUE}→${NC} $1"; }

# ---------- Paths ----------
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBRECHAT_DIR="$ROOT_DIR/InstiLibreChat"

# ---------- Main Execution ----------
main() {
    echo -e "${BOLD}${CYAN}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║     LibreChat Complete Setup & Build Script              ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    
    # Step 1: Verify project structure
    step "Step 1: Verifying project structure"
    verify_project_structure
    
    # Step 2: Check prerequisites
    step "Step 2: Checking prerequisites"
    check_prerequisites
    
    # Step 3: Clean previous builds
    step "Step 3: Cleaning previous builds"
    clean_builds
    
    # Step 4: Install dependencies
    step "Step 4: Installing dependencies"
    install_dependencies
    
    # Step 5: Build packages in order
    step "Step 5: Building workspace packages"
    build_packages
    
    # Step 6: Build frontend
    step "Step 6: Building frontend application"
    build_frontend
    
    # Step 7: Verify installation
    step "Step 7: Verifying installation"
    verify_installation
    
    # Step 8: Final summary
    step "Step 8: Setup complete!"
    show_summary
}

# ---------- Functions ----------

verify_project_structure() {
    if [ ! -d "$LIBRECHAT_DIR" ]; then
        error "InstiLibreChat directory not found at: $LIBRECHAT_DIR"
    fi
    
    if [ ! -f "$LIBRECHAT_DIR/package.json" ]; then
        error "package.json not found in InstiLibreChat directory"
    fi
    
    success "Project structure verified"
    substep "Root directory: $ROOT_DIR"
    substep "LibreChat directory: $LIBRECHAT_DIR"
}

check_prerequisites() {
    # Check Node.js
    if ! command -v node >/dev/null 2>&1; then
        error "Node.js is not installed"
        echo ""
        info "Please install Node.js v18 or higher:"
        info "  • macOS (Homebrew): brew install node"
        info "  • macOS (nvm): curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash"
        info "  • Ubuntu/Debian: sudo apt update && sudo apt install nodejs npm"
        info "  • Windows: Download from https://nodejs.org/"
        info "  • Or visit: https://nodejs.org/"
        exit 1
    fi
    
    NODE_VERSION=$(node -v)
    NPM_VERSION=$(npm -v)
    NODE_MAJOR=$(echo "$NODE_VERSION" | sed 's/v\([0-9]*\).*/\1/')
    
    if [ "$NODE_MAJOR" -lt 18 ]; then
        warn "Node.js version $NODE_VERSION detected (v18+ recommended)"
        echo ""
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            info "Please upgrade Node.js to v18 or higher"
            exit 1
        fi
    else
        success "Node.js $NODE_VERSION found"
    fi
    
    # Check npm
    if ! command -v npm >/dev/null 2>&1; then
        error "npm is not installed (should come with Node.js)"
    fi
    success "npm $NPM_VERSION found"
    
    # Check MongoDB (optional but recommended)
    if command -v mongod >/dev/null 2>&1; then
        success "MongoDB found (mongod command available)"
    else
        warn "MongoDB not found in PATH (backend requires MongoDB)"
        info "Install MongoDB: https://www.mongodb.com/try/download/community"
    fi
    
    echo ""
}

clean_builds() {
    cd "$LIBRECHAT_DIR"
    
    substep "Removing node_modules directories..."
    rm -rf node_modules 2>/dev/null || true
    rm -rf packages/*/node_modules 2>/dev/null || true
    rm -rf packages/*/dist 2>/dev/null || true
    rm -rf client/node_modules 2>/dev/null || true
    rm -rf client/dist 2>/dev/null || true
    rm -rf api/node_modules 2>/dev/null || true
    rm -rf api/dist 2>/dev/null || true
    
    success "Previous builds cleaned"
    echo ""
}

install_dependencies() {
    cd "$LIBRECHAT_DIR"
    
    # Check and install rimraf globally if needed (fixes clean errors in data-schemas)
    substep "Checking for rimraf (required for package builds)..."
    if ! command -v rimraf >/dev/null 2>&1; then
        info "rimraf not found. Installing globally..."
        npm install -g rimraf --no-audit || warn "Failed to install rimraf globally (may cause build issues)"
    else
        success "rimraf found"
    fi
    
    substep "Installing root workspace dependencies (this may take several minutes)..."
    info "Using npm workspaces to install all dependencies"
    
    if npm install --no-audit --legacy-peer-deps; then
        success "Root dependencies installed successfully"
    else
        error "Failed to install root dependencies"
    fi
    
    # Install dependencies in client directory explicitly
    substep "Installing client workspace dependencies..."
    if [ -d "client" ]; then
        cd client
        if npm install --no-audit --legacy-peer-deps; then
            success "Client dependencies installed"
        else
            warn "Client dependency installation had issues (continuing anyway)"
        fi
        cd "$LIBRECHAT_DIR"
    else
        warn "client directory not found"
    fi
    
    # Verify critical dependencies
    substep "Verifying critical dependencies..."
    CRITICAL_DEPS=(
        "express"
        "mongoose"
        "@librechat/api"
        "@librechat/data-schemas"
        "librechat-data-provider"
        "@librechat/client"
    )
    
    MISSING=()
    for dep in "${CRITICAL_DEPS[@]}"; do
        if [ ! -d "node_modules/$dep" ] && [ ! -f "api/node_modules/$dep/package.json" ] 2>/dev/null; then
            MISSING+=("$dep")
        fi
    done
    
    if [ ${#MISSING[@]} -gt 0 ]; then
        warn "Some dependencies may be missing: ${MISSING[*]}"
        substep "Attempting to install backend dependencies explicitly..."
        cd api
        npm install --no-audit --legacy-peer-deps || warn "Backend dependency installation had issues"
        cd "$LIBRECHAT_DIR"
    else
        success "All critical dependencies verified"
    fi
    
    # Install lucide-react in client workspace (required for frontend build)
    substep "Installing lucide-react in client workspace..."
    if [ -d "client" ]; then
        cd client
        if [ ! -d "node_modules/lucide-react" ]; then
            info "Installing lucide-react@^0.394.0..."
            npm install lucide-react@^0.394.0 --no-audit --legacy-peer-deps || warn "Failed to install lucide-react (may cause frontend build issues)"
        else
            success "lucide-react already installed in client workspace"
        fi
        cd "$LIBRECHAT_DIR"
    else
        warn "client directory not found - skipping lucide-react installation"
    fi
    
    echo ""
}

build_packages() {
    cd "$LIBRECHAT_DIR"
    
    # Temporarily disable exit on error for build commands
    set +e
    
    # Build order is critical due to dependencies:
    # 1. data-provider (no dependencies)
    # 2. data-schemas (depends on data-provider)
    # 3. api (depends on data-provider and data-schemas)
    # 4. client package (depends on data-provider)
    
    # 1. Build data-provider
    substep "Building librechat-data-provider (1/4)..."
    BUILD_OUTPUT=$(npm run build:data-provider 2>&1)
    BUILD_STATUS=$?
    
    if [ $BUILD_STATUS -ne 0 ]; then
        set -e
        error "data-provider build failed"
        echo "$BUILD_OUTPUT" | tail -20
    fi
    
    if [ -d "packages/data-provider/dist" ]; then
        success "data-provider built successfully"
    else
        set -e
        error "data-provider build failed - dist directory not found"
    fi
    
    # 2. Build data-schemas
    substep "Building @librechat/data-schemas (2/4)..."
    BUILD_OUTPUT=$(npm run build:data-schemas 2>&1)
    BUILD_STATUS=$?
    
    if [ $BUILD_STATUS -ne 0 ]; then
        set -e
        error "data-schemas build failed"
        echo "$BUILD_OUTPUT" | tail -20
    fi
    
    if [ -d "packages/data-schemas/dist" ]; then
        success "data-schemas built successfully"
    else
        set -e
        error "data-schemas build failed - dist directory not found"
    fi
    
    # 3. Build api package
    substep "Building @librechat/api (3/4)..."
    info "This package requires data-provider and data-schemas to be built first"
    
    # Verify dependencies are built
    if [ ! -d "packages/data-provider/dist" ]; then
        error "data-provider must be built before api"
    fi
    if [ ! -d "packages/data-schemas/dist" ]; then
        error "data-schemas must be built before api"
    fi
    
    # Check for critical source files (memory module)
    substep "Verifying API source files..."
    if [ ! -f "packages/api/src/memory/config.ts" ] || [ ! -f "packages/api/src/memory/index.ts" ]; then
        error "Critical API source files are missing!"
        echo ""
        warn "The memory module files are required for the API build:"
        echo "  • packages/api/src/memory/config.ts"
        echo "  • packages/api/src/memory/index.ts"
        echo ""
        info "These files may be missing because they're ignored by .gitignore"
        info "To fix this issue:"
        substep "1. Check if files exist but are ignored:"
        echo "     ls -la InstiLibreChat/packages/api/src/memory/"
        substep "2. If files exist, force add them to git:"
        echo "     git add -f InstiLibreChat/packages/api/src/memory/"
        echo "     git commit -m 'Add memory module files'"
        substep "3. If files don't exist, check the original repository or create them:"
        echo "     The memory module should export loadMemoryConfig and isMemoryEnabled"
        echo ""
        error "Cannot proceed without memory module files"
    else
        success "Memory module source files found"
    fi
    
    # Check if API package dependencies are installed
    substep "Verifying API package dependencies..."
    if [ ! -d "packages/api/node_modules" ] && [ ! -f "packages/api/node_modules/rollup/package.json" ]; then
        warn "API package dependencies may be missing"
        info "Installing dependencies in packages/api..."
        cd packages/api
        npm install --no-audit --legacy-peer-deps || warn "API dependency installation had issues"
        cd "$LIBRECHAT_DIR"
    fi
    
    # Check for critical build tools (rollup should be in root node_modules via workspaces)
    if [ ! -f "node_modules/.bin/rollup" ] && [ ! -f "packages/api/node_modules/.bin/rollup" ] && ! command -v rollup >/dev/null 2>&1; then
        warn "rollup not found - this should be installed via root npm install"
        info "If build fails, rollup may need to be installed in root or API package"
    fi
    
    BUILD_OUTPUT=$(npm run build:api 2>&1)
    BUILD_STATUS=$?
    
    if [ $BUILD_STATUS -ne 0 ]; then
        # Check if it's just TypeScript warnings (which are non-fatal)
        if echo "$BUILD_OUTPUT" | grep -q "created dist"; then
            warn "API build completed with warnings (TypeScript type errors)"
            warn "These are usually non-fatal - checking if dist/index.js exists..."
        else
            # Show the error output
            echo ""
            error "API build failed. Error output:"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "$BUILD_OUTPUT" | tail -40
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo ""
            
            # Provide troubleshooting steps
            info "Troubleshooting steps:"
            substep "1. Try installing dependencies in the API package:"
            echo "     cd InstiLibreChat/packages/api"
            echo "     npm install --legacy-peer-deps"
            echo "     cd ../.."
            echo "     npm run build:api"
            echo ""
            substep "2. Check if rollup and plugins are installed:"
            echo "     cd InstiLibreChat/packages/api"
            echo "     npm install rollup @rollup/plugin-typescript @rollup/plugin-node-resolve --legacy-peer-deps"
            echo "     cd ../.."
            echo ""
            substep "3. Verify data-provider and data-schemas are built:"
            echo "     ls -la InstiLibreChat/packages/data-provider/dist"
            echo "     ls -la InstiLibreChat/packages/data-schemas/dist"
            echo ""
            
            # Try to auto-fix by installing dependencies
            warn "Attempting to auto-fix by installing API package dependencies..."
            cd packages/api
            npm install --no-audit --legacy-peer-deps
            cd "$LIBRECHAT_DIR"
            
            info "Retrying API build..."
            BUILD_OUTPUT=$(npm run build:api 2>&1)
            BUILD_STATUS=$?
            
            if [ $BUILD_STATUS -ne 0 ]; then
                if echo "$BUILD_OUTPUT" | grep -q "created dist"; then
                    warn "Build completed with errors but dist was created"
                else
                    set -e
                    error "API build failed after retry. Please check the error messages above."
                fi
            else
                success "API build succeeded after installing dependencies"
            fi
        fi
    fi
    
    # Verify the build output exists
    if [ -f "packages/api/dist/index.js" ]; then
        # Verify memory module is included
        if grep -q "loadMemoryConfig\|isMemoryEnabled" packages/api/dist/index.js 2>/dev/null; then
            success "api built successfully (memory module verified)"
            # Show warnings if present but don't fail
            if echo "$BUILD_OUTPUT" | grep -q "TS2345\|TS.*:"; then
                warn "TypeScript warnings detected (non-fatal):"
                echo "$BUILD_OUTPUT" | grep -E "TS[0-9]+:" | head -3 | sed 's/^/    /' || true
                info "These warnings don't prevent the build from working"
            fi
        else
            warn "api built but memory module exports not found"
            warn "This may cause issues with backend startup"
        fi
    else
        set -e
        error "api build failed - dist/index.js not found"
    fi
    
    # 4. Build client package
    substep "Building @librechat/client package (4/4)..."
    BUILD_OUTPUT=$(npm run build:client-package 2>&1)
    BUILD_STATUS=$?
    
    if [ $BUILD_STATUS -ne 0 ]; then
        set -e
        error "client package build failed"
        echo "$BUILD_OUTPUT" | tail -20
    fi
    
    if [ -d "packages/client/dist" ]; then
        success "client package built successfully"
    else
        set -e
        error "client package build failed - dist directory not found"
    fi
    
    # Re-enable exit on error
    set -e
    echo ""
}

build_frontend() {
    cd "$LIBRECHAT_DIR"
    
    substep "Building frontend application (Vite + React)..."
    info "This requires all packages to be built first"
    
    # Verify packages are built
    if [ ! -d "packages/data-provider/dist" ]; then
        error "data-provider must be built before frontend"
    fi
    if [ ! -d "packages/client/dist" ]; then
        error "client package must be built before frontend"
    fi
    
    # Verify lucide-react is installed (should be installed during dependency installation)
    if [ ! -d "node_modules/lucide-react" ] && [ ! -d "client/node_modules/lucide-react" ]; then
        warn "lucide-react not found - attempting to install..."
        cd client
        npm install lucide-react@^0.394.0 --no-audit --legacy-peer-deps || warn "Failed to install lucide-react"
        cd "$LIBRECHAT_DIR"
    fi
    
    # Temporarily disable exit on error for frontend build
    set +e
    
    # Attempt build with error capture
    BUILD_OUTPUT=$(npm run build:client 2>&1)
    BUILD_STATUS=$?
    
    # Re-enable exit on error
    set -e
    
    if [ $BUILD_STATUS -ne 0 ]; then
        # Check for specific error patterns
        if echo "$BUILD_OUTPUT" | grep -q "lucide-react"; then
            warn "Frontend build failed due to lucide-react import issue"
            echo ""
            info "Troubleshooting steps:"
            substep "1. Ensure lucide-react is installed in client workspace:"
            echo "     cd InstiLibreChat/client"
            echo "     npm install lucide-react@^0.394.0"
            substep "2. Rebuild client package:"
            echo "     cd InstiLibreChat"
            echo "     npm run build:client-package"
            substep "3. Try building frontend again:"
            echo "     npm run build:client"
            echo ""
            warn "You can still run the development server which doesn't require a build:"
            info "  npm run frontend:dev"
        elif echo "$BUILD_OUTPUT" | grep -q "Rollup failed to resolve"; then
            warn "Frontend build failed due to module resolution issue"
            echo ""
            info "This is often caused by missing peer dependencies."
            substep "Try installing dependencies in client workspace:"
            echo "  cd InstiLibreChat/client"
            echo "  npm install --legacy-peer-deps"
            echo "  cd .."
            echo "  npm run build:client"
        else
            warn "Frontend build failed (see error above)"
            info "You can still run the development server: npm run frontend:dev"
        fi
    else
        if [ -d "client/dist" ]; then
            success "Frontend built successfully"
        else
            warn "Frontend build completed but dist directory not found"
            warn "You can still run the development server"
        fi
    fi
    
    echo ""
}

verify_installation() {
    cd "$LIBRECHAT_DIR"
    
    substep "Verifying critical files and directories..."
    
    # Check backend
    if [ -f "api/server/index.js" ]; then
        success "Backend entry point found: api/server/index.js"
    else
        error "Backend entry point missing: api/server/index.js"
    fi
    
    # Check built packages
    PACKAGES=(
        "packages/data-provider/dist"
        "packages/data-schemas/dist"
        "packages/api/dist/index.js"
        "packages/client/dist"
    )
    
    for pkg in "${PACKAGES[@]}"; do
        if [ -e "$pkg" ]; then
            success "Package verified: $pkg"
        else
            error "Package missing: $pkg"
        fi
    done
    
    # Check critical dependencies
    substep "Verifying runtime dependencies..."
    RUNTIME_DEPS=(
        "express"
        "mongoose"
        "@librechat/api"
        "@librechat/data-schemas"
        "librechat-data-provider"
    )
    
    for dep in "${RUNTIME_DEPS[@]}"; do
        if [ -d "node_modules/$dep" ] || [ -f "api/node_modules/$dep/package.json" ] 2>/dev/null; then
            success "Dependency available: $dep"
        else
            warn "Dependency may be missing: $dep"
        fi
    done
    
    echo ""
}

show_summary() {
    echo -e "${GREEN}${BOLD}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║              Setup Complete! 🎉                            ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    
    # Check if frontend build succeeded
    FRONTEND_BUILT=false
    if [ -d "$LIBRECHAT_DIR/client/dist" ]; then
        FRONTEND_BUILT=true
    fi
    
    echo -e "${BOLD}Next Steps:${NC}"
    echo ""
    echo -e "${CYAN}1. Configure Environment:${NC}"
    echo "   • Create a .env file in InstiLibreChat directory, OR"
    echo "   • Configure librechat.yaml file"
    echo "   • Set required variables: MONGO_URI, API keys, etc."
    echo "   • Copy from .env.example if available"
    echo ""
    echo -e "${CYAN}2. Start MongoDB:${NC}"
    echo "   • Ensure MongoDB is running on your system"
    echo "   • Default connection: mongodb://localhost:27017"
    echo ""
    echo -e "${CYAN}3. Run the Backend:${NC}"
    echo "   ${BOLD}cd InstiLibreChat${NC}"
    echo "   ${BOLD}npm run backend${NC}        # Production mode"
    echo "   ${BOLD}npm run backend:dev${NC}    # Development mode (with auto-reload - recommended)"
    echo ""
    echo -e "${CYAN}4. Run the Frontend:${NC}"
    if [ "$FRONTEND_BUILT" = "true" ]; then
        echo "   ${BOLD}npm run frontend:dev${NC}   # Development server (recommended)"
        echo "   ${BOLD}npm run build:client${NC}   # Build for production (already built)"
    else
        echo "   ${BOLD}npm run frontend:dev${NC}   # Development server (recommended - no build needed)"
        echo ""
        warn "Frontend production build was not successful"
        info "To fix frontend build issues:"
        echo "   1. cd InstiLibreChat/client"
        echo "   2. npm install --legacy-peer-deps"
        echo "   3. npm install lucide-react@^0.394.0"
        echo "   4. cd .."
        echo "   5. npm run build:client"
    fi
    echo ""
    echo -e "${CYAN}5. Access the Application:${NC}"
    echo "   • Frontend: http://localhost:3080"
    echo "   • Backend API: http://localhost:3080/api"
    echo ""
    echo -e "${YELLOW}Note:${NC} Run backend and frontend in separate terminal windows"
    echo ""
    
    # Show troubleshooting if there were issues
    if [ "$FRONTEND_BUILT" = "false" ]; then
        echo -e "${YELLOW}${BOLD}Troubleshooting:${NC}"
        echo ""
        echo "If you encountered errors during setup:"
        echo ""
        echo "1. TypeScript warnings in API build:"
        echo "   • These are usually non-fatal and don't prevent the backend from running"
        echo "   • The build succeeded if dist/index.js exists"
        echo ""
        echo "2. Frontend build failed (lucide-react or module resolution):"
        echo "   • Development mode doesn't require a build"
        echo "   • Run: npm run frontend:dev (works without building)"
        echo "   • To fix production build:"
        echo "     cd InstiLibreChat/client"
        echo "     npm install lucide-react@^0.394.0"
        echo "     cd .."
        echo "     npm run build:client"
        echo ""
        echo "3. Missing dependencies:"
        echo "   • Run: npm install --legacy-peer-deps"
        echo "   • Or: cd client && npm install --legacy-peer-deps"
        echo ""
    fi
}

# ---------- Error Handling ----------
# Disable strict error handling for build functions (they handle errors themselves)
handle_error() {
    local line=$1
    local command="${BASH_COMMAND}"
    echo ""
    error "Script failed at line $line: $command"
    echo ""
    info "Common issues and solutions:"
    echo "  • npm install failed: Try 'npm cache clean --force' then rerun"
    echo "  • Build failed: Check that all dependencies are installed"
    echo "  • Permission errors: Check file permissions"
    echo ""
    exit 1
}

# Only trap errors in main execution, not in build functions
trap 'handle_error $LINENO' ERR

# ---------- Run Main Function ----------
main "$@"
