import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import AdminDashboard from '~/components/Admin/AdminDashboard';
import { saasApi } from '~/services/saasApi';

export default function AdminRoute() {
  const [userInfo, setUserInfo] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

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

  // Check if user has access to admin
  const isSuperAdmin = userInfo.is_super_admin;
  const hasOrgId = userInfo.org_id;
  const storedPerms = localStorage.getItem('permissions');
  let hasAdminPermission = false;
  
  if (storedPerms) {
    try {
      const perms = JSON.parse(storedPerms);
      hasAdminPermission = perms.some(
        (p: any) => p.resource === 'admin' && p.action === 'read'
      );
    } catch (e) {
      // Ignore parse errors
    }
  }

  const canAccessAdmin = isSuperAdmin || hasAdminPermission || (hasOrgId && !isSuperAdmin);

  if (!canAccessAdmin) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-center">
          <p className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">Access Denied</p>
          <p className="text-gray-500 dark:text-gray-400">You don't have permission to access the Admin section.</p>
        </div>
      </div>
    );
  }

  return <AdminDashboard userInfo={userInfo} />;
}

