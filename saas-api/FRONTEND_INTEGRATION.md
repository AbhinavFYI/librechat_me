# Frontend Integration Guide

## Token Management in Frontend Applications

### 1. Login and Store Tokens

#### React Example
```javascript
// Login component
import { useState } from 'react';

function Login() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const handleLogin = async (e) => {
    e.preventDefault();
    
    try {
      const response = await fetch('http://localhost:8080/api/v1/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }),
      });

      const data = await response.json();

      if (response.ok && data.access_token) {
        // Store tokens
        localStorage.setItem('access_token', data.access_token);
        localStorage.setItem('refresh_token', data.refresh_token);
        localStorage.setItem('user', JSON.stringify(data.user));
        
        // Redirect to dashboard
        window.location.href = '/dashboard';
      } else {
        alert(data.message || 'Login failed');
      }
    } catch (error) {
      console.error('Login error:', error);
      alert('Login failed. Please try again.');
    }
  };

  return (
    <form onSubmit={handleLogin}>
      <input
        type="email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        placeholder="Email"
        required
      />
      <input
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        placeholder="Password"
        required
      />
      <button type="submit">Login</button>
    </form>
  );
}
```

### 2. Create API Client with Token

#### Using Fetch API
```javascript
// api.js
const API_BASE_URL = 'http://localhost:8080/api/v1';

// Get token from storage
function getToken() {
  return localStorage.getItem('access_token');
}

// Make authenticated request
async function apiRequest(endpoint, options = {}) {
  const token = getToken();
  
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
      ...options.headers,
    },
  });

  // Handle token expiration
  if (response.status === 401) {
    // Try to refresh token
    const refreshed = await refreshToken();
    if (refreshed) {
      // Retry original request
      return apiRequest(endpoint, options);
    } else {
      // Redirect to login
      localStorage.clear();
      window.location.href = '/login';
      throw new Error('Unauthorized');
    }
  }

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.message || 'Request failed');
  }

  return response.json();
}

// API methods
export const api = {
  // Auth
  login: (email, password) => 
    fetch(`${API_BASE_URL}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    }).then(res => res.json()),

  getCurrentUser: () => apiRequest('/auth/me'),
  logout: (refreshToken) => 
    apiRequest('/auth/logout', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: refreshToken }),
    }),

  // Users
  listUsers: (page = 1, limit = 20) => 
    apiRequest(`/users?page=${page}&limit=${limit}`),
  
  getUser: (id) => apiRequest(`/users/${id}`),
  
  createUser: (userData) => 
    apiRequest('/users', {
      method: 'POST',
      body: JSON.stringify(userData),
    }),
  
  updateUser: (id, userData) => 
    apiRequest(`/users/${id}`, {
      method: 'PUT',
      body: JSON.stringify(userData),
    }),
  
  deleteUser: (id) => 
    apiRequest(`/users/${id}`, {
      method: 'DELETE',
    }),
  
  getUserPermissions: (id) => 
    apiRequest(`/users/${id}/permissions`),

  // Organizations
  listOrganizations: (page = 1, limit = 20) => 
    apiRequest(`/organizations?page=${page}&limit=${limit}`),
  
  getOrganization: (id) => apiRequest(`/organizations/${id}`),
  
  createOrganization: (orgData) => 
    apiRequest('/organizations', {
      method: 'POST',
      body: JSON.stringify(orgData),
    }),
  
  updateOrganization: (id, orgData) => 
    apiRequest(`/organizations/${id}`, {
      method: 'PUT',
      body: JSON.stringify(orgData),
    }),
  
  deleteOrganization: (id) => 
    apiRequest(`/organizations/${id}`, {
      method: 'DELETE',
    }),
};

