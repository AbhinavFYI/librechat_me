# LibreChat Proxy Server

This proxy server bypasses LibreChat's login by authenticating users through our system and injecting authentication headers.

## Setup

1. Install dependencies:
```bash
cd saas-api
go mod tidy
```

2. Install required packages:
```bash
go get github.com/golang-jwt/jwt/v5
go get github.com/gorilla/websocket
```

3. Configure environment variables (optional):
```bash
export JWT_SECRET="your-secret-key"  # Default: "mysecret123"
export LIBRE_BACKEND="http://localhost:3080"  # LibreChat backend API (Default: "http://localhost:3080")
export LIBRE_FRONTEND="http://localhost:3090"  # LibreChat frontend dev server (Default: "http://localhost:3090")
export PROXY_PORT="9443"             # Default: "9443"
export USE_HTTPS="false"             # Set to "true" for HTTPS (requires cert.pem and key.pem)
```

4. Run the proxy server:
```bash
cd cmd/proxy
go run main.go
```

Or build and run:
```bash
cd cmd/proxy
go build -o proxy main.go
./proxy
```

## How it works

1. **Login Endpoint** (`/login`): Accepts email and creates a JWT token, setting it as an HttpOnly cookie
2. **Backend Proxy** (`/api/*`, `/oauth/*`): Proxies API requests to LibreChat backend (`http://localhost:3080`)
3. **Frontend Proxy** (`/proxy/*`, `/`): Proxies frontend requests to Vite dev server (`http://localhost:3090`)
4. **Authentication**: Extracts JWT from cookie or Authorization header and injects `X-Authenticated-User` header
5. **WebSocket Support**: Proxies WebSocket connections for both backend and frontend
6. **HTML URL Rewriting**: Rewrites URLs in HTML responses to use `/proxy/` prefix for frontend assets

## Integration with Main App

The frontend automatically:
- Calls `/login` after successful OTP verification
- Uses `http://localhost:9443/proxy/c/new` instead of direct LibreChat URL
- Maintains authentication through cookies
- API calls to `/api/*` are automatically proxied to backend (port 3080)
- Frontend assets from Vite dev server (port 3090) are proxied through `/proxy/*`

## LibreChat Configuration

Make sure LibreChat is configured to trust the `X-Authenticated-User` header. This typically requires:
- Setting up LibreChat to read user email from this header
- Creating/updating user in LibreChat database based on the email

## Notes

- For development, the server runs on HTTP (port 9443)
- For production, set `USE_HTTPS=true` and provide `cert.pem` and `key.pem`
- The cookie uses `SameSite=Lax` for iframe compatibility
- WebSocket connections are fully proxied with authentication headers
- **Backend** (port 3080): Handles `/api` and `/oauth` routes
- **Frontend** (port 3090): Handles all other routes including Vite HMR (`/@vite/client`, `/src/`, etc.)
- HTML responses are automatically rewritten to use `/proxy/` prefix for relative URLs

