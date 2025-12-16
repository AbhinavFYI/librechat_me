#!/usr/bin/env bash

# ==========================================
# LibreChat Start Script
# ==========================================
# This script prepares and starts LibreChat frontend and backend
# ==========================================

set -euo pipefail

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
    echo "║           LibreChat Start Script                          ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    
    # Verify project structure
    if [ ! -d "$LIBRECHAT_DIR" ]; then
        error "InstiLibreChat directory not found at: $LIBRECHAT_DIR"
    fi
    
    cd "$LIBRECHAT_DIR"
    
    # Step 1: Setup frontend dependencies
    step "Step 1: Setting up frontend dependencies"
    setup_frontend
    
    # Step 2: Setup backend dependencies
    step "Step 2: Setting up backend dependencies"
    setup_backend
    
    # Step 3: Start services
    step "Step 3: Starting LibreChat"
    start_services
}

# ---------- Functions ----------

setup_frontend() {
    substep "Installing frontend dependencies..."
    
    if [ ! -d "client" ]; then
        error "client directory not found"
    fi
    
    cd client
    
    # Install vite
    if [ ! -d "node_modules/vite" ] && [ ! -f "node_modules/.bin/vite" ]; then
        info "Installing vite..."
        npm install vite --no-audit --legacy-peer-deps || warn "Failed to install vite"
    else
        success "vite already installed"
    fi
    
    cd "$LIBRECHAT_DIR"
    success "Frontend dependencies ready"
    echo ""
}

setup_backend() {
    substep "Installing backend dependencies..."
    
    # Install lucide-react in client and build
    if [ -d "client" ]; then
        cd client
        
        # Install lucide-react
        if [ ! -d "node_modules/lucide-react" ]; then
            info "Installing lucide-react..."
            npm install lucide-react@^0.394.0 --no-audit --legacy-peer-deps || warn "Failed to install lucide-react"
        else
            success "lucide-react already installed"
        fi
        
        cd "$LIBRECHAT_DIR"
        
        # Build client (from root directory)
        substep "Building frontend client..."
        npm run build:client || warn "Frontend build had issues (development server will still work)"
    fi
    
    # Install winston-daily-rotate-file for backend
    substep "Installing winston-daily-rotate-file..."
    if [ ! -d "node_modules/winston-daily-rotate-file" ]; then
        npm install winston-daily-rotate-file --no-audit --legacy-peer-deps || warn "Failed to install winston-daily-rotate-file"
    else
        success "winston-daily-rotate-file already installed"
    fi
    
    success "Backend dependencies ready"
    echo ""
}

start_services() {
    echo -e "${GREEN}${BOLD}"
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║              Starting LibreChat                           ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    
    info "Starting backend..."
    info "Backend will run in the foreground. Press Ctrl+C to stop."
    echo ""
    info "To run frontend in another terminal:"
    echo "  ${BOLD}cd InstiLibreChat && npm run frontend:dev${NC}"
    echo ""
    info "Or:"
    echo "  ${BOLD}cd InstiLibreChat/client && npm run dev${NC}"
    echo ""
    
    # Check for .env file
    if [ ! -f ".env" ] && [ ! -f "librechat.yaml" ]; then
        warn "No .env or librechat.yaml file found"
        info "Make sure you have configured your environment variables"
        echo ""
    fi
    
    # Start backend
    success "Starting backend server..."
    npm run backend
}

# ---------- Error Handling ----------
trap 'error "Script failed at line $LINENO"' ERR

# ---------- Run Main Function ----------
main "$@"

