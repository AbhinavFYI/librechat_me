#!/bin/bash

BASE_URL="http://localhost:8080/api/v1"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "COMPREHENSIVE API TESTING"
echo "=========================================="
echo ""

# ==========================================
# 1. AUTHENTICATION
# ==========================================
echo -e "${YELLOW}=== 1. AUTHENTICATION ===${NC}"

echo -e "\n1.1 Login"
LOGIN_RESPONSE=$(curl -s -X POST $BASE_URL/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"superadmin@yourapp.com","password":"superadmin123"}')

TOKEN=$(echo $LOGIN_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('access_token', ''))" 2>/dev/null)
if [ -z "$TOKEN" ] || [ "$TOKEN" = "None" ]; then
  echo -e "${RED}❌ Login FAILED${NC}"
  echo "$LOGIN_RESPONSE"
  exit 1
else
  echo -e "${GREEN}✅ Login SUCCESS${NC}"
  echo "Token: ${TOKEN:0:50}..."
fi

REFRESH_TOKEN=$(echo $LOGIN_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('refresh_token', ''))" 2>/dev/null)

echo -e "\n1.2 Get Current User (Me)"
ME_RESPONSE=$(curl -s -X GET $BASE_URL/auth/me -H "Authorization: Bearer $TOKEN")
if echo "$ME_RESPONSE" | grep -q "user_id"; then
  echo -e "${GREEN}✅ Get Me SUCCESS${NC}"
else
  echo -e "${RED}❌ Get Me FAILED${NC}"
  echo "$ME_RESPONSE"
fi

echo -e "\n1.3 Refresh Token"
if [ -z "$REFRESH_TOKEN" ] || [ "$REFRESH_TOKEN" = "None" ] || [ "$REFRESH_TOKEN" = "" ]; then
  echo -e "${RED}❌ Refresh Token FAILED - No refresh token from login${NC}"
  echo "Login response (first 200 chars): ${LOGIN_RESPONSE:0:200}..."
else
  echo "Refresh token extracted (length: ${#REFRESH_TOKEN})"
  REFRESH_RESPONSE=$(curl -s -X POST $BASE_URL/auth/refresh \
    -H "Content-Type: application/json" \
    -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}")
  if echo "$REFRESH_RESPONSE" | grep -q "access_token"; then
    echo -e "${GREEN}✅ Refresh Token SUCCESS${NC}"
    NEW_TOKEN=$(echo $REFRESH_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('access_token', ''))" 2>/dev/null)
    TOKEN=$NEW_TOKEN
  else
    echo -e "${RED}❌ Refresh Token FAILED${NC}"
    echo "Response: $REFRESH_RESPONSE"
    echo "Check server logs for details"
  fi
fi

# ==========================================
# 2. ORGANIZATION MANAGEMENT
# ==========================================
echo -e "\n${YELLOW}=== 2. ORGANIZATION MANAGEMENT ===${NC}"

echo -e "\n2.1 Create Organization"
ORG_CREATE_RESPONSE=$(curl -s -X POST $BASE_URL/organizations \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Org '$(date +%s)'",
    "slug": "test-org-'$(date +%s)'",
    "website": "https://test.com"
  }')
ORG_ID=$(echo $ORG_CREATE_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null)
if [ -n "$ORG_ID" ] && [ "$ORG_ID" != "None" ]; then
  echo -e "${GREEN}✅ Create Organization SUCCESS${NC}"
  echo "Org ID: $ORG_ID"
else
  echo -e "${RED}❌ Create Organization FAILED${NC}"
  echo "$ORG_CREATE_RESPONSE"
  ORG_ID=""
fi

echo -e "\n2.2 List Organizations"
ORG_LIST_RESPONSE=$(curl -s -X GET "$BASE_URL/organizations?page=1&limit=5" \
  -H "Authorization: Bearer $TOKEN")
if echo "$ORG_LIST_RESPONSE" | grep -q "data"; then
  echo -e "${GREEN}✅ List Organizations SUCCESS${NC}"
  ORG_COUNT=$(echo "$ORG_LIST_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data.get('data', [])))" 2>/dev/null)
  echo "Found $ORG_COUNT organizations"
