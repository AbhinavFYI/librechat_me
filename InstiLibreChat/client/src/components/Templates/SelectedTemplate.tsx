import React, { useState, useEffect, useCallback } from 'react';
import { X, Check } from 'lucide-react';
import { useParams } from 'react-router-dom';
import { Constants } from 'librechat-data-provider';
import { cn } from '~/utils';

export default function SelectedTemplate() {
  const { conversationId } = useParams<{ conversationId?: string }>();
  const [templateName, setTemplateName] = useState<string>('');

  const loadSelectedTemplate = useCallback(() => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Try actual conversationId first, then fallback to NEW_CONVO
    let templateDataStr = localStorage.getItem(`template_data_${convoId}`);
    
    // If no data found for actual conversationId and we're not on NEW_CONVO, also check NEW_CONVO
    if (!templateDataStr && convoId !== Constants.NEW_CONVO) {
      templateDataStr = localStorage.getItem(`template_data_${Constants.NEW_CONVO}`);
    }
    
    if (templateDataStr) {
      try {
        const templateData = JSON.parse(templateDataStr);
        console.log('[SelectedTemplate] Loaded template data:', templateData);
        
        // Priority: name > template > description > framework > first lines of detailedPrompt
        if (templateData.name) {
          setTemplateName(templateData.name);
          console.log('[SelectedTemplate] Using name field:', templateData.name);
        } else if (templateData.template) {
          setTemplateName(templateData.template);
          console.log('[SelectedTemplate] Using template field:', templateData.template);
        } else if (templateData.description) {
          const truncated = templateData.description.length > 40 
            ? `${templateData.description.substring(0, 40)}...` 
            : templateData.description;
          setTemplateName(truncated);
          console.log('[SelectedTemplate] Using description field:', truncated);
        } else if (templateData.framework) {
          setTemplateName(templateData.framework);
          console.log('[SelectedTemplate] Using framework field:', templateData.framework);
        } else if (templateData.detailedPrompt) {
          // Show the template structure (first few lines or truncated) as fallback
          const lines = templateData.detailedPrompt.split('\n').filter(line => line.trim());
          const preview = lines.slice(0, 3).join(' | '); // Show first 3 lines
          const truncated = preview.length > 40 ? `${preview.substring(0, 40)}...` : preview;
          setTemplateName(truncated);
          console.log('[SelectedTemplate] Using detailedPrompt:', truncated);
        } else {
          setTemplateName('Template');
          console.log('[SelectedTemplate] Using fallback: Template');
        }
      } catch (error) {
        console.error('[SelectedTemplate] Error parsing template data:', error);
        setTemplateName('');
      }
    } else {
      console.log('[SelectedTemplate] No template data found in localStorage');
      setTemplateName('');
    }
  }, [conversationId]);

  useEffect(() => {
    // Load immediately on mount
    loadSelectedTemplate();
    
    // Listen for custom events (for same-tab updates)
    const handleCustomStorageChange = () => {
      requestAnimationFrame(() => {
        loadSelectedTemplate();
      });
    };
    
    // Poll localStorage for instant updates
    const intervalId = setInterval(() => {
      loadSelectedTemplate();
    }, 200);
    
    window.addEventListener('templateUpdated', handleCustomStorageChange);
    
    // Listen to storage events for cross-tab updates
    const handleStorageChange = (e: StorageEvent) => {
      const convoId = conversationId || Constants.NEW_CONVO;
      if (e.key === `template_data_${convoId}`) {
        requestAnimationFrame(() => {
          loadSelectedTemplate();
        });
      }
    };
    
    window.addEventListener('storage', handleStorageChange);
    
    return () => {
      clearInterval(intervalId);
      window.removeEventListener('templateUpdated', handleCustomStorageChange);
      window.removeEventListener('storage', handleStorageChange);
    };
  }, [conversationId, loadSelectedTemplate]);

  const handleRemove = useCallback(() => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Clear from both actual conversationId and NEW_CONVO to ensure it's removed
    localStorage.removeItem(`template_data_${convoId}`);
    if (convoId !== Constants.NEW_CONVO) {
      localStorage.removeItem(`template_data_${Constants.NEW_CONVO}`);
    }
    setTemplateName('');
    // Dispatch custom event to notify other components
    window.dispatchEvent(new Event('templateUpdated'));
  }, [conversationId]);

  if (!templateName) {
    return null;
  }

  // Get full template content for tooltip
  const convoId = conversationId || Constants.NEW_CONVO;
  const templateDataStr = localStorage.getItem(`template_data_${convoId}`);
  let fullTemplateContent = templateName;
  if (templateDataStr) {
    try {
      const templateData = JSON.parse(templateDataStr);
      fullTemplateContent = templateData.detailedPrompt || templateData.description || templateName;
    } catch (e) {
      // Use templateName as fallback
    }
  }

  const displayName = templateName.length > 30 ? `${templateName.substring(0, 30)}...` : templateName;

  return (
    <div
      className={cn(
        'flex items-center gap-1.5 rounded-lg border border-border-light bg-surface-secondary px-2 py-1 text-xs',
        'hover:bg-surface-hover transition-colors'
      )}
    >
      <Check className="h-3 w-3 text-text-secondary flex-shrink-0" />
      <img 
        src="/assets/documents.svg" 
        alt="Template" 
        className="h-3 w-3 text-text-secondary flex-shrink-0 opacity-70 dark:brightness-0 dark:invert dark:opacity-70" 
      />
      <span className="text-text-primary whitespace-nowrap" title={fullTemplateContent}>
        {displayName}
      </span>
      <button
        type="button"
        onClick={handleRemove}
        className="ml-0.5 flex-shrink-0 rounded p-0.5 hover:bg-surface-hover"
        aria-label="Remove template"
      >
        <X className="h-3 w-3 text-text-secondary" />
      </button>
    </div>
  );
}

