# Quick Start Guide

## 1. Install Dependencies

```bash
cd saas-api
go mod tidy
```

## 2. Set Environment Variables

Create a `.env` file or export variables:

```bash
export DB_NAME=saas_database
export DB_USER=aep014
export JWT_SECRET=your-secret-key-here
```

## 3. Run the API

```bash
go run cmd/api/main.go
```

The API will start on `http://localhost:8080`

## 4. Test the API

### Login as Super Admin

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@yourapp.com",
    "password": "superadmin123"
  }'
```

Response:
```json
{
  "access_token": "...",
  "refresh_token": "...",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {...}
}
```

### Use the Access Token

```bash
TOKEN="your_access_token_here"

# Get current user
curl -X GET http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer $TOKEN"

# List users
curl -X GET "http://localhost:8080/api/v1/users?page=1&limit=10" \
  -H "Authorization: Bearer $TOKEN"
```

## API Structure

- **Public Routes**: `/api/v1/auth/*` (login, refresh)
- **Protected Routes**: `/api/v1/users/*`, `/api/v1/organizations/*` (require auth)
- **Admin Routes**: `/api/v1/admin/*` (require super admin)

## Frontend Integration

1. Store tokens in localStorage or httpOnly cookies
2. Include `Authorization: Bearer {token}` header in requests
3. Handle 401 responses by refreshing token
4. Use pagination query params: `?page=1&limit=20`

## Next Steps

- Implement role and permission management endpoints
- Add invitation system endpoints
- Add audit log endpoints
- Configure CORS for your frontend domain
- Set up rate limiting
- Add request validation middleware

