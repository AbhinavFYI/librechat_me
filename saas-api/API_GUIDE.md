# Complete API Guide with cURL Examples

## Base URL
```
http://localhost:8080
```

## Authentication Flow

### 1. Login (Get Access Token)

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@yourapp.com",
    "password": "superadmin123"
  }'
```

**Response:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {
    "id": "...",
    "email": "superadmin@yourapp.com",
    "is_super_admin": true
  }
}
```

### 2. Save Token for Subsequent Requests

**Option A: Save to variable (Bash/Zsh)**
```bash
# Login and save token
RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@yourapp.com",
    "password": "superadmin123"
  }')

# Extract access token
TOKEN=$(echo $RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('access_token', ''))")

# Or using jq (if installed)
TOKEN=$(echo $RESPONSE | jq -r '.access_token')

# Use token in subsequent requests
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/auth/me
```

**Option B: Save to file**
```bash
# Login and save to file
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"superadmin@yourapp.com","password":"superadmin123"}' \
  > login_response.json

# Extract token
TOKEN=$(cat login_response.json | jq -r '.access_token')
# Or
TOKEN=$(python3 -c "import json; print(json.load(open('login_response.json')).get('access_token', ''))")
```

---

## Authentication Endpoints

### Login
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@yourapp.com",
    "password": "superadmin123"
  }'
```

### Refresh Token
```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "YOUR_REFRESH_TOKEN_HERE"
  }'
```

### Get Current User (Me)
```bash
curl -X GET http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Logout
```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "YOUR_REFRESH_TOKEN_HERE"
  }'
```

---

## User Management Endpoints

### Create User
```bash
# Create user without specifying role (will get default "User" role)
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123",
    "first_name": "John",
    "last_name": "Doe",
    "phone": "+1234567890"
  }'

# Create user and assign role by NAME (e.g., make them "Org Admin")
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newadmin@example.com",
    "password": "password123",
    "first_name": "Jane",
    "last_name": "Admin",
    "phone": "+1234567890",
    "role_name": "Org Admin"
  }'

# Create user and assign role by UUID (alternative method)
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin2@example.com",
    "password": "password123",
    "first_name": "Bob",
    "last_name": "Admin",
    "role_id": "ROLE_UUID_HERE"
  }'
```

**Important:** 
- `role_name` or `role_id` specifies what role the **NEW USER** will have, NOT your role as the creator
- **Org Admin**: Always creates users in **their own organization** (cannot specify different org)
- **Super Admin**: Can create users in any org by specifying `org_id` (UUID) or `org_slug` (string)

**Organization Assignment Logic:**
- **Org Admin creating user**: User is **always** created in the org admin's organization (cannot specify different org)
- **Super Admin creating user**: **Must specify** organization (super admin may not have their own org)
  - If `org_slug` is provided: Looks up org by slug (e.g., `"acme-corp"`)
  - If `org_id` is provided: Uses the org UUID directly
  - **Error if neither is provided**: Super admin must specify `org_id` or `org_slug`

**Role Assignment Logic:**
- **If `role_name` is provided**: The new user gets that role (e.g., `"Org Admin"` makes the new user an org admin)
- **If `role_id` is provided**: The new user gets that role by UUID
- **If neither is provided**: The new user automatically gets the default role (usually "User")

**Examples:**

**Scenario 1: Org Admin creates users (always in their own org)**
```bash
# Org Admin creates a regular user (no role_name = gets default "User" role)
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer ORG_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "employee@example.com",
    "password": "password123",
    "first_name": "Employee",
    "last_name": "User"
  }'
# Result: New user created in org admin's organization with "User" role

# Org Admin creates another admin user (specify role_name = "Org Admin")
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer ORG_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "coadmin@example.com",
    "password": "password123",
    "first_name": "Co",
    "last_name": "Admin",
    "role_name": "Org Admin"
  }'
# Result: New user created in org admin's organization with "Org Admin" role
```

