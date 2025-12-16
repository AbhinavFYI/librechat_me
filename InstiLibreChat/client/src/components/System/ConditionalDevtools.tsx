import { useState, useEffect } from 'react';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { saasApi } from '~/services/saasApi';

export default function ConditionalDevtools() {
  const [userInfo, setUserInfo] = useState<any>(null);
  const [userInfoLoaded, setUserInfoLoaded] = useState(false);

  useEffect(() => {
    const loadUserInfo = async () => {
      try {
        const token = localStorage.getItem('access_token');
        if (!token) {
          setUserInfoLoaded(true);
          return;
        }
        const data = await saasApi.getMe();
        setUserInfo(data);
      } catch (error) {
        console.error('Error loading user info for devtools:', error);
      } finally {
        setUserInfoLoaded(true);
      }
    };
    loadUserInfo();
  }, []);

  // Only show React Query Devtools for super admins
  const isSuperAdmin = userInfoLoaded && userInfo !== null && userInfo?.is_super_admin === true;

  if (!isSuperAdmin) {
    return null;
  }

  return <ReactQueryDevtools initialIsOpen={false} position="top-right" />;
}

