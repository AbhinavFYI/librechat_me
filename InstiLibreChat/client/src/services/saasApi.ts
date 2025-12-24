/**
 * API Service for insti-inquora backend
 * 
 * This service connects to the insti-inquora API (Go backend) through the proxy.
 * All requests are sent to /api/v1/* which routes to:
 *   - insti-inquora backend (port 3001) for most API calls
 *   - Handles authentication, documents, folders, users, organizations, etc.
 * 
 * The service handles both response formats:
 *   - insti-inquora: { code: 200, s: "ok", data: {...}, message: "..." }
 *   - Legacy saas-api: { access_token, refresh_token, user, ... } (flat structure)
 * 
 */

const API_BASE_URL = '/api/v1';

function getAuthToken(): string | null {
  return localStorage.getItem('access_token');
}

function getAuthHeaders(): HeadersInit {
  const token = getAuthToken();
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

/**
 * Normalize API response structure
 * Handles both response formats:
 * 1. insti-inquora: { code: 200, s: "ok", data: {...}, message: "..." }
 * 2. saas-api: { access_token, refresh_token, user, ... } (flat structure)
 */
function normalizeResponse<T>(data: any): T {
  // If response has 'data' field (insti-inquora format), extract it
  if (data && typeof data === 'object' && 'data' in data) {
    return data.data as T;
  }
  // Otherwise return as-is (saas-api format)
  return data as T;
}

async function handleResponse<T>(response: Response, originalRequest?: { url: string; method: string; body?: string }, retry = true): Promise<T> {
  if (!response.ok) {
    // Handle 401 Unauthorized - try to refresh token
    if (response.status === 401 && retry && originalRequest) {
      const refreshToken = localStorage.getItem('refresh_token');
      if (refreshToken) {
        try {
          const refreshResponse = await fetch(`${API_BASE_URL}/auth/refresh`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({ refresh_token: refreshToken }),
          });

          if (refreshResponse.ok) {
            const refreshData: any = await refreshResponse.json();
            // Handle both response formats for access_token
            const normalizedRefreshData = normalizeResponse<any>(refreshData);
            if (normalizedRefreshData.access_token) {
              localStorage.setItem('access_token', normalizedRefreshData.access_token);
              if (normalizedRefreshData.refresh_token) {
                localStorage.setItem('refresh_token', normalizedRefreshData.refresh_token);
              }
              // Retry the original request with new token
              const retryResponse = await fetch(originalRequest.url, {
                method: originalRequest.method as any,
                headers: {
                  ...getAuthHeaders(),
                  'Content-Type': 'application/json',
                },
                body: originalRequest.body,
              });
              return handleResponse(retryResponse, originalRequest, false); // Don't retry again
            }
          } else {
            // Refresh token failed - clear tokens and redirect to login
            console.error('Token refresh failed with status:', refreshResponse.status);
            const errorData = await refreshResponse.json().catch(() => ({}));
            console.error('Refresh error details:', errorData);
            
            // Clear tokens
            localStorage.removeItem('access_token');
            localStorage.removeItem('refresh_token');
            
            // Don't throw error if we're already handling redirect
            // This prevents showing error messages when user is being redirected to login
            if (window.location.pathname.includes('/login')) {
              return Promise.reject(new Error(errorData.message || errorData.error || 'Session expired. Please login again.'));
            }
            
            // Only redirect if we're not already on the login page
            window.location.href = '/login';
            return Promise.reject(new Error(errorData.message || errorData.error || 'Session expired. Please login again.'));
          }
        } catch (refreshError) {
          console.error('Token refresh failed:', refreshError);
          // Clear tokens on any error
          localStorage.removeItem('access_token');
          localStorage.removeItem('refresh_token');
          // Only redirect if we're not already on the login page
          if (!window.location.pathname.includes('/login')) {
            window.location.href = '/login';
          }
          // Fall through to throw original error
        }
      }
    }

    const error = await response.json().catch(() => ({ message: 'Request failed' }));
    // Handle both 'message' and 'error' fields
    throw new Error(error.message || error.error || `HTTP error! status: ${response.status}`);
  }
  
  const data = await response.json();
  return normalizeResponse<T>(data);
}