else
  echo -e "${RED}❌ List Organizations FAILED${NC}"
  echo "$ORG_LIST_RESPONSE"
fi

if [ -n "$ORG_ID" ] && [ "$ORG_ID" != "None" ]; then
  echo -e "\n2.3 Get Organization by ID"
  ORG_GET_RESPONSE=$(curl -s -X GET "$BASE_URL/organizations/$ORG_ID" \
    -H "Authorization: Bearer $TOKEN")
  if echo "$ORG_GET_RESPONSE" | grep -q "id"; then
    echo -e "${GREEN}✅ Get Organization SUCCESS${NC}"
  else
    echo -e "${RED}❌ Get Organization FAILED${NC}"
    echo "$ORG_GET_RESPONSE"
  fi

  echo -e "\n2.4 Update Organization"
  ORG_UPDATE_RESPONSE=$(curl -s -X PUT "$BASE_URL/organizations/$ORG_ID" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name": "Updated Test Org"}')
  if echo "$ORG_UPDATE_RESPONSE" | grep -q "Updated Test Org"; then
    echo -e "${GREEN}✅ Update Organization SUCCESS${NC}"
  else
    echo -e "${RED}❌ Update Organization FAILED${NC}"
    echo "$ORG_UPDATE_RESPONSE"
  fi
fi

# ==========================================
# 3. ROLE MANAGEMENT
# ==========================================
echo -e "\n${YELLOW}=== 3. ROLE MANAGEMENT ===${NC}"

echo -e "\n3.1 List Roles"
ROLE_LIST_RESPONSE=$(curl -s -X GET "$BASE_URL/roles" \
  -H "Authorization: Bearer $TOKEN")
if echo "$ROLE_LIST_RESPONSE" | grep -q "id" || echo "$ROLE_LIST_RESPONSE" | grep -q "\[\]"; then
  echo -e "${GREEN}✅ List Roles SUCCESS${NC}"
  ROLE_COUNT=$(echo "$ROLE_LIST_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 0)" 2>/dev/null)
  echo "Found $ROLE_COUNT roles"
else
  echo -e "${RED}❌ List Roles FAILED${NC}"
  echo "$ROLE_LIST_RESPONSE"
fi

echo -e "\n3.2 Create Role"
# Super admin can create role without org_id (system role) or with org_id
# Try creating a role - super admin will create system role if no org_id
ROLE_CREATE_RESPONSE=$(curl -s -X POST $BASE_URL/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Manager",
    "description": "Test role for API testing",
    "is_default": false
  }')
ROLE_ID=$(echo $ROLE_CREATE_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null)
if [ -n "$ROLE_ID" ] && [ "$ROLE_ID" != "None" ] && [ "$ROLE_ID" != "" ]; then
  echo -e "${GREEN}✅ Create Role SUCCESS${NC}"
  echo "Role ID: $ROLE_ID"
else
  echo -e "${RED}❌ Create Role FAILED${NC}"
  echo "$ROLE_CREATE_RESPONSE"
  ROLE_ID=""
fi

if [ -n "$ORG_ID" ] && [ "$ORG_ID" != "None" ] && [ -z "$ROLE_ID" ]; then
  # If first attempt failed and we have org_id, try with org_id
  echo -e "\n3.2.1 Create Role (with org_id)"
  ROLE_CREATE_RESPONSE=$(curl -s -X POST $BASE_URL/roles \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"Test Manager Org\",
      \"description\": \"Test role for API testing with org\",
      \"org_id\": \"$ORG_ID\",
      \"is_default\": false
    }")
  ROLE_ID=$(echo $ROLE_CREATE_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null)
  if [ -n "$ROLE_ID" ] && [ "$ROLE_ID" != "None" ] && [ "$ROLE_ID" != "" ]; then
    echo -e "${GREEN}✅ Create Role SUCCESS${NC}"
    echo "Role ID: $ROLE_ID"
  else
    echo -e "${RED}❌ Create Role FAILED${NC}"
    echo "$ROLE_CREATE_RESPONSE"
    ROLE_ID=""
  fi
fi

if [ -n "$ROLE_ID" ] && [ "$ROLE_ID" != "None" ]; then

  if [ -n "$ROLE_ID" ] && [ "$ROLE_ID" != "None" ]; then
    echo -e "\n3.3 Get Role by ID"
    ROLE_GET_RESPONSE=$(curl -s -X GET "$BASE_URL/roles/$ROLE_ID" \
      -H "Authorization: Bearer $TOKEN")
    if echo "$ROLE_GET_RESPONSE" | grep -q "id"; then
      echo -e "${GREEN}✅ Get Role SUCCESS${NC}"
    else
      echo -e "${RED}❌ Get Role FAILED${NC}"
      echo "$ROLE_GET_RESPONSE"
    fi

    echo -e "\n3.4 Update Role"
    ROLE_UPDATE_RESPONSE=$(curl -s -X PUT "$BASE_URL/roles/$ROLE_ID" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"description": "Updated test role description"}')
    if echo "$ROLE_UPDATE_RESPONSE" | grep -q "Updated test role"; then
      echo -e "${GREEN}✅ Update Role SUCCESS${NC}"
    else
      echo -e "${RED}❌ Update Role FAILED${NC}"
      echo "$ROLE_UPDATE_RESPONSE"
    fi

    echo -e "\n3.5 Get Role Permissions"
    ROLE_PERM_RESPONSE=$(curl -s -X GET "$BASE_URL/roles/$ROLE_ID/permissions" \
      -H "Authorization: Bearer $TOKEN")
    # Check for array (empty [] or with items) - not null
    if echo "$ROLE_PERM_RESPONSE" | grep -q "resource" || echo "$ROLE_PERM_RESPONSE" | grep -qE "^\[\]" || echo "$ROLE_PERM_RESPONSE" | grep -qE "^\[.*\]"; then
      echo -e "${GREEN}✅ Get Role Permissions SUCCESS${NC}"
      PERM_COUNT=$(echo "$ROLE_PERM_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 0)" 2>/dev/null)
      echo "Found $PERM_COUNT permissions"
    elif echo "$ROLE_PERM_RESPONSE" | grep -q "null"; then
      echo -e "${RED}❌ Get Role Permissions FAILED - returned null instead of array${NC}"
      echo "$ROLE_PERM_RESPONSE"
    else
      echo -e "${RED}❌ Get Role Permissions FAILED${NC}"
      echo "$ROLE_PERM_RESPONSE"
    fi
  fi
fi

# ==========================================
# 4. PERMISSION MANAGEMENT
# ==========================================
echo -e "\n${YELLOW}=== 4. PERMISSION MANAGEMENT ===${NC}"

echo -e "\n4.1 List Permissions"
PERM_LIST_RESPONSE=$(curl -s -X GET "$BASE_URL/permissions" \
  -H "Authorization: Bearer $TOKEN")
if echo "$PERM_LIST_RESPONSE" | grep -q "resource"; then
  echo -e "${GREEN}✅ List Permissions SUCCESS${NC}"
  PERM_COUNT=$(echo "$PERM_LIST_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 0)" 2>/dev/null)
  echo "Found $PERM_COUNT permissions"
  
  # Get first permission ID for testing
  FIRST_PERM_ID=$(echo "$PERM_LIST_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data[0]['id'] if isinstance(data, list) and len(data) > 0 else '')" 2>/dev/null)
else
  echo -e "${RED}❌ List Permissions FAILED${NC}"
  echo "$PERM_LIST_RESPONSE"
  FIRST_PERM_ID=""
fi

# Assign permissions to role if we have both
if [ -n "$ROLE_ID" ] && [ "$ROLE_ID" != "None" ] && [ -n "$FIRST_PERM_ID" ]; then
  echo -e "\n4.2 Assign Permissions to Role"
  # Get a few permission IDs
  PERM_IDS=$(echo "$PERM_LIST_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); ids=[p['id'] for p in data[:3] if isinstance(data, list)]; print(','.join(ids))" 2>/dev/null)
  if [ -n "$PERM_IDS" ]; then
    PERM_IDS_ARRAY=$(echo "$PERM_IDS" | python3 -c "import sys; ids=sys.stdin.read().strip().split(','); print('[\"' + '\",\"'.join(ids) + '\"]')")
    ASSIGN_PERM_RESPONSE=$(curl -s -X POST "$BASE_URL/roles/$ROLE_ID/permissions" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"permission_ids\": $PERM_IDS_ARRAY}")
    if echo "$ASSIGN_PERM_RESPONSE" | grep -q "message" || echo "$ASSIGN_PERM_RESPONSE" | grep -q "success"; then
      echo -e "${GREEN}✅ Assign Permissions SUCCESS${NC}"
    else
      echo -e "${RED}❌ Assign Permissions FAILED${NC}"
      echo "$ASSIGN_PERM_RESPONSE"
    fi
  fi
fi

# ==========================================
# 5. USER MANAGEMENT
# ==========================================
echo -e "\n${YELLOW}=== 5. USER MANAGEMENT ===${NC}"

if [ -n "$ORG_ID" ] && [ "$ORG_ID" != "None" ]; then
  # Get org slug for user creation
  ORG_SLUG=$(echo "$ORG_CREATE_RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin).get('slug', ''))" 2>/dev/null)
  
  echo -e "\n5.1 Create User (with org_slug and role_name)"
  USER_CREATE_RESPONSE=$(curl -s -X POST $BASE_URL/users \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"email\": \"testuser$(date +%s)@example.com\",
      \"password\": \"password123\",
      \"first_name\": \"Test\",
      \"last_name\": \"User\",
      \"org_slug\": \"$ORG_SLUG\",
      \"role_name\": \"User\"
    }")
  USER_ID=$(echo $USER_CREATE_RESPONSE | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null)
  if [ -n "$USER_ID" ] && [ "$USER_ID" != "None" ]; then
    echo -e "${GREEN}✅ Create User SUCCESS${NC}"
    echo "User ID: $USER_ID"
  else
    echo -e "${RED}❌ Create User FAILED${NC}"
    echo "$USER_CREATE_RESPONSE"
    USER_ID=""
  fi

  if [ -n "$USER_ID" ] && [ "$USER_ID" != "None" ]; then
    echo -e "\n5.2 Get User by ID"
    USER_GET_RESPONSE=$(curl -s -X GET "$BASE_URL/users/$USER_ID" \
      -H "Authorization: Bearer $TOKEN")
    if echo "$USER_GET_RESPONSE" | grep -q "id"; then
      echo -e "${GREEN}✅ Get User SUCCESS${NC}"
    else
      echo -e "${RED}❌ Get User FAILED${NC}"
      echo "$USER_GET_RESPONSE"
    fi

    echo -e "\n5.3 Update User"
    USER_UPDATE_RESPONSE=$(curl -s -X PUT "$BASE_URL/users/$USER_ID" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"first_name": "Updated", "last_name": "Name"}')
    if echo "$USER_UPDATE_RESPONSE" | grep -q "Updated"; then
      echo -e "${GREEN}✅ Update User SUCCESS${NC}"
    else
      echo -e "${RED}❌ Update User FAILED${NC}"
      echo "$USER_UPDATE_RESPONSE"
    fi

    echo -e "\n5.4 Get User Permissions"
    USER_PERM_RESPONSE=$(curl -s -X GET "$BASE_URL/users/$USER_ID/permissions" \
      -H "Authorization: Bearer $TOKEN")
    # Check for array (empty [] or with items) - not null
    if echo "$USER_PERM_RESPONSE" | grep -q "resource" || echo "$USER_PERM_RESPONSE" | grep -qE "^\[\]" || echo "$USER_PERM_RESPONSE" | grep -qE "^\[.*\]"; then
      echo -e "${GREEN}✅ Get User Permissions SUCCESS${NC}"
      PERM_COUNT=$(echo "$USER_PERM_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 0)" 2>/dev/null)
      echo "Found $PERM_COUNT permissions"
    elif echo "$USER_PERM_RESPONSE" | grep -q "null"; then
      echo -e "${RED}❌ Get User Permissions FAILED - returned null instead of array${NC}"
      echo "$USER_PERM_RESPONSE"
    else
      echo -e "${RED}❌ Get User Permissions FAILED${NC}"
      echo "$USER_PERM_RESPONSE"
    fi

    if [ -n "$ROLE_ID" ] && [ "$ROLE_ID" != "None" ]; then
      echo -e "\n5.5 Assign Role to User"
      ASSIGN_ROLE_RESPONSE=$(curl -s -X POST "$BASE_URL/users/$USER_ID/roles" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"role_id\": \"$ROLE_ID\"}")
      if echo "$ASSIGN_ROLE_RESPONSE" | grep -q "message" || echo "$ASSIGN_ROLE_RESPONSE" | grep -q "success"; then
        echo -e "${GREEN}✅ Assign Role to User SUCCESS${NC}"
      else
        echo -e "${RED}❌ Assign Role to User FAILED${NC}"
        echo "$ASSIGN_ROLE_RESPONSE"
      fi

      echo -e "\n5.6 Remove Role from User"
      REMOVE_ROLE_RESPONSE=$(curl -s -X DELETE "$BASE_URL/users/$USER_ID/roles/$ROLE_ID" \
        -H "Authorization: Bearer $TOKEN")
      if echo "$REMOVE_ROLE_RESPONSE" | grep -q "message" || echo "$REMOVE_ROLE_RESPONSE" | grep -q "success"; then
        echo -e "${GREEN}✅ Remove Role from User SUCCESS${NC}"
      else
        echo -e "${RED}❌ Remove Role from User FAILED${NC}"
        echo "$REMOVE_ROLE_RESPONSE"
      fi
    fi

    echo -e "\n5.7 List Users"
    USER_LIST_RESPONSE=$(curl -s -X GET "$BASE_URL/users?page=1&limit=5" \
      -H "Authorization: Bearer $TOKEN")
    if echo "$USER_LIST_RESPONSE" | grep -q "data"; then
      echo -e "${GREEN}✅ List Users SUCCESS${NC}"
    else
      echo -e "${RED}❌ List Users FAILED${NC}"
      echo "$USER_LIST_RESPONSE"
    fi
  fi
fi

# ==========================================
# 6. ADMIN ENDPOINTS
# ==========================================
echo -e "\n${YELLOW}=== 6. ADMIN ENDPOINTS (Super Admin Only) ===${NC}"

echo -e "\n6.1 List All Users (Admin)"
ADMIN_USERS_RESPONSE=$(curl -s -X GET "$BASE_URL/admin/users?page=1&limit=5" \
  -H "Authorization: Bearer $TOKEN")
if echo "$ADMIN_USERS_RESPONSE" | grep -q "data"; then
  echo -e "${GREEN}✅ Admin List Users SUCCESS${NC}"
else
  echo -e "${RED}❌ Admin List Users FAILED${NC}"
  echo "$ADMIN_USERS_RESPONSE"
fi

echo -e "\n6.2 List All Organizations (Admin)"
ADMIN_ORGS_RESPONSE=$(curl -s -X GET "$BASE_URL/admin/organizations?page=1&limit=5" \
  -H "Authorization: Bearer $TOKEN")
if echo "$ADMIN_ORGS_RESPONSE" | grep -q "data"; then
  echo -e "${GREEN}✅ Admin List Organizations SUCCESS${NC}"
else
  echo -e "${RED}❌ Admin List Organizations FAILED${NC}"
  echo "$ADMIN_ORGS_RESPONSE"
fi

# ==========================================
# 7. HEALTH CHECK
# ==========================================
echo -e "\n${YELLOW}=== 7. HEALTH CHECK ===${NC}"

HEALTH_RESPONSE=$(curl -s -X GET "http://localhost:8080/health")
if echo "$HEALTH_RESPONSE" | grep -q "ok" || echo "$HEALTH_RESPONSE" | grep -q "status"; then
  echo -e "${GREEN}✅ Health Check SUCCESS${NC}"
else
  echo -e "${RED}❌ Health Check FAILED${NC}"
  echo "$HEALTH_RESPONSE"
fi

# ==========================================
# SUMMARY
# ==========================================
echo -e "\n${YELLOW}=========================================="
echo "TEST SUMMARY"
echo "==========================================${NC}"
echo ""
echo "All API endpoints have been tested."
echo "Check the results above for any failures."
echo ""
echo "Note: Some tests may fail if:"
echo "  - Required permissions are missing"
echo "  - Resources don't exist"
echo "  - Server is not running"
echo ""

