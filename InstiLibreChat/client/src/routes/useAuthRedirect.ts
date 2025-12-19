import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthContext } from '~/hooks';

// Helper function to check if authentication cookies exist
function hasAuthCookies(): boolean {
  const cookies = document.cookie;
  // Check for any of the authentication cookies
  return cookies.includes('refreshToken') || 
         cookies.includes('libre_jwt') || 
         cookies.includes('token_provider');
}

export default function useAuthRedirect() {
  const { user, isAuthenticated } = useAuthContext();
  const navigate = useNavigate();
  const [hasCheckedAuth, setHasCheckedAuth] = useState(false);
  const [checkCount, setCheckCount] = useState(0);

  useEffect(() => {
    // Give more time for authentication to be established
    // This is important when cookies are set but AuthContext hasn't loaded yet
    const timeout = setTimeout(() => {
      setHasCheckedAuth(true);
      setCheckCount(prev => prev + 1);
      
      // Check if auth cookies exist - if they do, give more time for auth to complete
      const cookiesExist = hasAuthCookies();
      
      // Only redirect to login if:
      // 1. Not authenticated
      // 2. User is undefined (auth check complete)
      // 3. No auth cookies exist (not in the middle of authentication)
      // 4. We've checked at least twice (give extra time for slow networks)
      if (!isAuthenticated && user === undefined && !cookiesExist && checkCount >= 2) {
        console.log('Auth check failed, redirecting to login');
        navigate('/login', { replace: true });
      } else if (!isAuthenticated && cookiesExist && checkCount < 5) {
        // If cookies exist but not authenticated yet, wait longer (up to 5 checks = 5 seconds)
        console.log('Auth cookies found, waiting for authentication to complete...');
      }
    }, 1000); // Check every 1 second

    return () => {
      clearTimeout(timeout);
    };
  }, [isAuthenticated, user, navigate, checkCount]);

  return {
    user,
    isAuthenticated,
    hasCheckedAuth,
  };
}
