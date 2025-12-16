# Custom LibreChat Repository

This repository contains a customized version of LibreChat with additional SaaS API components.

## Repository Structure

- **`InstiLibreChat/`** - The main LibreChat application
- **`saas-api/`** - Additional SaaS API services (Go-based)

## Prerequisites

- Node.js (v18 or higher recommended)
- npm or bun
- MongoDB (for LibreChat)
- Go (for saas-api, if building from source)

## Setup Instructions

### 1. Clone the Repository

```bash
git clone https://github.com/AbhinavFYI/librechat_me.git
cd librechat_me
```

### 2. Install Dependencies

Navigate to the InstiLibreChat directory and install dependencies:

```bash
cd InstiLibreChat
npm install
```

This will install dependencies for the root workspace and all sub-packages.

### 3. Build Required Packages

**IMPORTANT:** After cloning, you must build the required packages before running the application. The build artifacts are not included in the repository (as they should be built locally).

Build all required packages in order:

```bash
# From the InstiLibreChat directory
npm run build:data-schemas
npm run build:data-provider
npm run build:api
npm run build:client-package
```

Or build all packages at once:

```bash
npm run build:packages
```

### 4. Build the Client (if needed)

If you plan to run the frontend:

```bash
npm run build:client
```

Or for development:

```bash
npm run frontend:dev
```

### 5. Configure Environment

Copy the example environment file and configure it:

```bash
cp .env.example .env
# Edit .env with your configuration
```

### 6. Start the Backend

```bash
npm run backend
```

Or for development:

```bash
npm run backend:dev
```

## Troubleshooting

### Missing librechat-data-provider package

If you see errors about missing `librechat-data-provider`:

```bash
cd InstiLibreChat
npm run build:data-provider
```

### Missing data-schemas build

If you see errors about missing data-schemas:

```bash
cd InstiLibreChat
npm run build:data-schemas
```

### Missing api/server/index.js

This file should exist after building. If it's missing:

```bash
cd InstiLibreChat
npm run build:api
```

## Quick Start (All-in-One)

From the `InstiLibreChat` directory:

```bash
# Install dependencies
npm install

# Build all packages
npm run build:packages

# Start backend
npm run backend
```

## Additional Resources

- [LibreChat Documentation](https://docs.librechat.ai)
- [LibreChat README](./InstiLibreChat/README.md)
- [SaaS API README](./saas-api/README.md)

## Notes

- Build artifacts (`dist/` directories) are excluded from git via `.gitignore`
- `node_modules/` are excluded and should be installed locally
- Large binary files and upload directories are excluded
- Always run `npm install` and `npm run build:packages` after cloning