export const saasApi = {
  // Auth endpoints
  async refreshToken(refreshToken: string) {
    const response = await fetch(`${API_BASE_URL}/auth/refresh`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    return handleResponse(response);
  },

  async getMe() {
    const url = `${API_BASE_URL}/auth/me`;
    const response = await fetch(url, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response, { url, method: 'GET' });
  },

  // Organizations
  async getOrganizations(isSuperAdmin: boolean, orgId?: string) {
    // Both super admin and org admin use the same list endpoint
    // Backend distinguishes based on JWT claims (is_super_admin, org_id)
    // Super admin sees all orgs, org admin sees only their org
    const url = `${API_BASE_URL}/organizations?limit=1000`;
    const response = await fetch(url, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async createOrganization(data: any) {
    const response = await fetch(`${API_BASE_URL}/organizations`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async updateOrganization(id: string, data: any) {
    const response = await fetch(`${API_BASE_URL}/organizations/${id}`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async deleteOrganization(id: string) {
    const response = await fetch(`${API_BASE_URL}/organizations/${id}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  // Users
  async getUsers(isSuperAdmin: boolean, orgId?: string) {
    // Both super admin and org admin use the same endpoint
    // Backend distinguishes based on JWT claims (is_super_admin, org_id)
    let url = `${API_BASE_URL}/users?limit=1000`;
    if (isSuperAdmin && orgId) {
      url += `&org_id=${orgId}`;
    }
    const response = await fetch(url, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async createUser(data: any) {
    const response = await fetch(`${API_BASE_URL}/users`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async updateUser(id: string, data: any) {
    const response = await fetch(`${API_BASE_URL}/users/${id}`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async deleteUser(id: string) {
    const response = await fetch(`${API_BASE_URL}/users/${id}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async getUserPermissions(id: string) {
    const response = await fetch(`${API_BASE_URL}/users/${id}/permissions`, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async assignRoleToUser(userId: string, roleId: string) {
    const response = await fetch(`${API_BASE_URL}/users/${userId}/roles`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ role_id: roleId }),
    });
    return handleResponse(response);
  },

  async removeRoleFromUser(userId: string, roleId: string) {
    const response = await fetch(`${API_BASE_URL}/users/${userId}/roles/${roleId}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  // Roles
  async getRoles(orgFilterId?: string) {
    let url = `${API_BASE_URL}/roles`;
    if (orgFilterId) {
      url += `?org_id=${orgFilterId}`;
    }
    const response = await fetch(url, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async createRole(data: any) {
    const response = await fetch(`${API_BASE_URL}/roles`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async updateRole(id: string, data: any) {
    const response = await fetch(`${API_BASE_URL}/roles/${id}`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async deleteRole(id: string) {
    const response = await fetch(`${API_BASE_URL}/roles/${id}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async getRolePermissions(id: string) {
    const response = await fetch(`${API_BASE_URL}/roles/${id}/permissions`, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async assignPermissionsToRole(roleId: string, permissionIds: string[]) {
    const response = await fetch(`${API_BASE_URL}/roles/${roleId}/permissions`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ permission_ids: permissionIds }),
    });
    return handleResponse(response);
  },

  // Permissions
  async getPermissions() {
    const response = await fetch(`${API_BASE_URL}/permissions`, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  // Templates
  async getTemplates() {
    const url = `${API_BASE_URL}/templates?limit=1000`;
    const response = await fetch(url, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response, { url, method: 'GET' });
  },

  async createTemplate(data: any) {
    const url = `${API_BASE_URL}/templates`;
    const method = 'POST';
    const body = JSON.stringify(data);
    const response = await fetch(url, {
      method,
      headers: getAuthHeaders(),
      body,
    });
    return handleResponse(response, { url, method, body });
  },

  async updateTemplate(id: string, data: any) {
    const url = `${API_BASE_URL}/templates/${id}`;
    const method = 'PUT';
    const body = JSON.stringify(data);
    const response = await fetch(url, {
      method,
      headers: getAuthHeaders(),
      body,
    });
    return handleResponse(response, { url, method, body });
  },

  async deleteTemplate(id: string) {
    const url = `${API_BASE_URL}/templates/${id}`;
    const method = 'DELETE';
    const response = await fetch(url, {
      method,
      headers: getAuthHeaders(),
    });
    return handleResponse(response, { url, method });
  },

  // Personas
  async getPersonas() {
    const url = `${API_BASE_URL}/personas?limit=1000`;
    const response = await fetch(url, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response, { url, method: 'GET' });
  },

  async createPersona(data: any) {
    const url = `${API_BASE_URL}/personas`;
    const method = 'POST';
    const body = JSON.stringify(data);
    const response = await fetch(url, {
      method,
      headers: getAuthHeaders(),
      body,
    });
    return handleResponse(response, { url, method, body });
  },

  async updatePersona(id: string, data: any) {
    const url = `${API_BASE_URL}/personas/${id}`;
    const method = 'PUT';
    const body = JSON.stringify(data);
    const response = await fetch(url, {
      method,
      headers: getAuthHeaders(),
      body,
    });
    return handleResponse(response, { url, method, body });
  },

  async deletePersona(id: string) {
    const url = `${API_BASE_URL}/personas/${id}`;
    const method = 'DELETE';
    const response = await fetch(url, {
      method,
      headers: getAuthHeaders(),
    });
    return handleResponse(response, { url, method });
  },

  // Folders
  async getFolders(parentId?: string) {
    const url = parentId 
      ? `${API_BASE_URL}/folders?parent_id=${parentId}`
      : `${API_BASE_URL}/folders`;
    const response = await fetch(url, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async getFolderTree(orgId?: string | null) {
    let url = `${API_BASE_URL}/folders/tree`;
    if (orgId) {
      url += `?org_id=${orgId}`;
    }
    const response = await fetch(url, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async getFolder(id: string) {
    const response = await fetch(`${API_BASE_URL}/folders/${id}`, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async downloadFile(id: number): Promise<Blob> {
    const token = getAuthToken();
    const headers: HeadersInit = {};
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    const url = `${API_BASE_URL}/documents/${id}/download`;
    console.log('Downloading file:', url);
    
    const response = await fetch(url, {
      method: 'GET',
      headers,
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      console.error('Download failed:', {
        status: response.status,
        statusText: response.statusText,
        error: errorText,
      });
      throw new Error(`Failed to download file: ${response.statusText} - ${errorText}`);
    }
    
    const contentType = response.headers.get('content-type');
    console.log('Download response:', {
      status: response.status,
      contentType,
      contentLength: response.headers.get('content-length'),
    });
    
    const blob = await response.blob();
    console.log('Downloaded blob:', {
      size: blob.size,
      type: blob.type,
    });
    
    // Validate PDF blob
    if (blob.type.includes('pdf') || blob.type.includes('octet-stream')) {
      const arrayBuffer = await blob.arrayBuffer();
      const bytes = new Uint8Array(arrayBuffer);
      if (bytes.length >= 4) {
        const header = String.fromCharCode(...bytes.slice(0, 4));
        console.log('File header:', header);
        if (header === '%PDF') {
          console.log('Valid PDF detected');
        } else {
          console.warn('Warning: File does not start with PDF header:', header);
        }
      }
    }
    
    return blob;
  },

  async createFolder(data: any) {
    const response = await fetch(`${API_BASE_URL}/folders`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async updateFolder(id: string, data: any) {
    const response = await fetch(`${API_BASE_URL}/folders/${id}`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async deleteFolder(id: string) {
    const response = await fetch(`${API_BASE_URL}/folders/${id}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async getFolderPermissions(id: string) {
    const response = await fetch(`${API_BASE_URL}/folders/${id}/permissions`, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async assignFolderPermission(folderId: string, data: any) {
    const response = await fetch(`${API_BASE_URL}/folders/${folderId}/permissions`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async removeFolderPermission(folderId: string, roleId: string) {
    const response = await fetch(`${API_BASE_URL}/folders/${folderId}/permissions/${roleId}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  // Files (now using documents API)
  async getFiles(folderId?: string, page = 1, limit = 1000) {
    const url = folderId
      ? `${API_BASE_URL}/documents?folder_id=${folderId}&page=${page}&limit=${limit}`
      : `${API_BASE_URL}/documents?page=${page}&limit=${limit}`;
    const response = await fetch(url, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async getFile(id: number) {
    const response = await fetch(`${API_BASE_URL}/documents/${id}`, {
      method: 'GET',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },

  async createFile(data: any) {
    // Files are created via document upload
    const response = await fetch(`${API_BASE_URL}/documents/upload`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async uploadFile(file: File, folderId?: string, orgId?: string | null) {
    const formData = new FormData();
    formData.append('file', file);
    if (folderId) {
      formData.append('folder_id', folderId);
    }
    if (orgId) {
      formData.append('org_id', orgId);
    }

    const token = getAuthToken();
    const headers: HeadersInit = {};
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(`${API_BASE_URL}/documents/upload`, {
      method: 'POST',
      headers,
      body: formData,
    });
    return handleResponse(response);
  },

  async updateFile(id: number, data: any) {
    const response = await fetch(`${API_BASE_URL}/documents/${id}`, {
      method: 'PUT',
      headers: getAuthHeaders(),
      body: JSON.stringify(data),
    });
    return handleResponse(response);
  },

  async deleteFile(id: number) {
    const response = await fetch(`${API_BASE_URL}/documents/${id}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    });
    return handleResponse(response);
  },
};

