import React, { useState, useEffect, useCallback } from 'react';
import { User, X, Check } from 'lucide-react';
import { useParams } from 'react-router-dom';
import { Constants } from 'librechat-data-provider';
import { cn } from '~/utils';

export default function SelectedPersona() {
  const { conversationId } = useParams<{ conversationId?: string }>();
  const [personaName, setPersonaName] = useState<string>('');

  const loadSelectedPersona = useCallback(() => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Try actual conversationId first, then fallback to NEW_CONVO
    let personaDataStr = localStorage.getItem(`persona_data_${convoId}`);
    
    // If no data found for actual conversationId and we're not on NEW_CONVO, also check NEW_CONVO
    if (!personaDataStr && convoId !== Constants.NEW_CONVO) {
      personaDataStr = localStorage.getItem(`persona_data_${Constants.NEW_CONVO}`);
    }
    
    if (personaDataStr) {
      try {
        const personaData = JSON.parse(personaDataStr);
        // Extract persona name from various possible fields
        if (personaData.detailedPrompt) {
          // Get first line or first 30 chars as name
          const firstLine = personaData.detailedPrompt.split('\n')[0].trim();
          setPersonaName(firstLine.substring(0, 30) || 'Persona');
        } else if (personaData.persona || personaData.name) {
          setPersonaName(personaData.persona || personaData.name);
        } else if (personaData.description) {
          setPersonaName(personaData.description.substring(0, 30));
        } else {
          setPersonaName('Persona');
        }
      } catch (error) {
        console.error('Error parsing persona data:', error);
        setPersonaName('');
      }
    } else {
      setPersonaName('');
    }
  }, [conversationId]);

  useEffect(() => {
    // Load immediately on mount
    loadSelectedPersona();
    
    // Listen for custom events (for same-tab updates)
    const handleCustomStorageChange = () => {
      requestAnimationFrame(() => {
        loadSelectedPersona();
      });
    };
    
    // Poll localStorage for instant updates
    const intervalId = setInterval(() => {
      loadSelectedPersona();
    }, 200);
    
    window.addEventListener('personaUpdated', handleCustomStorageChange);
    
    // Listen to storage events for cross-tab updates
    const handleStorageChange = (e: StorageEvent) => {
      const convoId = conversationId || Constants.NEW_CONVO;
      if (e.key === `persona_data_${convoId}`) {
        requestAnimationFrame(() => {
          loadSelectedPersona();
        });
      }
    };
    
    window.addEventListener('storage', handleStorageChange);
    
    return () => {
      clearInterval(intervalId);
      window.removeEventListener('personaUpdated', handleCustomStorageChange);
      window.removeEventListener('storage', handleStorageChange);
    };
  }, [conversationId, loadSelectedPersona]);

  const handleRemove = useCallback(() => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Clear from both actual conversationId and NEW_CONVO to ensure it's removed
    localStorage.removeItem(`persona_data_${convoId}`);
    if (convoId !== Constants.NEW_CONVO) {
      localStorage.removeItem(`persona_data_${Constants.NEW_CONVO}`);
    }
    setPersonaName('');
    // Dispatch custom event to notify other components
    window.dispatchEvent(new Event('personaUpdated'));
  }, [conversationId]);

  if (!personaName) {
    return null;
  }

  const displayName = personaName.length > 20 ? `${personaName.substring(0, 20)}...` : personaName;

  return (
    <div
      className={cn(
        'flex items-center gap-1.5 rounded-lg border border-border-light bg-surface-secondary px-2 py-1 text-xs',
        'hover:bg-surface-hover transition-colors'
      )}
    >
      <Check className="h-3 w-3 text-text-secondary flex-shrink-0" />
      <User className="h-3 w-3 text-text-secondary flex-shrink-0" />
      <span className="text-text-primary whitespace-nowrap" title={personaName}>
        {displayName}
      </span>
      <button
        type="button"
        onClick={handleRemove}
        className="ml-0.5 flex-shrink-0 rounded p-0.5 hover:bg-surface-hover"
        aria-label="Remove persona"
      >
        <X className="h-3 w-3 text-text-secondary" />
      </button>
    </div>
  );
}

