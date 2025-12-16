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
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBRECHAT_DIR="$ROOT_DIR/InstiLibreChat"

# ---------- Checks ----------
info "LibreChat Bootstrap Script"
info "Root: $LIBRECHAT_DIR"
echo

[ -d "$LIBRECHAT_DIR" ] || error "InstiLibreChat directory not found"

# Check for Node.js
if ! command -v node >/dev/null; then
  error "Node.js is not installed"
  echo ""
  info "Please install Node.js (v18 or higher recommended)"
  info ""
  info "Installation options:"
  info "  • macOS (using Homebrew): brew install node"
  info "  • Linux (using nvm): curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash"
  info "  • Windows: Download from https://nodejs.org/"
  info "  • Or use nvm: https://github.com/nvm-sh/nvm"
  info ""
  info "After installing Node.js, run this script again."
  exit 1
fi

# Check for npm
if ! command -v npm >/dev/null; then
  error "npm is not installed"
  echo ""
  info "npm should come with Node.js. If you see this error,"
  info "your Node.js installation may be incomplete."
  info "Please reinstall Node.js from https://nodejs.org/"
  exit 1
fi

# Check Node.js version
NODE_VERSION=$(node -v)
NPM_VERSION=$(npm -v)

# Extract major version number
NODE_MAJOR_VERSION=$(echo "$NODE_VERSION" | sed 's/v\([0-9]*\).*/\1/')

if [ "$NODE_MAJOR_VERSION" -lt 18 ]; then
  warn "Node.js version $NODE_VERSION detected"
  warn "Node.js v18 or higher is recommended"
  warn "You may encounter issues with older versions"
  echo ""
  read -p "Continue anyway? (y/N) " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    info "Please upgrade Node.js to v18 or higher and run this script again"
    exit 1
  fi
fi

success "Node.js $NODE_VERSION"
success "npm $NPM_VERSION"
echo

cd "$LIBRECHAT_DIR"

# ---------- Clean State ----------
info "Cleaning previous installs/builds..."
rm -rf node_modules
rm -rf packages/*/node_modules
rm -rf packages/*/dist
rm -rf client/node_modules
rm -rf client/dist
rm -rf api/node_modules
rm -rf api/dist
success "Clean slate ready"
echo

# ---------- Install ----------
info "Installing workspace dependencies (npm workspaces)..."
info "This installs all dependencies including peer dependencies for @librechat/api"
npm install --no-audit --legacy-peer-deps
success "Dependencies installed"
echo

# ---------- Verify Dependencies ----------
info "Verifying dependencies for all workspaces..."

# Verify API package build dependencies
if [ -d "node_modules/@rollup" ] && [ -d "node_modules/rollup" ]; then
  success "API build dependencies verified (hoisted to root)"
else
  warn "Some API build dependencies may be missing"
fi

# Verify backend (api workspace) dependencies
info "Verifying backend (api) dependencies..."
BACKEND_DEPS=(
  "express"
  "mongoose"
  "passport"
  "cors"
  "dotenv"
  "@librechat/api"
  "@librechat/data-schemas"
  "librechat-data-provider"
)

MISSING_DEPS=()
for dep in "${BACKEND_DEPS[@]}"; do
  # Check if dependency exists in node_modules (workspace dependencies are hoisted)
  if [ ! -d "node_modules/$dep" ] && [ ! -f "api/node_modules/$dep/package.json" ] 2>/dev/null; then
    MISSING_DEPS+=("$dep")
  fi
done

if [ ${#MISSING_DEPS[@]} -gt 0 ]; then
  warn "Some backend dependencies may be missing: ${MISSING_DEPS[*]}"
  info "Installing backend dependencies explicitly..."
  cd api
  npm install --no-audit --legacy-peer-deps || warn "Backend dependency installation had issues"
  cd "$LIBRECHAT_DIR"
else
  success "Backend dependencies verified"
fi
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
info "Building @librechat/api (requires all peer dependencies)..."

# Ensure data-provider is available (peer dependency)
if [ ! -d "packages/data-provider/dist" ]; then
  error "data-provider must be built before api. Run: npm run build:data-provider"
fi

# Ensure data-schemas is available (peer dependency)
if [ ! -d "packages/data-schemas/dist" ]; then
  error "data-schemas must be built before api. Run: npm run build:data-schemas"
fi

# Verify TypeScript config has path aliases configured
if ! grep -q '"~/\*"' packages/api/tsconfig.build.json 2>/dev/null; then
  warn "tsconfig.build.json may be missing path alias configuration"
  warn "This could cause memory module build issues"
fi

# Build API package
cd packages/api
npm run build
cd "$LIBRECHAT_DIR"

# Verify API build succeeded
[ -f packages/api/dist/index.js ] || error "api build failed - dist/index.js not found"

# Verify memory module is included in the build (critical for backend)
if grep -q "loadMemoryConfig\|isMemoryEnabled" packages/api/dist/index.js 2>/dev/null; then
  success "api built (memory module verified)"
else
  error "api build failed - memory module exports not found in bundle"
  info "The memory module is required for the backend to start"
  info "This usually means the path alias ~/memory/config is not resolving correctly"
  info "Check rollup.config.js alias plugin configuration"
  info "Try rebuilding: cd packages/api && npm run build"
  exit 1
fi
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

# Verify critical backend dependencies are available
info "Verifying critical backend dependencies..."
CRITICAL_BACKEND_DEPS=(
  "express"
  "mongoose"
  "@librechat/api"
  "@librechat/data-schemas"
  "librechat-data-provider"
)

for dep in "${CRITICAL_BACKEND_DEPS[@]}"; do
  if [ -d "node_modules/$dep" ] || [ -f "api/node_modules/$dep/package.json" ] 2>/dev/null; then
    success "  ✓ $dep"
  else
    error "  ✗ $dep missing - backend may not start"
  fi
done

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
