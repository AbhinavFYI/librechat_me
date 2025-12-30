import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { FileText, X, Check } from 'lucide-react';
import { useParams } from 'react-router-dom';
import { Constants } from 'librechat-data-provider';
import { HoverCard, HoverCardTrigger, HoverCardContent, HoverCardPortal } from '@librechat/client';
import { cn } from '~/utils';

interface StoredDocument {
  filename?: string;
  name?: string;
  document_id: number;
  file_path?: string;
  status?: string;
}

export default function SelectedDocuments() {
  const { conversationId } = useParams<{ conversationId?: string }>();
  const [selectedDocuments, setSelectedDocuments] = useState<StoredDocument[]>([]);

  const loadSelectedDocuments = useCallback(() => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Try actual conversationId first, then fallback to NEW_CONVO
    let documentDataStr = localStorage.getItem(`persona_documents_${convoId}`);
    
    // If no data found for actual conversationId and we're not on NEW_CONVO, also check NEW_CONVO
    if (!documentDataStr && convoId !== Constants.NEW_CONVO) {
      documentDataStr = localStorage.getItem(`persona_documents_${Constants.NEW_CONVO}`);
    }
    
    if (documentDataStr) {
      try {
        const documentData = JSON.parse(documentDataStr);
        if (documentData.documents && Array.isArray(documentData.documents)) {
          setSelectedDocuments(documentData.documents as StoredDocument[]);
        } else {
          setSelectedDocuments([]);
        }
      } catch (error) {
        console.error('Error parsing document data:', error);
        setSelectedDocuments([]);
      }
    } else {
      setSelectedDocuments([]);
    }
  }, [conversationId]);

  useEffect(() => {
    // Load immediately on mount
    loadSelectedDocuments();
    
    // Listen for custom events (for same-tab updates) - with immediate execution
    const handleCustomStorageChange = () => {
      // Use requestAnimationFrame to ensure UI updates immediately
      requestAnimationFrame(() => {
        loadSelectedDocuments();
      });
    };
    
    // Also poll localStorage for instant updates (in case events don't fire)
    const intervalId = setInterval(() => {
      loadSelectedDocuments();
    }, 200); // Check every 200ms for instant updates
    
    window.addEventListener('documentsUpdated', handleCustomStorageChange);
    
    // Also listen to storage events for cross-tab updates
    const handleStorageChange = (e: StorageEvent) => {
      const convoId = conversationId || Constants.NEW_CONVO;
      if (e.key === `persona_documents_${convoId}`) {
        requestAnimationFrame(() => {
          loadSelectedDocuments();
        });
      }
    };
    
    window.addEventListener('storage', handleStorageChange);
    
    return () => {
      clearInterval(intervalId);
      window.removeEventListener('documentsUpdated', handleCustomStorageChange);
      window.removeEventListener('storage', handleStorageChange);
    };
  }, [conversationId, loadSelectedDocuments]);

  const handleRemoveAll = useCallback(() => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Clear from both actual conversationId and NEW_CONVO to ensure it's removed
    localStorage.removeItem(`persona_documents_${convoId}`);
    if (convoId !== Constants.NEW_CONVO) {
      localStorage.removeItem(`persona_documents_${Constants.NEW_CONVO}`);
    }
    setSelectedDocuments([]);
    // Dispatch custom event to notify other components
    window.dispatchEvent(new Event('documentsUpdated'));
  }, [conversationId]);

  const documentCount = useMemo(() => selectedDocuments.length, [selectedDocuments.length]);

  const documentNames = useMemo(() => {
    return selectedDocuments
      .map(doc => doc.filename || doc.name || 'Unknown')
      .join('\n');
  }, [selectedDocuments]);

  if (documentCount === 0) {
    return null;
  }

  return (
    <HoverCard openDelay={150}>
      <HoverCardTrigger asChild>
    <div
      className={cn(
        'flex items-center gap-1.5 rounded-lg border border-border-light bg-surface-secondary px-2 py-1 text-xs',
            'hover:bg-surface-hover transition-colors cursor-default'
      )}
    >
      <Check className="h-3 w-3 text-text-secondary flex-shrink-0" />
      <FileText className="h-3 w-3 text-text-secondary flex-shrink-0" />
      <span className="text-text-primary whitespace-nowrap">
        {documentCount} {documentCount === 1 ? 'doc' : 'docs'}
      </span>
      <button
        type="button"
        onClick={handleRemoveAll}
        className="ml-0.5 flex-shrink-0 rounded p-0.5 hover:bg-surface-hover"
        aria-label="Remove all documents"
      >
        <X className="h-3 w-3 text-text-secondary" />
      </button>
    </div>
      </HoverCardTrigger>
      <HoverCardPortal>
        <HoverCardContent side="bottom" className="w-auto max-w-md">
          <div className="space-y-1">
            {selectedDocuments.map((doc, index) => (
              <p key={index} className="text-xs text-text-secondary whitespace-nowrap">
                • {doc.filename || doc.name || 'Unknown'}
              </p>
            ))}
          </div>
        </HoverCardContent>
      </HoverCardPortal>
    </HoverCard>
  );
}

