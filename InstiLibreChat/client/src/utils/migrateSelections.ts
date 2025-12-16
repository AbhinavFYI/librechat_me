import { Constants } from 'librechat-data-provider';

/**
 * Migrates selections (persona, template, documents) from NEW_CONVO to actual conversationId
 * when a new conversation is created
 */
export function migrateSelectionsToConversation(newConversationId: string) {
  if (!newConversationId || newConversationId === Constants.NEW_CONVO) {
    return;
  }

  const keysToMigrate = [
    'persona_data',
    'template_data',
    'persona_documents',
  ];

  keysToMigrate.forEach((keyPrefix) => {
    const newConvoKey = `${keyPrefix}_${Constants.NEW_CONVO}`;
    const actualConvoKey = `${keyPrefix}_${newConversationId}`;

    // Check if data exists for NEW_CONVO
    const newConvoData = localStorage.getItem(newConvoKey);
    
    if (newConvoData) {
      // Always copy to actual conversation (overwrite if exists to ensure latest data)
      localStorage.setItem(actualConvoKey, newConvoData);
      console.log(`[Migration] Migrated ${keyPrefix} from NEW_CONVO to ${newConversationId}`);
      
      // IMPORTANT: Keep the data in NEW_CONVO as well so it persists until user navigates away
      // This ensures the selections stay visible even after the conversation is created
      // The data will be available for both NEW_CONVO and the actual conversationId
    }
  });

  // Dispatch events to notify components
  window.dispatchEvent(new Event('personaUpdated'));
  window.dispatchEvent(new Event('templateUpdated'));
  window.dispatchEvent(new Event('documentsUpdated'));
}