**Scenario 2: Super Admin creates org admin for specific organization**
```bash
# Step 1: Super Admin creates Organization "Acme Corp" (slug: "acme-corp")
curl -X POST http://localhost:8080/api/v1/organizations \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "slug": "acme-corp"
  }'
# Response: { "id": "org-uuid-1", "name": "Acme Corp", "slug": "acme-corp", ... }

# Step 2: Super Admin creates Organization "Tech Inc" (slug: "tech-inc")
curl -X POST http://localhost:8080/api/v1/organizations \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Tech Inc",
    "slug": "tech-inc"
  }'
# Response: { "id": "org-uuid-2", "name": "Tech Inc", "slug": "tech-inc", ... }

# Step 3: Super Admin creates org admin for "Acme Corp" using org_slug
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "password123",
    "first_name": "Acme",
    "last_name": "Admin",
    "org_slug": "acme-corp",
    "role_name": "Org Admin"
  }'
# Result: New user created in "Acme Corp" organization with "Org Admin" role

# Step 4: Super Admin creates org admin for "Tech Inc" using org_id
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@tech.com",
    "password": "password123",
    "first_name": "Tech",
    "last_name": "Admin",
    "org_id": "org-uuid-2",
    "role_name": "Org Admin"
  }'
# Result: New user created in "Tech Inc" organization with "Org Admin" role
```

**Q: If I create 2 orgs and make one org admin, which org will that admin belong to?**
**A:** The org admin belongs to whichever organization you specify:
- If you use `org_slug: "acme-corp"` → admin belongs to "Acme Corp"
- If you use `org_id: "org-uuid-2"` → admin belongs to that specific org
- **Important:** Super Admin **must** specify either `org_slug` or `org_id` (super admin may not have their own org)

**Examples:**
```bash
# Create regular user (gets default "User" role)
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123",
    "first_name": "John",
    "last_name": "Doe"
  }'

# Create org admin user (using role name)
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "password123",
    "first_name": "Jane",
    "last_name": "Admin",
    "role_name": "Org Admin"
  }'

# Create user with "Viewer" role (read-only access)
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "viewer@example.com",
    "password": "password123",
    "first_name": "View",
    "last_name": "Only",
    "role_name": "Viewer"
  }'
```

### List Users (Paginated)
```bash
# Basic list
curl -X GET "http://localhost:8080/api/v1/users?page=1&limit=20" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"

# With pagination
curl -X GET "http://localhost:8080/api/v1/users?page=2&limit=10" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Get User by ID
```bash
curl -X GET http://localhost:8080/api/v1/users/USER_UUID_HERE \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Update User
```bash
curl -X PUT http://localhost:8080/api/v1/users/USER_UUID_HERE \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "Jane",
    "last_name": "Smith",
    "phone": "+9876543210",
    "avatar_url": "https://example.com/avatar.jpg",
    "status": "active",
    "timezone": "America/New_York",
    "locale": "en-US"
  }'
```

### Delete User
```bash
curl -X DELETE http://localhost:8080/api/v1/users/USER_UUID_HERE \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Get User Permissions
```bash
curl -X GET http://localhost:8080/api/v1/users/USER_UUID_HERE/permissions \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Assign Role to User
```bash
# Assign role by UUID
curl -X POST http://localhost:8080/api/v1/users/USER_UUID_HERE/roles \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "role_id": "ROLE_UUID_HERE",
    "expires_at": null
  }'
```

**Note:** 
- `expires_at` is optional. If provided, use ISO 8601 format (e.g., "2025-12-31T23:59:59Z"). Set to `null` for permanent assignment.
- Currently, role assignment endpoint only accepts `role_id` (UUID). To use role names, assign the role during user creation with `role_name`.

### Remove Role from User
```bash
curl -X DELETE http://localhost:8080/api/v1/users/USER_UUID_HERE/roles/ROLE_UUID_HERE \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

---

## Organization Management Endpoints

### Create Organization
```bash
curl -X POST http://localhost:8080/api/v1/organizations \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corporation",
    "slug": "acme-corp",
    "legal_name": "Acme Corporation Inc.",
    "website": "https://acme.com",
    "primary_contact_email": "contact@acme.com",
    "primary_contact_name": "John Doe",
    "primary_contact_phone": "+1234567890",
    "billing_email": "billing@acme.com",
    "address_line1": "123 Main St",
    "city": "New York",
    "state_province": "NY",
    "postal_code": "10001",
    "country": "US",
    "subscription_plan": "pro",
    "settings": {
      "theme": {
        "primary_color": "#3B82F6"
      }
      
    }
  }'
