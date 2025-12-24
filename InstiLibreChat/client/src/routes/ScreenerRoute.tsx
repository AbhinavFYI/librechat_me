import { useState, useEffect, useRef } from 'react';
import { Search, Heart, Info, History, Trash2, ArrowLeft } from 'lucide-react';
import { HoverCard, HoverCardTrigger, HoverCardContent, HoverCardPortal, useToastContext } from '@librechat/client';

interface HeaderColumn {
  name: string;
  data_type: string;
  allow_sort?: boolean;
  is_formated_to_klcr?: boolean;
}

interface ScreeningResult {
  [key: string]: any;
}

// Helper function to clean string values by removing all surrounding quotes
const cleanStringValue = (value: string): string => {
  if (!value) return '';
  
  let cleaned = value.trim();
  
  // Keep removing outer quotes (both single and double) until none are left
  while (cleaned.length > 0) {
    const firstChar = cleaned[0];
    const lastChar = cleaned[cleaned.length - 1];
    
    // Check if surrounded by quotes (single or double)
    if ((firstChar === "'" || firstChar === '"') && (lastChar === "'" || lastChar === '"')) {
      cleaned = cleaned.slice(1, -1).trim();
    } else {
      break;
    }
  }
  
  return cleaned;
};

export default function ScreenerRoute() {
  const { showToast } = useToastContext();
  // Load state from localStorage on mount
  const [activeTab, setActiveTab] = useState<'create' | 'saved'>(() => {
    const saved = localStorage.getItem('screener_activeTab');
    return (saved as 'create' | 'saved') || 'create';
  });
  const [searchQuery, setSearchQuery] = useState(() => {
    return localStorage.getItem('screener_searchQuery') || '';
  });
  const [results, setResults] = useState<ScreeningResult[]>(() => {
    const saved = localStorage.getItem('screener_results');
    return saved ? JSON.parse(saved) : [];
  });
  const [headers, setHeaders] = useState<HeaderColumn[]>(() => {
    const saved = localStorage.getItem('screener_headers');
    return saved ? JSON.parse(saved) : [];
  });
  const [universe, setUniverse] = useState<string>(() => {
    return localStorage.getItem('screener_universe') || '';
  });
  const [exchange, setExchange] = useState<string>(() => {
    return localStorage.getItem('screener_exchange') || '';
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [explainer, setExplainer] = useState<string>(() => {
    return localStorage.getItem('screener_explainer') || '';
  });
  const [query, setQuery] = useState<string>(() => {
    return localStorage.getItem('screener_query') || '';
  });
  const [savedScreeners, setSavedScreeners] = useState<any[]>(() => {
    // Load cached screeners from localStorage for instant display
    const cached = localStorage.getItem('cached_saved_screeners');
    return cached ? JSON.parse(cached) : [];
  });
  const [isSaved, setIsSaved] = useState(false);
  const [saving, setSaving] = useState(false);
  const [showSavedScreenerOverlay, setShowSavedScreenerOverlay] = useState(false);
  const [overlayResults, setOverlayResults] = useState<ScreeningResult[]>([]);
  const [overlayHeaders, setOverlayHeaders] = useState<HeaderColumn[]>([]);
  const [overlayUniverse, setOverlayUniverse] = useState<string>('');
  const [overlayExchange, setOverlayExchange] = useState<string>('');
  const [overlayExplainer, setOverlayExplainer] = useState<string>('');
  const [overlayQuery, setOverlayQuery] = useState<string>('');
  const [overlayScreenerName, setOverlayScreenerName] = useState<string>('');

  // Save state to localStorage whenever it changes
  useEffect(() => {
    localStorage.setItem('screener_activeTab', activeTab);
  }, [activeTab]);

  useEffect(() => {
    localStorage.setItem('screener_searchQuery', searchQuery);
  }, [searchQuery]);

  useEffect(() => {
    localStorage.setItem('screener_results', JSON.stringify(results));
  }, [results]);

  useEffect(() => {
    localStorage.setItem('screener_headers', JSON.stringify(headers));
  }, [headers]);

  useEffect(() => {
    localStorage.setItem('screener_universe', universe);
  }, [universe]);

  useEffect(() => {
    localStorage.setItem('screener_exchange', exchange);
  }, [exchange]);

  useEffect(() => {
    localStorage.setItem('screener_explainer', explainer);
  }, [explainer]);

  useEffect(() => {
    localStorage.setItem('screener_query', query);
  }, [query]);

  // Fetch saved screeners on mount to show accurate count
  useEffect(() => {
    fetchSavedScreeners();
  }, []);

  // Also fetch when switching to saved tab to refresh the list
  useEffect(() => {
    if (activeTab === 'saved') {
      fetchSavedScreeners();
    } else if (activeTab === 'create') {
      checkIfSaved();
    }
  }, [activeTab]);

  // Check if current screener is saved
  useEffect(() => {
    if (results.length > 0 && searchQuery && explainer) {
      checkIfSaved();
    } else {
      setIsSaved(false);
    }
  }, [results, searchQuery, explainer, savedScreeners]);

  const fetchScreenerResults = async (prompt: string) => {
    setLoading(true);
    setError(null);
    setExplainer('');
    setQuery('');
    setIsSaved(false); // Reset saved state when creating a new screener
    try {
      // Use proxy through backend to avoid CORS issues
      // The backend should proxy this request to the external API
      const baseUrl = document.querySelector('base')?.getAttribute('href') || '/';
      // Remove trailing slash if present, then add the API path
      const cleanBaseUrl = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl;
      const apiUrl = `${cleanBaseUrl}/api/proxy/screener?prompt=${encodeURIComponent(prompt)}&method=multiple`;
      
      console.log('[Screener] Making API request to:', apiUrl);
      
      const response = await fetch(apiUrl, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include', // Include cookies for auth
      });
      
      // Check content type before parsing
      const contentType = response.headers.get('content-type') || '';
      
      if (!response.ok) {
        let errorText = '';
        let errorMessage = '';
        try {
          errorText = await response.text();
          console.log('[Screener] Error response:', errorText);
          // Try to parse as JSON if possible
          if (contentType.includes('application/json')) {
            const errorJson = JSON.parse(errorText);
            console.log('[Screener] Parsed error JSON:', errorJson);
            // Extract error message from nested structure
            // Handle: { error: { message: "..." }, status: 500 }
            if (errorJson.error) {
              if (typeof errorJson.error === 'string') {
                errorMessage = errorJson.error;
              } else if (errorJson.error.message) {
                errorMessage = errorJson.error.message;
              } else if (errorJson.error.status) {
                errorMessage = errorJson.error.status;
              } else {
                errorMessage = JSON.stringify(errorJson.error);
              }
            } else if (errorJson.message) {
              errorMessage = errorJson.message;
            } else {
              errorMessage = errorText;
            }
            throw new Error(errorMessage || `API error: ${response.status}`);
          } else {
            errorMessage = errorText;
          }
        } catch (parseError: any) {
          // If it's not JSON, use the text as-is
          if (parseError.message && parseError.message !== errorText) {
            // If it's our thrown error, re-throw it
            throw parseError;
          }
          errorMessage = errorText;
        }
        console.error('API Error:', errorText);
        throw new Error(errorMessage || `API error: ${response.status}`);
      }
      
      // Check if response is JSON before parsing
      if (!contentType.includes('application/json')) {
        const text = await response.text();
        console.error('Non-JSON response:', text.substring(0, 200));
        throw new Error(`Server returned non-JSON response. Content-Type: ${contentType}`);
      }
      
      const data = await response.json();
      
      // Transform API response to match our interface
      // Expected structure: { status, message, data: { response: { body: [...], header: [...] }, exchange, universe, explainer, query } }
      if (data.status === 'success' && data.data?.response?.body) {
        const body = data.data.response.body;
        const header = data.data.response.header || [];
        const responseExchange = data.data.exchange || '';
        const responseUniverse = data.data.universe || data.universe || '';
        // Try multiple possible locations for explainer and query
        const responseExplainer = data.data.explainer || data.data.response?.explainer || data.explainer || '';
        const responseQuery = data.data.query || data.data.response?.query || data.query || '';
        
        // Set headers and metadata
        setHeaders(header);
        setExchange(cleanStringValue(responseExchange));
        setUniverse(cleanStringValue(responseUniverse));
        setExplainer(responseExplainer);
        setQuery(responseQuery);
        
        // Transform body rows to objects using header names as keys
        const transformedResults: ScreeningResult[] = body.map((row: any[]) => {
          const result: ScreeningResult = {};
          header.forEach((col: HeaderColumn, index: number) => {
            const value = row[index];
            // Handle different data types
            if (col.data_type === 'float' || col.data_type === 'int') {
              result[col.name] = parseFloat(value) || 0;
            } else {
              result[col.name] = value || '';
            }
          });
          return result;
        });
        
        setResults(transformedResults);
      } else if (Array.isArray(data)) {
        // Fallback for array response
        const transformedResults: ScreeningResult[] = data.map((item: any) => ({
          symbol: item.symbol || item.name ,
          exchange: item.exchange ,
          price: item.price || item.current_price ,
          priceChange: item.price_change || item.change ,
          priceChangePercent: item.price_change_percent || item.change_percent ,
          fiveMinChange: item.five_min_change  || item['5m_change'] ,
          open: item.open || item.open_price ,
          high: item.high || item.high_price ,
          low: item.low || item.low_price ,
          close: item.close || item.close_price ,
          sector: item.sector || item.industry ,
        }));
        setResults(transformedResults);
      } else {
        console.warn('Unexpected API response structure:', data);
        setResults([]);
      }
    } catch (err: any) {
      console.error('Error fetching screener results:', err);
      setError(err.message || 'Failed to fetch screener results. Please check if the API is accessible.');
      setResults([]);
      setExplainer('');
      setQuery('');
    } finally {
      setLoading(false);
    }
  };


  const handleCreateScreener = () => {
    if (!searchQuery.trim()) {
      setError('Please enter a search query');
      return;
    }
    fetchScreenerResults(searchQuery);
  };

  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handleCreateScreener();
    }
  };

  const checkIfSaved = () => {
    if (!searchQuery || !savedScreeners.length) {
      setIsSaved(false);
      return;
    }
    
    // Normalize searchQuery for comparison (trim and lowercase)
    const normalizedSearchQuery = searchQuery.trim().toLowerCase();
    
    const isCurrentlySaved = savedScreeners.some(
      (screener) => {
        const screenerName = (screener.screener_name || screener.ScreenerName || '').trim().toLowerCase();
        return screenerName === normalizedSearchQuery;
      }
    );
    
    setIsSaved(isCurrentlySaved);
  };

  const fetchSavedScreeners = async () => {
    try {
      const baseUrl = document.querySelector('base')?.getAttribute('href') || '/';
      const cleanBaseUrl = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl;
      // Use Go API endpoint
      const apiUrl = `${cleanBaseUrl}/api/v1/screeners/saved`;

      const token = localStorage.getItem('access_token');
      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      const response = await fetch(apiUrl, {
        method: 'GET',
        headers,
        credentials: 'include',
      });

      if (!response.ok) {
        throw new Error('Failed to fetch saved screeners');
      }

      const data = await response.json();
      const screeners = data.data || [];
      
      setSavedScreeners(screeners);
      
      // Cache in localStorage for instant display on next load
      localStorage.setItem('cached_saved_screeners', JSON.stringify(screeners));
    } catch (err: any) {
      console.error('Error fetching saved screeners:', err);
    }
  };

  const handleSaveScreener = async () => {
    if (!searchQuery || !query || saving) {
      return;
    }

    // Toggle behavior: if already saved, delete it
    if (isSaved) {
      // Fetch latest saved screeners to get the real ID (not optimistic temp ID)
      const baseUrl = document.querySelector('base')?.getAttribute('href') || '/';
      const cleanBaseUrl = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl;
      const apiUrl = `${cleanBaseUrl}/api/v1/screeners/saved`;
      const token = localStorage.getItem('access_token');
      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      try {
        const response = await fetch(apiUrl, {
          method: 'GET',
          headers,
          credentials: 'include',
        });

        if (response.ok) {
          const data = await response.json();
          const currentSavedScreeners = data.data || [];
          
          // Normalize searchQuery for comparison
          const normalizedSearchQuery = searchQuery.trim().toLowerCase();
          
          const savedScreener = currentSavedScreeners.find(
            (screener: any) => {
              const screenerName = (screener.screener_name || screener.ScreenerName || '').trim().toLowerCase();
              return screenerName === normalizedSearchQuery;
            }
          );
          
          if (savedScreener) {
            await handleDeleteScreener(savedScreener.id || savedScreener.ID, true);
            return;
          } else {
            showToast({
              message: 'Screener not found',
              status: 'error',
              duration: 3000
            });
          }
        }
      } catch (err) {
        console.error('Error fetching screener for delete:', err);
        showToast({
          message: 'Failed to delete screener',
          status: 'error',
          duration: 3000
        });
        return;
      }
    }

    // Otherwise, save the screener
    // OPTIMISTIC UI UPDATE: Set saved immediately and add to list
    setIsSaved(true);
    
    // Create optimistic screener object to add to list
    const optimisticScreener = {
      id: `temp-${Date.now()}`, // Temporary ID until DB fetch
      screener_name: searchQuery,
      tableName: cleanStringValue(exchange) || '',
      query: query,
      universeList: cleanStringValue(universe) || '',
      explainer: explainer,
      created_at: new Date().toISOString(),
    };
    
    // Add to list optimistically
    const previousScreeners = [...savedScreeners];
    const updatedScreeners = [optimisticScreener, ...savedScreeners];
    setSavedScreeners(updatedScreeners);
    localStorage.setItem('cached_saved_screeners', JSON.stringify(updatedScreeners));
    
    setSaving(true);
    
    try {
      const baseUrl = document.querySelector('base')?.getAttribute('href') || '/';
      const cleanBaseUrl = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl;
      // Use Go API endpoint
      const apiUrl = `${cleanBaseUrl}/api/v1/screeners/save`;

      const token = localStorage.getItem('access_token');
      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      // Clean and normalize optional fields before sending - remove all quotes and whitespace
      const cleanedUniverse = cleanStringValue(universe) || '';
      const cleanedExchange = cleanStringValue(exchange) || '';
      
      const response = await fetch(apiUrl, {
        method: 'POST',
        headers,
        credentials: 'include',
        body: JSON.stringify({
          prompt: searchQuery, // Backend expects 'prompt' field for screener name
          data: {
            exchange: cleanedExchange,
            explainer: explainer || '',
            query: query || '',
            universe: cleanedUniverse,
          },
        }),
      });

      if (!response.ok) {
        // Revert optimistic updates on error
        setIsSaved(false);
        setSavedScreeners(previousScreeners);
        localStorage.setItem('cached_saved_screeners', JSON.stringify(previousScreeners));
        
        // Handle 409 Conflict - should not happen anymore as backend handles toggle
        if (response.status === 409) {
          const errorData = await response.json().catch(() => ({}));
          showToast({ 
            message: errorData.message || 'This screener is already saved', 
            status: 'info',
            duration: 3000 
          });
          setIsSaved(true);
          // Fetch fresh data from DB
          await fetchSavedScreeners();
          return;
        }
        // Handle 401 Unauthorized - try to refresh token
        if (response.status === 401) {
          const refreshToken = localStorage.getItem('refresh_token');
          if (refreshToken) {
            try {
              const refreshResponse = await fetch(`${cleanBaseUrl}/api/v1/auth/refresh`, {
                method: 'POST',
                headers: {
                  'Content-Type': 'application/json',
                },
                body: JSON.stringify({ refresh_token: refreshToken }),
              });

              if (refreshResponse.ok) {
                const refreshData: any = await refreshResponse.json();
                if (refreshData.access_token) {
                  localStorage.setItem('access_token', refreshData.access_token);
                  if (refreshData.refresh_token) {
                    localStorage.setItem('refresh_token', refreshData.refresh_token);
                  }
                  // Retry the original request with new token
                  const retryHeaders: HeadersInit = {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${refreshData.access_token}`,
                  };
                  const retryResponse = await fetch(apiUrl, {
                    method: 'POST',
                    headers: retryHeaders,
                    credentials: 'include',
                    body: JSON.stringify({
                      screener_name: searchQuery,
                      tableName: cleanedExchange,
                      query: query,
                      universeList: cleanedUniverse,
                      explainer: explainer,
                    }),
                  });

                  if (!retryResponse.ok) {
                    // Revert optimistic updates
                    setIsSaved(false);
                    setSavedScreeners(previousScreeners);
                    localStorage.setItem('cached_saved_screeners', JSON.stringify(previousScreeners));
                    
                    // Handle 409 Conflict - should not happen anymore as backend handles toggle
                    if (retryResponse.status === 409) {
                      const errorData = await retryResponse.json().catch(() => ({}));
                      showToast({ 
                        message: errorData.message || 'This screener is already saved', 
                        status: 'info',
                        duration: 3000 
                      });
                      setIsSaved(true);
                      await fetchSavedScreeners(); // Fetch fresh data from DB
                      return;
                    }
                    const errorData = await retryResponse.json().catch(() => ({}));
                    throw new Error(errorData.message || errorData.error || 'UNAUTHORIZED');
                  }

                  const retryData = await retryResponse.json();
                  if (retryData.status === 'success') {
                    // Fetch fresh data from DB
                    await fetchSavedScreeners();
                  }
                  return;
                }
              }
            } catch (refreshError) {
              console.error('Token refresh failed:', refreshError);
            }
          }
        }

        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.message || errorData.error || 'UNAUTHORIZED');
      }

      const data = await response.json();
      if (data.status === 'success') {
        // Fetch fresh data from DB to update the list and cache
        await fetchSavedScreeners();
        showToast({
          message: 'Screener saved successfully',
          status: 'success',
          duration: 3000
        });
      }
    } catch (err: any) {
      console.error('Error saving screener:', err);
      // Revert optimistic updates on error
      setIsSaved(false);
      setSavedScreeners(previousScreeners);
      localStorage.setItem('cached_saved_screeners', JSON.stringify(previousScreeners));
      setError(err.message || 'Failed to save screener');
      showToast({
        message: err.message || 'Failed to save screener',
        status: 'error',
        duration: 3000
      });
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteScreener = async (screenerId: string, skipConfirm: boolean = false) => {
    if (!skipConfirm && !confirm('Are you sure you want to delete this screener?')) {
      return;
    }

    // OPTIMISTIC UI UPDATE: Update state immediately
    const wasCurrentlyDisplayed = isSaved;
    if (wasCurrentlyDisplayed) {
      setIsSaved(false);
    }
    
    // Optimistically remove from saved list
    const previousScreeners = [...savedScreeners];
    const updatedScreeners = savedScreeners.filter(s => (s.id || s.ID) !== screenerId);
    setSavedScreeners(updatedScreeners);
    
    // Update cache immediately
    localStorage.setItem('cached_saved_screeners', JSON.stringify(updatedScreeners));

    try {
      const baseUrl = document.querySelector('base')?.getAttribute('href') || '/';
      const cleanBaseUrl = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl;
      // Use Go API endpoint
      const apiUrl = `${cleanBaseUrl}/api/v1/screeners/${screenerId}`;

      const token = localStorage.getItem('access_token');
      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      const response = await fetch(apiUrl, {
        method: 'DELETE',
        headers,
        credentials: 'include',
      });

      if (!response.ok) {
        // Revert optimistic updates on error
        setSavedScreeners(previousScreeners);
        localStorage.setItem('cached_saved_screeners', JSON.stringify(previousScreeners));
        if (wasCurrentlyDisplayed) {
          setIsSaved(true);
        }
        throw new Error('Failed to delete screener');
      }

      // Success - fetch fresh data from DB to confirm and update cache
      await fetchSavedScreeners();
      
      showToast({
        message: 'Screener deleted successfully',
        status: 'success',
        duration: 3000
      });
    } catch (err: any) {
      console.error('Error deleting screener:', err);
      // Optimistic updates already reverted above
      setError(err.message || 'Failed to delete screener');
      showToast({
        message: err.message || 'Failed to delete screener',
        status: 'error',
        duration: 3000
      });
    }
  };

  const handleRunSavedScreener = async (screener: any) => {
    setLoading(true);
    setError(null);
    setShowSavedScreenerOverlay(true);
    
    try {
      const baseUrl = document.querySelector('base')?.getAttribute('href') || '/';
      const cleanBaseUrl = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl;
      // Use new Go API endpoint to run saved screener
      const apiUrl = `${cleanBaseUrl}/api/v1/screeners/${screener.id}/run`;

      const token = localStorage.getItem('access_token');
      const headers: HeadersInit = {};
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      console.log('[Screener] Running saved screener:', screener.id);

      const response = await fetch(apiUrl, {
        method: 'POST',
        headers,
        credentials: 'include',
      });

      if (!response.ok) {
        throw new Error(`Failed to run screener: ${response.status}`);
      }

      const data = await response.json();
      console.log('[Screener] Saved screener results:', data);

      // Set the search query for display
      const screenerName = screener.screener_name || screener.ScreenerName;
      setOverlayScreenerName(screenerName);

      // Handle multiple response formats:
      // 1. External API format (direct): {code: 200, s: 'ok', data: {body, header, universe, exchange}}
      // 2. External API format (nested): {code: 200, s: 'ok', data: {response: {body, header}, exchange, universe}}
      // 3. Standard format: {status: 'success', data: {response: {body, header}}}
      const isExternalFormat = data.code === 200 || data.s === 'ok';
      const isStandardFormat = data.status === 'success';
      
      // Check for direct format first (body and header directly under data.data)
      let body, header;
      
      if (isExternalFormat && data.data?.body) {
        // Direct format: data.data.body and data.data.header
        body = data.data.body;
        header = data.data.header || [];
      } else if ((isExternalFormat || isStandardFormat) && data.data?.response?.body) {
        // Nested format: data.data.response.body and data.data.response.header
        body = data.data.response.body;
        header = data.data.response.header || [];
      }
      
      if (body && header) {
        // Get metadata from database (screener object) instead of API response
        const responseExplainer = screener.explainer || screener.Explainer || '';
        const responseQuery = screener.query || screener.Query || '';
        // Use universe_list and table_name from database
        const responseUniverse = screener.universe_list || screener.universeList || '';
        const responseExchange = screener.table_name || screener.tableName || '';

        // Set overlay headers and metadata
        setOverlayHeaders(header);
        setOverlayExchange(cleanStringValue(responseExchange));
        setOverlayUniverse(cleanStringValue(responseUniverse));
        setOverlayExplainer(responseExplainer);
        setOverlayQuery(responseQuery);

        // Transform body rows to objects using header names as keys
        const transformedResults: ScreeningResult[] = body.map((row: any[]) => {
          const result: ScreeningResult = {};
          header.forEach((col: HeaderColumn, index: number) => {
            const value = row[index];
            // Handle different data types
            if (col.data_type === 'float' || col.data_type === 'int') {
              result[col.name] = parseFloat(value) || 0;
            } else {
              result[col.name] = value || '';
            }
          });
          return result;
        });

        setOverlayResults(transformedResults);
      } else {
        console.warn('Unexpected API response structure:', data);
        setOverlayResults([]);
      }
    } catch (err: any) {
      console.error('Error running saved screener:', err);
      setError(err.message || 'Failed to run saved screener');
      setOverlayResults([]);
    } finally {
      setLoading(false);
    }
  };



  return (
    <div className="h-full flex flex-col bg-white dark:bg-gray-850">
      {/* Main Content */}
      <div className="flex-1 overflow-auto p-6">
        {/* Header with Tabs and View History */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex border-b border-gray-200 dark:border-gray-700">
            <button
              onClick={() => setActiveTab('create')}
              className={`px-0 py-3 font-normal text-[14px] leading-[20px] ${
                activeTab === 'create'
                  ? 'border-b-2 border-[#2434E7] text-[#2A2A2A] dark:text-gray-100'
                  : 'text-[#6D6D6D] dark:text-gray-400 hover:text-[#2A2A2A] dark:hover:text-gray-200'
              }`}
              style={{ fontFamily: 'Inter, sans-serif' }}
            >
              Create
            </button>
            <button
              onClick={() => setActiveTab('saved')}
              className={`px-0 py-3 font-normal text-[14px] leading-[20px] ml-6 ${
                activeTab === 'saved'
                  ? 'border-b-2 border-[#2434E7] text-[#2A2A2A] dark:text-gray-100'
                  : 'text-[#6D6D6D] dark:text-gray-400 hover:text-[#2A2A2A] dark:hover:text-gray-200'
              }`}
              style={{ fontFamily: 'Inter, sans-serif' }}
            >
              Saved screeners ({savedScreeners.length})
            </button>
          </div>
          <button className="px-4 py-2 border border-gray-300 dark:border-gray-400 rounded-lg text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 flex items-center gap-2">
            <History className="w-4 h-4" />
            View history
          </button>
        </div>

        {activeTab === 'create' && (
          <>
            {/* Search Section */}
            <div className="mb-6">
              <div className="flex items-center gap-4 mb-4">
                <div className="flex-1 relative">
                  <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-5 h-5" />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    onKeyPress={handleKeyPress}
                    placeholder="Search"
                    className="w-full pl-10 pr-4 py-3 border border-gray-300 dark:border-gray-400 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                </div>
                <button
                  onClick={handleCreateScreener}
                  disabled={loading}
                  className="px-6 py-3 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
                >
                  {loading ? 'Creating...' : 'Create screener'}
                </button>
              </div>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                Use natural language to describe your screening criteria. AI will convert it into technical parameters.
              </p>
            </div>

            {/* Error Message */}
            {error && (
              <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-400">
                {error}
              </div>
            )}

            {/* Loading State - Show above table when loading new results */}
            {loading && results.length > 0 && (
              <div className="mt-6 mb-4 text-center py-8">
                <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
                <p className="mt-4 text-gray-600 dark:text-gray-400">Loading screening results...</p>
              </div>
            )}

            {/* Results Section */}
            {results.length > 0 && (
              <div className="mt-6">
                <div className="flex items-center justify-between mb-4">
                  <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">
                    Screening results
                  </h2>
                  <div className="flex items-center gap-3">
                    <button
                      onClick={handleSaveScreener}
                      disabled={saving || !query || !explainer}
                      className={`p-2 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors ${
                        isSaved ? 'text-red-500 dark:text-red-500' : 'text-gray-600 dark:text-gray-400'
                      } ${saving ? 'opacity-50 cursor-not-allowed' : ''}`}
                      title={isSaved ? 'Click to delete screener' : 'Click to save screener'}
                    >
                      <Heart className={`w-5 h-5 ${isSaved ? 'fill-current' : ''}`} />
                    </button>
                    {explainer && (
                      <HoverCard openDelay={200}>
                        <HoverCardTrigger asChild>
                          <button className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium flex items-center gap-2 cursor-help">
                            <Info className="w-4 h-4" />
                            Screening criteria
                          </button>
                        </HoverCardTrigger>
                        <HoverCardPortal>
                          <HoverCardContent side="bottom" align="end" className="w-[500px] max-h-[400px] overflow-y-auto">
                            <div>
                              <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-2">
                                Description
                              </h3>
                              <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed">
                                {explainer}
                              </p>
                            </div>
                          </HoverCardContent>
                        </HoverCardPortal>
                      </HoverCard>
                    )}
                    {!explainer && (
                      <button className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium flex items-center gap-2">
                        <Info className="w-4 h-4" />
                        Screening criteria
                      </button>
                    )}
                  </div>
                </div>

                {/* Universe and Exchange Info */}
                <div className="flex items-center justify-end mb-4">
                  <div className="flex items-center gap-2">
                    {universe && (
                      <span className="inline-flex items-center px-3 py-1 bg-gray-100 dark:bg-gray-800 rounded-full text-sm font-medium">
                        <span className="text-gray-700 dark:text-gray-300">Universe: </span>
                        <span className="text-blue-600 dark:text-blue-400 ml-1">{universe}</span>
                      </span>
                    )}
                    {exchange && (
                    <span className="inline-flex items-center px-3 py-1 bg-gray-100 dark:bg-gray-800 rounded-full text-sm font-medium">
                        <span className="text-gray-700 dark:text-gray-300">Exchange: </span>
                        <span className="text-blue-600 dark:text-blue-400 ml-1">{exchange}</span>
                    </span>
                    )}
                  </div>
                </div>

                {/* Results Table */}
                <div className="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded-lg">
                  <table className="min-w-full w-full border-collapse bg-white dark:bg-gray-800">
                    <thead>
                      <tr className="border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900">
                        {headers.map((header, index) => (
                          <th
                            key={index}
                            className={`py-3 px-4 text-sm font-semibold text-gray-700 dark:text-gray-300 align-top ${
                              index === 0 ? 'text-left' : 'text-center'
                            } ${index === headers.length - 1 ? 'text-right' : ''}`}
                          >
                            {header.name === 't0_close_ltp' ? 'Price' : header.name}
                        </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {results.map((result, rowIndex) => (
                          <tr
                          key={rowIndex}
                            className="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
                          >
                          {headers.map((header, colIndex) => {
                            const value = result[header.name];
                            const isNumeric = header.data_type === 'float' || header.data_type === 'int';
                            const isFirstColumn = colIndex === 0;
                            const isLastColumn = colIndex === headers.length - 1;
                            
                            return (
                              <td
                                key={colIndex}
                                className={`py-3 px-4 text-gray-700 dark:text-gray-300 align-top ${
                                  isFirstColumn ? 'text-left' : 'text-center'
                                } ${isLastColumn ? 'text-right' : ''}`}
                              >
                                {isNumeric ? (
                                  <span className={header.is_formated_to_klcr ? 'font-semibold' : ''}>
                                    {typeof value === 'number' 
                                      ? header.data_type === 'int' 
                                        ? value.toLocaleString() 
                                        : value.toFixed(2)
                                      : value}
                                  </span>
                                ) : (
                                  <span>{value || 'N/A'}</span>
                                )}
                            </td>
                        );
                      })}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Loading State - Show when no results yet */}
            {loading && results.length === 0 && (
              <div className="mt-6 text-center py-12">
                <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
                <p className="mt-4 text-gray-600 dark:text-gray-400">Loading screening results...</p>
              </div>
            )}

            {/* Empty State */}
            {!loading && results.length === 0 && !error && (
              <div className="mt-12 text-center">
                <p className="text-gray-500 dark:text-gray-400">
                  Enter a search query and click "Create screener" to see results
                </p>
              </div>
            )}
          </>
        )}

        {activeTab === 'saved' && (
          <div className="relative">
            {!showSavedScreenerOverlay && (
              <>
                {savedScreeners.length === 0 ? (
                  <div className="text-center py-12">
                    <p className="text-gray-500 dark:text-gray-400">
                      No saved screeners yet. Create a screener to save it here.
                    </p>
                  </div>
                ) : (
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {savedScreeners.map((screener) => {
                      const createdDate = new Date(screener.created_at || screener.CreatedAt);
                      const formattedDate = createdDate.toLocaleDateString('en-GB', {
                        day: '2-digit',
                        month: '2-digit',
                        year: 'numeric',
                      });

                      return (
                        <div
                          key={screener.id}
                          className="border border-gray-200 dark:border-gray-700 rounded-lg p-6 bg-white dark:bg-gray-800 hover:shadow-md transition-shadow cursor-pointer select-none"
                          onDoubleClick={(e) => {
                            // Prevent double-click from triggering on buttons
                            if ((e.target as HTMLElement).closest('button')) {
                              return;
                            }
                            handleDeleteScreener(screener.id || screener.ID, true);
                          }}
                          title="Double-click to delete"
                        >
                          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
                            {screener.screener_name || screener.ScreenerName}
                          </h3>
                          {(screener.explainer || screener.Explainer) && (
                            <p className="text-sm text-gray-600 dark:text-gray-400 mb-4 line-clamp-3">
                              {screener.explainer || screener.Explainer}
                            </p>
                          )}
                          <p className="text-xs text-gray-500 dark:text-gray-500 mb-4">
                            Created: {formattedDate}
                          </p>
                          <div className="flex items-center justify-end gap-2">
                            <button
                              onClick={(e) => {
                                e.stopPropagation();
                                handleRunSavedScreener(screener);
                              }}
                              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors"
                            >
                              Run
                            </button>
                            <button
                              onClick={(e) => {
                                e.stopPropagation();
                                handleDeleteScreener(screener.id || screener.ID);
                              }}
                              className="p-2 bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30 text-red-600 dark:text-red-400 rounded-lg transition-colors"
                              title="Delete screener"
                            >
                              <Trash2 className="w-4 h-4" />
                            </button>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </>
            )}

            {/* Overlay for saved screener results - appears within saved screeners area */}
            {showSavedScreenerOverlay && (
              <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-lg">
                {/* Header with back button */}
                <div className="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
                  <button
                    onClick={() => {
                      setShowSavedScreenerOverlay(false);
                      setOverlayResults([]);
                      setOverlayHeaders([]);
                      setError(null);
                    }}
                    className="flex items-center gap-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 transition-colors"
                  >
                    <ArrowLeft className="w-5 h-5" />
                    <span className="font-medium">Back</span>
                  </button>
                  <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">
                    {overlayScreenerName}
                  </h2>
                  <div className="w-20"></div> {/* Spacer for centering */}
                </div>

                {/* Content */}
                <div className="p-6 max-h-[calc(100vh-300px)] overflow-y-auto">
                  {/* Error Message */}
                  {error && (
                    <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-400">
                      {error}
                    </div>
                  )}

                  {/* Loading State */}
                  {loading && (
                    <div className="text-center py-12">
                      <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
                      <p className="mt-4 text-gray-600 dark:text-gray-400">Loading screening results...</p>
                    </div>
                  )}

                  {/* Results */}
                  {!loading && overlayResults.length > 0 && (
                    <>
                      {/* Universe and Exchange Info */}
                      <div className="flex items-center justify-end mb-4">
                        <div className="flex items-center gap-2">
                          {overlayUniverse && (
                            <span className="inline-flex items-center px-3 py-1 bg-gray-100 dark:bg-gray-800 rounded-full text-sm font-medium">
                              <span className="text-gray-700 dark:text-gray-300">Universe: </span>
                              <span className="text-blue-600 dark:text-blue-400 ml-1">{overlayUniverse}</span>
                            </span>
                          )}
                          {overlayExchange && (
                            <span className="inline-flex items-center px-3 py-1 bg-gray-100 dark:bg-gray-800 rounded-full text-sm font-medium">
                              <span className="text-gray-700 dark:text-gray-300">Exchange: </span>
                              <span className="text-blue-600 dark:text-blue-400 ml-1">{overlayExchange}</span>
                            </span>
                          )}
                        </div>
                      </div>

                      {/* Results Table */}
                      <div className="overflow-x-auto border border-gray-200 dark:border-gray-700 rounded-lg">
                        <table className="min-w-full w-full border-collapse bg-white dark:bg-gray-800">
                          <thead>
                            <tr className="border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900">
                              {overlayHeaders.map((header, index) => (
                                <th
                                  key={index}
                                  className={`py-3 px-4 text-sm font-semibold text-gray-700 dark:text-gray-300 align-top ${
                                    index === 0 ? 'text-left' : 'text-center'
                                  } ${index === overlayHeaders.length - 1 ? 'text-right' : ''}`}
                                >
                                  {header.name === 't0_close_ltp' ? 'Price' : header.name}
                                </th>
                              ))}
                            </tr>
                          </thead>
                          <tbody>
                            {overlayResults.map((result, rowIndex) => (
                              <tr
                                key={rowIndex}
                                className="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
                              >
                                {overlayHeaders.map((header, colIndex) => {
                                  const value = result[header.name];
                                  const isNumeric = header.data_type === 'float' || header.data_type === 'int';
                                  const isFirstColumn = colIndex === 0;
                                  const isLastColumn = colIndex === overlayHeaders.length - 1;
                                  
                                  return (
                                    <td
                                      key={colIndex}
                                      className={`py-3 px-4 text-gray-700 dark:text-gray-300 align-top ${
                                        isFirstColumn ? 'text-left' : 'text-center'
                                      } ${isLastColumn ? 'text-right' : ''}`}
                                    >
                                      {isNumeric ? (
                                        <span className={header.is_formated_to_klcr ? 'font-semibold' : ''}>
                                          {typeof value === 'number' 
                                            ? header.data_type === 'int' 
                                              ? value.toLocaleString() 
                                              : value.toFixed(2)
                                            : value}
                                        </span>
                                      ) : (
                                        <span>{value || 'N/A'}</span>
                                      )}
                                    </td>
                                  );
                                })}
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}