// Refresh token function
async function refreshToken() {
  const refreshToken = localStorage.getItem('refresh_token');
  if (!refreshToken) return false;

  try {
    const response = await fetch(`${API_BASE_URL}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });

    const data = await response.json();
    if (data.access_token) {
      localStorage.setItem('access_token', data.access_token);
      return true;
    }
  } catch (error) {
    console.error('Token refresh failed:', error);
  }

  return false;
}
```

#### Using Axios (Recommended)
```javascript
// api.js
import axios from 'axios';

const api = axios.create({
  baseURL: 'http://localhost:8080/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor - add token
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('access_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor - handle 401 and refresh
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // If 401 and haven't retried yet
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;

      try {
        const refreshToken = localStorage.getItem('refresh_token');
        const response = await axios.post(
          'http://localhost:8080/api/v1/auth/refresh',
          { refresh_token: refreshToken }
        );

        const { access_token } = response.data;
        localStorage.setItem('access_token', access_token);

        // Retry original request
        originalRequest.headers.Authorization = `Bearer ${access_token}`;
        return api(originalRequest);
      } catch (refreshError) {
        // Refresh failed - logout
        localStorage.clear();
        window.location.href = '/login';
        return Promise.reject(refreshError);
      }
    }

    return Promise.reject(error);
  }
);

// API methods
export const authAPI = {
  login: (email, password) => 
    api.post('/auth/login', { email, password }),
  
  getCurrentUser: () => api.get('/auth/me'),
  
  logout: (refreshToken) => 
    api.post('/auth/logout', { refresh_token: refreshToken }),
  
  refresh: (refreshToken) => 
    api.post('/auth/refresh', { refresh_token: refreshToken }),
};

export const usersAPI = {
  list: (page = 1, limit = 20) => 
    api.get(`/users?page=${page}&limit=${limit}`),
  
  get: (id) => api.get(`/users/${id}`),
  
  create: (userData) => api.post('/users', userData),
  
  update: (id, userData) => api.put(`/users/${id}`, userData),
  
  delete: (id) => api.delete(`/users/${id}`),
  
  getPermissions: (id) => api.get(`/users/${id}/permissions`),
};

export const organizationsAPI = {
  list: (page = 1, limit = 20) => 
    api.get(`/organizations?page=${page}&limit=${limit}`),
  
  get: (id) => api.get(`/organizations/${id}`),
  
  create: (orgData) => api.post('/organizations', orgData),
  
  update: (id, orgData) => api.put(`/organizations/${id}`, orgData),
  
  delete: (id) => api.delete(`/organizations/${id}`),
};

export default api;
```

### 3. React Hook for Authentication

```javascript
// useAuth.js
import { useState, useEffect, createContext, useContext } from 'react';
import { authAPI } from './api';

const AuthContext = createContext();

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Check if user is logged in on mount
    const token = localStorage.getItem('access_token');
    if (token) {
      loadUser();
    } else {
      setLoading(false);
    }
  }, []);

  const loadUser = async () => {
    try {
      const response = await authAPI.getCurrentUser();
      setUser(response);
    } catch (error) {
      // Token invalid, clear it
      localStorage.removeItem('access_token');
      localStorage.removeItem('refresh_token');
    } finally {
      setLoading(false);
    }
  };

  const login = async (email, password) => {
    try {
      const response = await authAPI.login(email, password);
      const { access_token, refresh_token, user } = response.data;
      
      localStorage.setItem('access_token', access_token);
      localStorage.setItem('refresh_token', refresh_token);
      setUser(user);
      
      return { success: true };
    } catch (error) {
      return { 
        success: false, 
        error: error.response?.data?.message || 'Login failed' 
      };
    }
  };

  const logout = async () => {
    const refreshToken = localStorage.getItem('refresh_token');
    if (refreshToken) {
      try {
        await authAPI.logout(refreshToken);
      } catch (error) {
        console.error('Logout error:', error);
      }
    }
    
    localStorage.clear();
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, loadUser }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
```

### 4. Protected Route Component

```javascript
// ProtectedRoute.jsx
import { Navigate } from 'react-router-dom';
import { useAuth } from './useAuth';

function ProtectedRoute({ children }) {
  const { user, loading } = useAuth();

  if (loading) {
    return <div>Loading...</div>;
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  return children;
}
```

### 5. Usage Example

```javascript
// App.jsx
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AuthProvider } from './useAuth';
import { ProtectedRoute } from './ProtectedRoute';
import Login from './Login';
import Dashboard from './Dashboard';

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <Dashboard />
              </ProtectedRoute>
            }
          />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}
```

### 6. Making API Calls in Components

```javascript
// UsersList.jsx
import { useState, useEffect } from 'react';
import { usersAPI } from './api';

function UsersList() {
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);

  useEffect(() => {
    loadUsers();
  }, [page]);

  const loadUsers = async () => {
    try {
      setLoading(true);
      const response = await usersAPI.list(page, 20);
      setUsers(response.data.data);
    } catch (error) {
      console.error('Failed to load users:', error);
    } finally {
      setLoading(false);
    }
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div>
      <h1>Users</h1>
      {users.map(user => (
        <div key={user.id}>
          {user.email} - {user.full_name}
        </div>
      ))}
      <button onClick={() => setPage(page + 1)}>Next Page</button>
    </div>
  );
}
```

---

## Complete cURL Reference

### Authentication
```bash
# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"superadmin@yourapp.com","password":"superadmin123"}'

# Get current user
curl -X GET http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer YOUR_TOKEN"

# Refresh token
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"YOUR_REFRESH_TOKEN"}'

# Logout
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"YOUR_REFRESH_TOKEN"}'
```

### Users
```bash
# List users
curl -X GET "http://localhost:8080/api/v1/users?page=1&limit=20" \
  -H "Authorization: Bearer YOUR_TOKEN"

# Get user
curl -X GET http://localhost:8080/api/v1/users/USER_ID \
  -H "Authorization: Bearer YOUR_TOKEN"

# Create user
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"pass123","first_name":"John","last_name":"Doe"}'

# Update user
curl -X PUT http://localhost:8080/api/v1/users/USER_ID \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"first_name":"Jane","last_name":"Smith"}'

# Delete user
curl -X DELETE http://localhost:8080/api/v1/users/USER_ID \
  -H "Authorization: Bearer YOUR_TOKEN"

# Get user permissions
curl -X GET http://localhost:8080/api/v1/users/USER_ID/permissions \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Organizations
```bash
# List organizations
curl -X GET "http://localhost:8080/api/v1/organizations?page=1&limit=20" \
  -H "Authorization: Bearer YOUR_TOKEN"

# Get organization
curl -X GET http://localhost:8080/api/v1/organizations/ORG_ID \
  -H "Authorization: Bearer YOUR_TOKEN"

# Create organization
curl -X POST http://localhost:8080/api/v1/organizations \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Acme Corp","slug":"acme-corp"}'

# Update organization
curl -X PUT http://localhost:8080/api/v1/organizations/ORG_ID \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Updated Name"}'

# Delete organization
curl -X DELETE http://localhost:8080/api/v1/organizations/ORG_ID \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

## Quick Reference

### Token Storage
- **Access Token**: Store in `localStorage` or `sessionStorage`
- **Refresh Token**: Store in `localStorage` (for persistence)
- **User Data**: Store in `localStorage` or state

### Token Usage
- **Header Format**: `Authorization: Bearer YOUR_ACCESS_TOKEN`
- **Expiration**: Access token expires in 15 minutes
- **Refresh**: Use refresh token when access token expires (401 response)

### Error Handling
- **401 Unauthorized**: Token expired or invalid → Refresh token or redirect to login
- **403 Forbidden**: User doesn't have permission → Show error message
- **404 Not Found**: Resource doesn't exist → Show error message
- **400 Bad Request**: Validation error → Show error message

