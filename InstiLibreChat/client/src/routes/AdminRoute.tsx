import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import AdminDashboard from '~/components/Admin/AdminDashboard';
import { saasApi } from '~/services/saasApi';

export default function AdminRoute() {
  const [userInfo, setUserInfo] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  // Fetch user info
  useEffect(() => {
    const fetchUserInfo = async () => {
      try {
        const token = localStorage.getItem('access_token');
        if (!token) {
          navigate('/login');
          return;
        }

        const data = await saasApi.getMe();
        setUserInfo(data);
      } catch (error) {
        console.error('Error fetching user info:', error);
        navigate('/login');
      } finally {
        setLoading(false);
      }
    };

    fetchUserInfo();
  }, [navigate]);

  // Redirect verified users immediately if they don't have admin access
  useEffect(() => {
    if (!loading && userInfo) {
      // First check localStorage (set during login for immediate availability)
      let canAccessAdmin = false;
      const storedCanAccessAdmin = localStorage.getItem('canAccessAdmin');
      if (storedCanAccessAdmin !== null) {
        try {
          canAccessAdmin = JSON.parse(storedCanAccessAdmin) === true;
        } catch (e) {
          // If parsing fails, fall through to check userInfo
        }
      }
      
      // If not in localStorage, check from userInfo
      // Check based on org_role: show admin panel if org_role === 'admin' OR is_super_admin === true
      if (!canAccessAdmin) {
        const isSuperAdmin = userInfo.is_super_admin === true;
        const orgRole = userInfo.org_role || userInfo.orgRole;
        const isOrgAdmin = orgRole === 'admin';

        canAccessAdmin = isSuperAdmin || isOrgAdmin;
        // Update localStorage
        localStorage.setItem('canAccessAdmin', JSON.stringify(canAccessAdmin));
      }

      // Only super admins or users with org_role === 'admin' can access admin panel
      // Verified users (email_verified && active) should NOT see the admin panel
      if (!canAccessAdmin) {
        // Immediately redirect to chat (FIA research)
        const baseHref = document.querySelector('base')?.getAttribute('href') || '/';
        const lastConversationId = localStorage.getItem('lastConversationId');
        if (lastConversationId && lastConversationId !== 'new') {
          navigate(`${baseHref}c/${lastConversationId}`, { replace: true });
        } else {
          navigate(`${baseHref}c/new`, { replace: true });
        }
      }
    }
  }, [loading, userInfo, navigate]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-gray-500 dark:text-gray-400">Loading...</div>
      </div>
    );
  }

  if (!userInfo) {
    return null;
  }

  // Check if user has access to admin (re-check for render)
  // First check localStorage (set during login)
  let canAccessAdmin = false;
  const storedCanAccessAdmin = localStorage.getItem('canAccessAdmin');
  if (storedCanAccessAdmin !== null) {
    try {
      canAccessAdmin = JSON.parse(storedCanAccessAdmin) === true;
    } catch (e) {
      // If parsing fails, fall through to check userInfo
    }
  }
  
  // If not in localStorage, check from userInfo
  // Check based on org_role: show admin panel if org_role === 'admin' OR is_super_admin === true
  if (!canAccessAdmin) {
    const isSuperAdmin = userInfo.is_super_admin === true;
    const orgRole = userInfo.org_role || userInfo.orgRole;
    const isOrgAdmin = orgRole === 'admin';

    canAccessAdmin = isSuperAdmin || isOrgAdmin;
    // Update localStorage
    localStorage.setItem('canAccessAdmin', JSON.stringify(canAccessAdmin));
  }

  // CRITICAL: Never render AdminDashboard if user doesn't have access
  if (!canAccessAdmin) {
    // Show access denied message while redirect is happening
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-center">
          <p className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">Access Denied</p>
          <p className="text-gray-500 dark:text-gray-400">You don't have permission to access the Admin section.</p>
          <p className="text-sm text-gray-400 dark:text-gray-500 mt-2">Redirecting to FIA research...</p>
        </div>
      </div>
    );
  }

  return <AdminDashboard userInfo={userInfo} />;
}

