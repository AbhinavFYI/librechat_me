import { useEffect, useState, useRef } from 'react';
import { Outlet } from 'react-router-dom';
import { QueryKeys } from 'librechat-data-provider';
import { useQueryClient } from '@tanstack/react-query';
import { useAuthContext, usePreviousLocation } from '~/hooks';
import { DashboardContext } from '~/Providers';
import store from '~/store';

export default function DashboardRoute() {
  const queryClient = useQueryClient();
  const { isAuthenticated } = useAuthContext();
  const prevLocationRef = usePreviousLocation();
  const clearConvoState = store.useClearConvoState();
  const [prevLocationPath, setPrevLocationPath] = useState('');
  
  // Track if we've already initialized (one-time setup)
  const hasInitializedRef = useRef(false);

  useEffect(() => {
    setPrevLocationPath(prevLocationRef.current?.pathname || '');
  }, [prevLocationRef]);

  // One-time initialization: Only clear conversation state on first mount
  // This should NOT run on every route transition
  useEffect(() => {
    // Only run once on initial mount, not on every route change
    if (hasInitializedRef.current) {
      return;
    }
    
    // Mark as initialized
    hasInitializedRef.current = true;
    
    // Only clean up 'new' conversation queries, don't clear all conversation state
    // Clearing all state on every mount causes users to lose their conversation when navigating
    queryClient.removeQueries([QueryKeys.messages, 'new']);
    
    // DO NOT call clearConvoState() here - it wipes all conversations on every route change
    // clearConvoState() should only be called explicitly when needed (e.g., logout, new chat)
  }, [queryClient]);

  if (!isAuthenticated) {
    return null;
  }

  return (
    <DashboardContext.Provider value={{ prevLocationPath }}>
      <div className="h-screen w-full">
        <Outlet />
      </div>
    </DashboardContext.Provider>
  );
}
