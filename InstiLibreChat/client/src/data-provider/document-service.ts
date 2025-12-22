/**
 * Document service for document upload API
 * Handles document upload to the external document service
 */

// External document service URL - use relative URL so it works on any domain
const DOCUMENT_API_BASE = '/api/v1';

export interface DocumentUploadResponse {
  code?: number;
  message?: string;
  s?: string;
  data?: any;
}

export interface DocumentListItem {
  document_id: number;
  name: string;
  file_path: string;
  status: string;
  uploaded_at: string | null;
  processed_at?: string | null;
}

export interface DocumentListResponse {
  code?: number;
  message?: string;
  s?: string;
  data: {
    documents?: DocumentListItem[];
    total_count?: number;
    page?: number;
    limit?: number;
  } | DocumentListItem[];
}

function getAuthToken(): string | null {
  return localStorage.getItem('access_token');
}

function getAuthHeaders(): HeadersInit {
  const token = getAuthToken();
  const headers: HeadersInit = {};
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

/**
 * Upload a document to the document service
 */
export const uploadDocument = async (
  file: File,
  owner?: string,
  orgId?: string
): Promise<DocumentUploadResponse> => {
  const ownerValue = owner || 'default_user';
  const formData = new FormData();
  formData.append('file', file);
  formData.append('db_backend', 'docling_postgres');
  formData.append('owner', ownerValue);
  
  // Add org_id if provided (optional for superadmins, required for org users)
  if (orgId) {
    formData.append('org_id', orgId);
  } else {
    // Try to get org_id from user data in localStorage
    const userDataStr = localStorage.getItem('user');
    if (userDataStr) {
      try {
        const userData = JSON.parse(userDataStr);
        // Only add org_id if user is not a superadmin or if they have an org_id
        if (userData.org_id) {
          formData.append('org_id', userData.org_id);
        }
        // Superadmins without org_id can upload without it
      } catch (e) {
        console.warn('Failed to parse user data for org_id:', e);
      }
    }
  }

  try {
    const response = await fetch(`${DOCUMENT_API_BASE}/documents/upload`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: formData,
      credentials: 'include', // Include cookies for auth
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({ message: 'Upload failed' }));
      throw new Error(errorData.message || `Upload failed: ${response.statusText}`);
    }

    return await response.json();
  } catch (error) {
    console.error('Document upload error:', error);
    throw error;
  }
};

/**
 * Fetch list of available documents from the external service
 */
export const fetchDocuments = async (): Promise<DocumentListResponse> => {
  try {
    // API endpoint: /api/v1/documents
    const response = await fetch(`${DOCUMENT_API_BASE}/documents`, {
      method: 'GET',
      headers: {
        ...getAuthHeaders(),
        'Content-Type': 'application/json',
      },
      credentials: 'include', // Include cookies for auth
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({ message: 'Fetch failed' }));
      throw new Error(errorData.message || `Fetch failed: ${response.statusText}`);
    }

    const data = await response.json();
    console.log('Documents API response:', data);
    
    // Handle different response structures
    // If data is directly an array, wrap it
    if (Array.isArray(data)) {
      return { data, code: 200, s: 'ok' };
    }
    
    // If data has a data property that is an array
    if (data.data && Array.isArray(data.data)) {
      return data;
    }
    
    // If data has documents property
    if (data.documents && Array.isArray(data.documents)) {
      return { data: data.documents, code: data.code || 200, s: data.s || 'ok' };
    }
    
    // Return as-is (might have different structure)
    return data;
  } catch (error) {
    console.error('Document fetch error:', error);
    throw error;
  }
};