```

### List Organizations (Paginated)
```bash
curl -X GET "http://localhost:8080/api/v1/organizations?page=1&limit=20" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Get Organization by ID
```bash
curl -X GET http://localhost:8080/api/v1/organizations/ORG_UUID_HERE \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Update Organization
```bash
curl -X PUT http://localhost:8080/api/v1/organizations/ORG_UUID_HERE \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Name",
    "website": "https://newwebsite.com",
    "settings": {
      "theme": {
        "primary_color": "#10B981"
      }
    }
  }'
```

### Delete Organization
```bash
# Super Admin can delete any organization
curl -X DELETE http://localhost:8080/api/v1/organizations/ORG_UUID_HERE \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN"

# Org Admin can delete their own organization (requires organizations:delete permission)
curl -X DELETE http://localhost:8080/api/v1/organizations/ORG_UUID_HERE \
  -H "Authorization: Bearer ORG_ADMIN_TOKEN"
```

**Important:**
- **Super Admin**: Can delete **any** organization (bypasses permission checks and org restrictions)
- **Org Admin**: Can only delete **their own** organization (requires `organizations:delete` permission)
- **Regular User**: Cannot delete organizations (requires `organizations:delete` permission + must be org admin)

**Example:**
```bash
# Step 1: Get organization ID (if you only know the slug)
curl -X GET "http://localhost:8080/api/v1/organizations?page=1&limit=20" \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN"
# Response will include the organization ID

# Step 2: Delete the organization
curl -X DELETE http://localhost:8080/api/v1/organizations/8fe888b6-f88d-4209-b642-d3137b390e1c \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN"
# Response: {"message": "Organization deleted successfully"}

# Note: Deletion is soft delete - organization is marked as deleted but not removed from database
```

---

## Role Management Endpoints

### Create Role
```bash
# Org Admin creates role in their organization
curl -X POST http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer ORG_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Manager",
    "description": "Manager role with specific permissions",
    "is_default": false,
    "permission_ids": [
      "PERMISSION_UUID_1",
      "PERMISSION_UUID_2"
    ]
  }'

# Super Admin creates role in specific organization
curl -X POST http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Custom Role",
    "description": "Custom role for specific org",
    "org_id": "ORG_UUID_HERE",
    "is_default": false
  }'

# Super Admin creates system role (no org_id)
curl -X POST http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "System Role",
    "description": "System-wide role",
    "is_default": false
  }'
```

**Important:**
- **Org Admin**: Creates roles in their own organization automatically (cannot specify different org)
- **Super Admin**: Can create roles in any org by specifying `org_id`, or create system roles (no `org_id`)
- `permission_ids` is optional. You can assign permissions when creating the role or later using the assign permissions endpoint.

### List Roles
```bash
# List all roles in your organization
curl -X GET http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

**Note:** Returns roles for your organization. Super admins can see system roles as well.

### Get Role by ID
```bash
curl -X GET http://localhost:8080/api/v1/roles/ROLE_UUID_HERE \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Update Role
```bash
curl -X PUT http://localhost:8080/api/v1/roles/ROLE_UUID_HERE \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Role Name",
    "description": "Updated description",
    "is_default": false
  }'
```

**Note:** All fields are optional. Only provided fields will be updated.

### Delete Role
```bash
curl -X DELETE http://localhost:8080/api/v1/roles/ROLE_UUID_HERE \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Get Role Permissions
```bash
curl -X GET http://localhost:8080/api/v1/roles/ROLE_UUID_HERE/permissions \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Assign Permissions to Role
```bash
curl -X POST http://localhost:8080/api/v1/roles/ROLE_UUID_HERE/permissions \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "permission_ids": [
      "PERMISSION_UUID_1",
      "PERMISSION_UUID_2",
      "PERMISSION_UUID_3"
    ]
  }'
