import { useState, useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAuthContext } from '~/hooks/AuthContext';
import { PermissionManager, type Permission } from '~/utils/permissions';
import { saasApi } from '~/services/saasApi';

interface NiftyData {
  symbol: string;
  ltp: number;
  open: number;
  high: number;
  low: number;
  prevClose: number;
  change: number;
  changePercent: number;
  volume: number;
  timestamp: string;
}

export default function TopNavBar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuthContext();
  const [userInfo, setUserInfo] = useState<any>(null);
  const [permissionManager, setPermissionManager] = useState<PermissionManager | null>(null);
  const [activeMenu, setActiveMenu] = useState('Stock research');
  const [niftyData, setNiftyData] = useState<NiftyData | null>(null);
  const [niftyLoading, setNiftyLoading] = useState(true);
  const [niftyError, setNiftyError] = useState<string | null>(null);

  useEffect(() => {
    // Load user info and permissions
    const loadUserData = async () => {
      try {
        const token = localStorage.getItem('access_token');
        if (!token) {
          console.warn('No access token found');
          return;
        }

        try {
          const data = await saasApi.getMe();
          setUserInfo(data);

          // Load permissions
          const storedPerms = localStorage.getItem('permissions');
          if (storedPerms) {
            try {
              const perms = JSON.parse(storedPerms);
              setPermissionManager(new PermissionManager(perms));
            } catch (error) {
              console.error('Error parsing permissions:', error);
            }
          }
        } catch (error: any) {
          console.error('Error loading user data:', error);
          // If it's a 401, try to refresh token
          if (error.message?.includes('401') || error.message?.includes('Unauthorized')) {
            const refreshToken = localStorage.getItem('refresh_token');
            if (refreshToken) {
              try {
                const refreshData: any = await saasApi.refreshToken(refreshToken);
                if (refreshData.access_token) {
                  localStorage.setItem('access_token', refreshData.access_token);
                  if (refreshData.refresh_token) {
                    localStorage.setItem('refresh_token', refreshData.refresh_token);
                  }
                  // Retry loading user data
                  const retryData = await saasApi.getMe();
                  setUserInfo(retryData);
                }
              } catch (refreshError) {
                console.error('Token refresh failed:', refreshError);
              }
            }
          }
        }
      } catch (error) {
        console.error('Error in loadUserData:', error);
      }
    };

    loadUserData();

    // Determine active menu based on current route
    const path = location.pathname;
    if (path.includes('/admin')) {
      setActiveMenu('Admin');
    } else if (path.includes('/resources')) {
      setActiveMenu('Resources');
    } else if (path.includes('/templates')) {
      setActiveMenu('Templates');
    } else if (path.includes('/screener')) {
      setActiveMenu('Screeners');
    } else {
      setActiveMenu('Stock research');
    }

    // Store current conversation ID when on a chat route
    // This allows us to restore it when navigating back from other tabs
    if (path.startsWith('/c/')) {
      const conversationId = path.replace('/c/', '');
      // Only store valid conversation IDs (not 'new')
      if (conversationId && conversationId !== 'new') {
        localStorage.setItem('lastConversationId', conversationId);
      }
    }
  }, [location.pathname]);

  // Fetch NIFTY data
  const fetchNiftyData = async () => {
    try {
      setNiftyError(null);
      const baseHref = document.querySelector('base')?.getAttribute('href') || '/';
      // Remove trailing slash if present, then add the API path
      const cleanBaseUrl = baseHref.endsWith('/') ? baseHref.slice(0, -1) : baseHref;
      const apiUrl = `${cleanBaseUrl}/api/market/nifty`;
      
      const response = await fetch(apiUrl, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include', // Include cookies for auth
      });

      // Check content type before parsing
      const contentType = response.headers.get('content-type') || '';
      
      // If response is HTML, the API endpoint doesn't exist (server needs restart or route not registered)
      if (contentType.includes('text/html')) {
        // Silently fail - API endpoint not available
        // Don't log or show errors - just silently return
        setNiftyError(null);
        setNiftyLoading(false);
        return;
      }

      if (!response.ok) {
        // If response is not OK, try to get error message
        if (contentType.includes('application/json')) {
          const errorData = await response.json();
          setNiftyError(errorData.error || `HTTP ${response.status}`);
        } else {
          // If non-HTML, non-JSON response
          setNiftyError(null); // Silently fail
        }
        setNiftyLoading(false);
        return;
      }

      // Ensure response is JSON before parsing
      if (!contentType.includes('application/json')) {
        // Silently fail if not JSON
        setNiftyError(null);
        setNiftyLoading(false);
        return;
      }

      const result = await response.json();

      if (result.success && result.data) {
        setNiftyData(result.data);
        setNiftyError(null); // Clear any previous errors
      } else {
        setNiftyError(result.error || null);
      }
    } catch (err: any) {
      // Silently handle all errors - don't log or show to user
      // This prevents console spam when API endpoint doesn't exist
      setNiftyError(null);
    } finally {
      setNiftyLoading(false);
    }
  };

  // Fetch NIFTY data on mount and set up auto-refresh
  useEffect(() => {
    fetchNiftyData();
    
    // Auto-refresh every 1 second for real-time updates
    const interval = setInterval(() => {
      fetchNiftyData();
    }, 1000);

    return () => clearInterval(interval);
  }, []);

  const handleMenuChange = (menu: string) => {
    setActiveMenu(menu);
    const baseHref = document.querySelector('base')?.getAttribute('href') || '/';

    switch (menu) {
      case 'Admin':
        navigate(`${baseHref}admin`);
        break;
      case 'Stock research': {
        // Restore last conversation if available, otherwise go to new chat
        const lastConversationId = localStorage.getItem('lastConversationId');
        if (lastConversationId && lastConversationId !== 'new') {
          navigate(`${baseHref}c/${lastConversationId}`);
        } else {
          navigate(`${baseHref}c/new`);
        }
        break;
      }
      case 'Templates':
        navigate(`${baseHref}templates`);
        break;
      case 'Screeners':
        navigate(`${baseHref}screener`);
        break;
      case 'Resources':
        navigate(`${baseHref}resources`);
        break;
    }
  };

  const handleLogout = async () => {
    try {
      const token = localStorage.getItem('access_token');
      const refreshToken = localStorage.getItem('refresh_token');
      if (token && refreshToken) {
        // Call logout endpoint if available
        try {
          await fetch('/api/v1/auth/logout', {
            method: 'POST',
            headers: {
              'Authorization': `Bearer ${token}`,
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({ refresh_token: refreshToken }),
          });
        } catch (error) {
          // Ignore logout API errors
        }
      }
    } catch (error) {
      console.error('Logout error:', error);
    }
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    localStorage.removeItem('user');
    localStorage.removeItem('permissions');
    logout();
  };

  const isSuperAdmin = userInfo?.is_super_admin || false;
  const hasOrgId = userInfo?.org_id;
  const canAccessAdmin =
    isSuperAdmin ||
    (permissionManager && permissionManager.hasPermission('admin', 'read')) ||
    (hasOrgId && !isSuperAdmin);

  // Always show Templates, Stock research, Screeners, and Resources; conditionally show Admin
  const menus = ['Admin', 'Stock research', 'Screeners', 'Resources', 'Templates'].filter((menu) => {
    if (menu === 'Admin' && !canAccessAdmin) {
      return false;
    }
    // Always show Templates, Stock research, Screeners, and Resources
    return true;
  });

  // Format NIFTY data for display
  const formatNiftyValue = () => {
    if (niftyLoading) {
      return 'Loading...';
    }
    if (niftyError || !niftyData) {
      return 'N/A';
    }
    return niftyData.ltp.toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  };

  const formatNiftyChange = () => {
    if (!niftyData) return { value: '0.00', percent: '(0.00%)', isPositive: true };
    const isPositive = niftyData.change >= 0;
    const changeValue = Math.abs(niftyData.change).toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    const changePercent = Math.abs(niftyData.changePercent).toFixed(2);
    return {
      value: `${isPositive ? '+' : '-'}${changeValue}`,
      percent: `(${isPositive ? '+' : '-'}${changePercent}%)`,
      isPositive
    };
  };

  return (
    <nav className="bg-white dark:bg-gray-850 border-b border-gray-200 dark:border-gray-700 px-6 py-4 flex-shrink-0 z-50">
      <div className="flex items-center justify-between">
        {/* Left: Branding/Financial Data */}
        <div className="flex items-center gap-4">
          {/* Organization Logo */}
          {userInfo?.org_logo_url && userInfo.org_logo_url.startsWith('data:') && !userInfo?.is_super_admin && (
            <div className="flex items-center">
              <img
                src={userInfo.org_logo_url}
                alt={userInfo.org_name || 'Organization logo'}
                className="h-10 w-10 object-contain"
                style={{ borderRadius: 0 }}
                onError={(e) => {
                  (e.target as HTMLImageElement).style.display = 'none';
                }}
              />
            </div>
          )}
          {/* NIFTY Display - Fixed width container to prevent layout shifts */}
          <div className="flex items-center gap-2 min-w-[280px]" style={{ fontVariantNumeric: 'tabular-nums' }}>
            <span className="text-gray-900 dark:text-gray-100 font-semibold whitespace-nowrap">NIFTY</span>
            <span className="text-gray-900 dark:text-gray-100 font-medium tabular-nums min-w-[90px] text-right">
              {formatNiftyValue()}
            </span>
            {niftyData && (
              <span 
                className={`font-medium tabular-nums whitespace-nowrap ${
                  formatNiftyChange().isPositive 
                    ? 'text-green-600 dark:text-green-400' 
                    : 'text-red-600 dark:text-red-400'
                }`}
              >
                {formatNiftyChange().value} {formatNiftyChange().percent}
              </span>
            )}
            {niftyError && (
              <span className="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap" title={niftyError}>
                ⚠️
              </span>
            )}
          </div>
        </div>

        {/* Center: Primary Menu */}
        <div className="flex items-center gap-1">
          {menus.map((menu) => (
            <button
              key={menu}
              onClick={() => handleMenuChange(menu)}
              className={`px-4 py-2 rounded-lg font-medium transition ${
                activeMenu === menu
                  ? 'bg-blue-600 text-white dark:bg-blue-500'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              {menu}
            </button>
          ))}
        </div>

        {/* Right: User Actions */}
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="w-10 h-10 bg-gray-300 dark:bg-gray-700 rounded-full flex items-center justify-center">
              <svg
                className="w-6 h-6 text-gray-600 dark:text-gray-300"
                fill="currentColor"
                viewBox="0 0 24 24"
              >
                <path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z" />
              </svg>
            </div>
            <button
              onClick={handleLogout}
              className="text-sm text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-gray-100"
            >
              Logout
            </button>
          </div>
        </div>
      </div>
    </nav>
  );
}

