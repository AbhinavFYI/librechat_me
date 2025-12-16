#!/usr/bin/env bash

# ==========================================
# LibreChat First-Time Bootstrap Script
# ==========================================

set -euo pipefail

# ---------- Colors ----------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()    { echo -e "${BLUE}ℹ${NC} $1"; }
success() { echo -e "${GREEN}✓${NC} $1"; }
warn()    { echo -e "${YELLOW}⚠${NC} $1"; }
error()   { echo -e "${RED}✗${NC} $1"; exit 1; }

# ---------- Paths ----------
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIBRECHAT_DIR="$ROOT_DIR/InstiLibreChat"

# ---------- Checks ----------
info "LibreChat Bootstrap Script"
info "Root: $LIBRECHAT_DIR"
echo

[ -d "$LIBRECHAT_DIR" ] || error "InstiLibreChat directory not found"

command -v node >/dev/null || error "Node.js not installed"
command -v npm  >/dev/null || error "npm not installed"

NODE_VERSION=$(node -v)
NPM_VERSION=$(npm -v)

success "Node.js $NODE_VERSION"
success "npm $NPM_VERSION"
echo

cd "$LIBRECHAT_DIR"

# ---------- Clean State ----------
info "Cleaning previous installs/builds..."
rm -rf node_modules
rm -rf packages/*/node_modules
rm -rf packages/*/dist
rm -rf client/dist
rm -rf api/dist
success "Clean slate ready"
echo

# ---------- Install ----------
info "Installing workspace dependencies (npm workspaces)..."
npm install --no-audit
success "Dependencies installed"
echo

# ---------- Build Order ----------
info "Building workspace packages (correct order)"
echo

info "1/4 → data-provider"
npm run build:data-provider
[ -d packages/data-provider/dist ] || error "data-provider build failed"
success "data-provider built"
echo

info "2/4 → data-schemas"
npm run build:data-schemas
[ -d packages/data-schemas/dist ] || error "data-schemas build failed"
success "data-schemas built"
echo

info "3/4 → api"
npm run build:api
[ -f packages/api/dist/index.js ] || error "api build failed"
success "api built"
echo

info "4/4 → client package"
npm run build:client-package
[ -d packages/client/dist ] || error "client package build failed"
success "client package built"
echo

# ---------- Optional Frontend ----------
info "Building frontend (optional, safe to fail)..."
set +e
npm run build:client
FRONTEND_STATUS=$?
set -e

if [ $FRONTEND_STATUS -eq 0 ]; then
  success "Frontend built"
else
  warn "Frontend build failed (can still run backend)"
fi
echo

# ---------- Runtime Checks ----------
info "Verifying runtime prerequisites"

[ -f api/server/index.js ] \
  && success "api/server/index.js found" \
  || error "api/server/index.js missing"

[ -f .env ] \
  && success ".env file found" \
  || warn ".env missing (create before running backend)"

echo

# ---------- Summary ----------
success "LibreChat bootstrap completed 🎉"
echo
info "Next steps:"
info "1. Create .env (DO NOT COMMIT IT)"
info "2. Ensure MongoDB is running"
info "3. Run: npm run backend"
info "4. Run: npm run frontend:dev"
echo
