import { useEffect, useRef } from 'react';
import { useResetRecoilState, useSetRecoilState } from 'recoil';
import { Constants } from 'librechat-data-provider';
import { logger } from '~/utils';
import store from '~/store';

/**
 * Hook to reset visible artifacts and clear submission state when the conversation ID changes
 * This prevents re-submission when navigating back to an existing conversation
 * @param conversationId - The current conversation ID
 */
export default function useIdChangeEffect(conversationId: string) {
  const lastConvoId = useRef<string | null>(null);
  const resetVisibleArtifacts = useResetRecoilState(store.visibleArtifacts);
  const setSubmission = useSetRecoilState(store.submissionByIndex(0));

  useEffect(() => {
    if (conversationId !== lastConvoId.current && lastConvoId.current !== null) {
      logger.log('conversation', 'Conversation ID change', { from: lastConvoId.current, to: conversationId });
      resetVisibleArtifacts();
      
      // Clear submission state when navigating to an existing conversation (not NEW_CONVO)
      // This prevents re-submission/regeneration when navigating back from other pages
      if (conversationId !== Constants.NEW_CONVO && conversationId !== 'new') {
        logger.log('conversation', 'Clearing submission state for existing conversation', conversationId);
        setSubmission(null);
      }
    }
    lastConvoId.current = conversationId;
  }, [conversationId, resetVisibleArtifacts, setSubmission]);
}