```

**Note:** This replaces all existing permissions for the role with the provided list.

---

## Permission Management Endpoints

### List All Permissions
```bash
curl -X GET http://localhost:8080/api/v1/permissions \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

**Response:**
```json
[
  {
    "id": "uuid",
    "resource": "users",
    "action": "create",
    "description": "Create new users",
    "is_system": true,
    "created_at": "2025-12-03T..."
  },
  {
    "id": "uuid",
    "resource": "users",
    "action": "update",
    "description": "Update users",
    "is_system": true,
    "created_at": "2025-12-03T..."
  }
]
```

---

## Admin Endpoints (Super Admin Only)

### List All Users (Admin)
```bash
curl -X GET "http://localhost:8080/api/v1/admin/users?page=1&limit=20" \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN"
```

### List All Organizations (Admin)
```bash
curl -X GET "http://localhost:8080/api/v1/admin/organizations?page=1&limit=20" \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN"
```

---

## Health Check

```bash
curl -X GET http://localhost:8080/health
```

---
## Complete Role & Permission Management Example

This example shows how to:
1. Get available permissions
2. Create a custom role
3. Assign permissions to the role
4. Create a user
5. Assign the role to the user
6. Verify user permissions

```bash
#!/bin/bash

BASE_URL="http://localhost:8080/api/v1"

# Step 1: Login
echo "=== 1. Login ==="
LOGIN_RESPONSE=$(curl -s -X POST $BASE_URL/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@yourapp.com",
    "password": "superadmin123"
  }')

TOKEN=$(echo $LOGIN_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('access_token', ''))")
echo "Token obtained: ${TOKEN:0:50}..."

# Step 2: Get available permissions
echo -e "\n=== 2. Get Available Permissions ==="
PERMISSIONS=$(curl -s -X GET $BASE_URL/permissions \
  -H "Authorization: Bearer $TOKEN")

# Extract permission IDs (example - you'd use actual UUIDs from response)
# USER_CREATE_PERM=$(echo $PERMISSIONS | python3 -c "import sys, json; perms=json.load(sys.stdin); print([p['id'] for p in perms if p['resource']=='users' and p['action']=='create'][0])")
# USER_UPDATE_PERM=$(echo $PERMISSIONS | python3 -c "import sys, json; perms=json.load(sys.stdin); print([p['id'] for p in perms if p['resource']=='users' and p['action']=='update'][0])")

echo $PERMISSIONS | python3 -m json.tool | head -20

# Step 3: Create a custom role
echo -e "\n=== 3. Create Custom Role ==="
ROLE_RESPONSE=$(curl -s -X POST $BASE_URL/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Content Manager",
    "description": "Can manage content and users",
    "is_default": false
  }')

ROLE_ID=$(echo $ROLE_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))")
echo "Role created with ID: $ROLE_ID"
echo $ROLE_RESPONSE | python3 -m json.tool

# Step 4: Assign permissions to role (replace PERMISSION_UUIDs with actual IDs from step 2)
echo -e "\n=== 4. Assign Permissions to Role ==="
curl -s -X POST "$BASE_URL/roles/$ROLE_ID/permissions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "permission_ids": [
      "PERMISSION_UUID_1",
      "PERMISSION_UUID_2"
    ]
  }' | python3 -m json.tool

# Step 5: Create a user with role assigned (using role name - easier!)
echo -e "\n=== 5. Create User with Role (using role name) ==="
USER_RESPONSE=$(curl -s -X POST $BASE_URL/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "manager@example.com",
    "password": "password123",
    "first_name": "John",
    "last_name": "Manager",
    "role_name": "Content Manager"
  }')

USER_ID=$(echo $USER_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))")
echo "User created with ID: $USER_ID and role assigned automatically"
echo $USER_RESPONSE | python3 -m json.tool

# Alternative: Create user first, then assign role
echo -e "\n=== 5b. Create User (then assign role separately) ==="
USER_RESPONSE2=$(curl -s -X POST $BASE_URL/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user2@example.com",
    "password": "password123",
    "first_name": "Regular",
    "last_name": "User"
  }')
USER_ID2=$(echo $USER_RESPONSE2 | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))")

# Step 6: Assign role to user (using UUID)
echo -e "\n=== 6. Assign Role to User (using UUID) ==="
curl -s -X POST "$BASE_URL/users/$USER_ID2/roles" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"role_id\": \"$ROLE_ID\"
  }" | python3 -m json.tool

# Step 7: Verify user permissions
echo -e "\n=== 7. Get User Permissions ==="
curl -s -X GET "$BASE_URL/users/$USER_ID/permissions" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

echo -e "\n✅ Complete workflow completed!"
```

