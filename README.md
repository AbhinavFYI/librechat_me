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

## Quick Setup (Automated)

**Easiest way:** Use the automated setup script after cloning:

```bash
# From the repository root directory
./setup.sh
```

This script will:
- Check prerequisites (Node.js, npm)
- Install all dependencies
- Build all required packages
- Build the client application

Alternatively, use the frontend-only build script:

```bash
# Build only the frontend (assumes dependencies are already installed)
./build-frontend.sh
```

## Setup Instructions (Manual)

**Important:** Most commands in this guide should be run from the `InstiLibreChat` directory unless otherwise specified. After step 2, you'll be in the `InstiLibreChat` directory for the remaining steps.

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

The following packages need to be built:
- `@librechat/data-schemas` - Mongoose schemas and models
- `librechat-data-provider` - Data services for LibreChat apps
- `@librechat/api` - API package (creates `api/server/index.js`)
- `@librechat/client` - React components package

Build all required packages in order:

```bash
# From the InstiLibreChat directory
npm run build:data-schemas
npm run build:data-provider
npm run build:api
npm run build:client-package
```

Or build all packages at once (recommended):

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

Create a `.env` file in the `InstiLibreChat` directory with your configuration. 

**Note:** There is no `.env.example` file in this repository. You'll need to create your own `.env` file based on your requirements. 

For detailed environment variable configuration, refer to the [LibreChat Documentation](https://docs.librechat.ai) or check the original [LibreChat repository](https://github.com/danny-avila/LibreChat) for example configurations.

Common environment variables include:
- `MONGO_URI` - MongoDB connection string
- `MEILI_HOST` - Meilisearch host URL
- `OPENAI_API_KEY` - OpenAI API key (if using OpenAI)
- `PORT` - Server port (default: 3080)

```bash
# Create .env file in InstiLibreChat directory
cd InstiLibreChat
touch .env
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

### Missing @librechat/client package

If you see errors about missing `@librechat/client`:

```bash
cd InstiLibreChat
npm run build:client-package
```

This builds the `packages/client` package which creates the `dist/` directory needed by the client application.

### Missing api/server/index.js

This file should exist after building. If it's missing:

```bash
cd InstiLibreChat
npm run build:api
```

## Quick Start (All-in-One)

### Option 1: Using Setup Script (Recommended)

From the repository root:

```bash
# Run the automated setup script
./setup.sh

# Then start the backend
cd InstiLibreChat
npm run backend
```

### Option 2: Manual Setup

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

