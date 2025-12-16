# SaaS Multi-Tenant API

Production-ready REST API for multi-tenant SaaS user management system built with Go, PostgreSQL, and Gin.

## Features

- ✅ Multi-tenant architecture with Row-Level Security (RLS)
- ✅ JWT-based authentication with refresh tokens
- ✅ Role-Based Access Control (RBAC)
- ✅ User and Organization management
- ✅ Comprehensive error handling
- ✅ Database connection pooling
- ✅ CORS support
- ✅ Graceful shutdown

## Project Structure

```
saas-api/
├── cmd/
│   └── api/
│       └── main.go          # Application entry point
├── config/
│   └── config.go            # Configuration management
├── internal/
│   ├── auth/
│   │   └── jwt.go           # JWT token service
│   ├── database/
│   │   └── database.go      # Database connection
│   ├── handlers/
│   │   ├── auth_handler.go
│   │   ├── user_handler.go
│   │   └── organization_handler.go
│   ├── middleware/
│   │   ├── auth_middleware.go
│   │   ├── rls_middleware.go
│   │   ├── cors_middleware.go
│   │   └── error_middleware.go
│   ├── models/
│   │   └── models.go        # Data models
│   ├── repositories/
│   │   ├── user_repository.go
│   │   ├── organization_repository.go
│   │   ├── role_repository.go
│   │   ├── permission_repository.go
│   │   └── refresh_token_repository.go
│   └── services/
│       └── auth_service.go
├── pkg/
│   ├── errors/
│   │   └── errors.go        # Error handling
│   └── utils/
│       └── utils.go         # Utility functions
└── go.mod
```

## Setup

### 1. Install Dependencies

```bash
go mod download
```

### 2. Configure Environment Variables

Copy `.env.example` to `.env` and configure:

```bash
# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_READ_TIMEOUT=15
SERVER_WRITE_TIMEOUT=15
SERVER_IDLE_TIMEOUT=60

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=saas_database
DB_SSLMODE=disable

# JWT
JWT_SECRET=your-super-secret-jwt-key-change-in-production
JWT_ACCESS_TTL=15
JWT_REFRESH_TTL=7

# App
APP_ENV=development
LOG_LEVEL=info
```

### 3. Run Database Migrations

Make sure your database is set up using the `db_setup.sql` script:

```bash
psql -d saas_database -f ../db_setup.sql
```

### 4. Run the Server

```bash
go run cmd/api/main.go
```

Or build and run:

```bash
go build -o bin/api cmd/api/main.go
./bin/api
```

## API Endpoints

### Authentication

- `POST /api/v1/auth/login` - Login
- `POST /api/v1/auth/refresh` - Refresh access token
- `POST /api/v1/auth/logout` - Logout
- `GET /api/v1/auth/me` - Get current user info

### Users

- `POST /api/v1/users` - Create user
- `GET /api/v1/users` - List users (paginated)
- `GET /api/v1/users/:id` - Get user by ID
- `PUT /api/v1/users/:id` - Update user
- `DELETE /api/v1/users/:id` - Delete user
- `GET /api/v1/users/:id/permissions` - Get user permissions

### Organizations

- `POST /api/v1/organizations` - Create organization
- `GET /api/v1/organizations` - List organizations (paginated)
- `GET /api/v1/organizations/:id` - Get organization by ID
- `PUT /api/v1/organizations/:id` - Update organization
- `DELETE /api/v1/organizations/:id` - Delete organization

### Admin (Super Admin Only)

- `GET /api/v1/admin/users` - List all users
- `GET /api/v1/admin/organizations` - List all organizations

## Example Requests

### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@yourapp.com",
    "password": "superadmin123"
  }'
```

### Create User (Authenticated)

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "email": "user@example.com",
    "password": "password123",
    "first_name": "John",
    "last_name": "Doe"
  }'
```

### Get Users

```bash
curl -X GET "http://localhost:8080/api/v1/users?page=1&limit=20" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

## Frontend Integration

The API is designed to work seamlessly with frontend frameworks:

1. **Authentication Flow:**
   - Login → Store `access_token` and `refresh_token`
   - Include `Authorization: Bearer {access_token}` in requests
   - Refresh token when access token expires (401 response)

2. **Error Handling:**
   - All errors follow consistent format:
   ```json
   {
     "error": "ERROR_CODE",
     "message": "Human readable message"
   }
   ```

3. **Pagination:**
   - Use `page` and `limit` query parameters
   - Response includes `total` and `total_pages`

4. **Multi-tenancy:**
   - Users automatically scoped to their organization
   - Super admins can access all resources

## Development

### Running Tests

```bash
go test ./...
```

### Building for Production

```bash
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api cmd/api/main.go
```

## Security Considerations

- Change default JWT secret in production
- Use HTTPS in production
- Configure CORS origins properly
- Set up rate limiting
- Use environment variables for secrets
- Enable SSL for database connections in production

## License

MIT