---

## Permission Requirements Summary

### User Management
- **Create User**: Requires `users:create` permission
- **Update User**: Requires `users:update` permission + same org
- **Delete User**: Requires `users:delete` permission + same org
- **List Users**: No permission required (org-scoped)
- **Get User**: No permission required (org-scoped)
- **Assign Role**: Requires `users:update` permission + same org

### Organization Management
- **Create Organization**: Requires `organizations:create` permission
- **Update Organization**: Requires `organizations:update` permission
- **Delete Organization**: Requires `organizations:delete` permission
- **List Organizations**: No permission required (org-scoped)

### Role Management
- **Create Role**: Requires `roles:create` permission
- **Update Role**: Requires `roles:update` permission + same org
- **Delete Role**: Requires `roles:delete` permission + same org
- **List Roles**: No permission required (org-scoped)
- **Assign Permissions**: Requires `roles:update` permission + same org

### Permission Management
- **List Permissions**: No permission required

### Notes
- **Super Admins**: Bypass all permission checks and org restrictions
- **Org Admins**: Can only manage resources in their own organization
- **Regular Users**: Need explicit permissions for each action

---

## Quick Reference: Using Role Names vs UUIDs & Organization Assignment

### When Creating Users

**Important:** 
- `role_name` or `role_id` specifies what role the **NEW USER** will have, not your role as the creator
- **Org Admin**: Always creates users in their own org (cannot specify different org)
- **Super Admin**: Can specify org using `org_slug` or `org_id`, or omit to use their own org

### Organization Assignment (Super Admin Only)

**Option 1: Use Org Slug (Recommended - Easier)**
```bash
# Super Admin creates org admin for "acme-corp" organization
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "password123",
    "first_name": "Acme",
    "last_name": "Admin",
    "org_slug": "acme-corp",
    "role_name": "Org Admin"
  }'
```

**Option 2: Use Org UUID**
```bash
# Super Admin creates org admin using organization UUID
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "password123",
    "first_name": "Acme",
    "last_name": "Admin",
    "org_id": "abc-123-def-456-uuid",
    "role_name": "Org Admin"
  }'
```

**Important:** Super Admin **must** specify `org_slug` or `org_id` when creating users (super admin may not have their own organization).

### Role Assignment

**Option 1: Use Role Name (Recommended - Easier)**
```bash
# Create a new user and make them an "Org Admin"
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newadmin@example.com",
    "password": "password123",
    "first_name": "New",
    "last_name": "Admin",
    "role_name": "Org Admin"
  }'
```

**Option 2: Use Role UUID**
```bash
# Create a new user and assign role by UUID
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newadmin@example.com",
    "password": "password123",
    "first_name": "New",
    "last_name": "Admin",
    "role_id": "abc-123-def-456-uuid"
  }'
```

**Option 3: Don't Specify Role (Gets Default)**
```bash
# Create a new user - they'll automatically get the default "User" role
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "employee@example.com",
    "password": "password123",
    "first_name": "Regular",
    "last_name": "User"
  }'
# Result: New user gets default "User" role
```

**Common Role Names:**
- `"Org Admin"` - Full administrative access to organization
- `"User"` - Standard user access (default)
- `"Viewer"` - Read-only access
- Custom roles you create (use exact name as created)

**Notes:** 
- Role names are case-sensitive. Use exact names as they appear in your organization.
- The `role_name` field is for the **new user being created**, not for you (the admin creating them).
- **Org Admin**: Cannot specify `org_id` or `org_slug` - users are always created in their own org.
- **Super Admin**: **Must** specify `org_slug` (easier) or `org_id` (UUID) when creating users - super admin may not have their own organization.

