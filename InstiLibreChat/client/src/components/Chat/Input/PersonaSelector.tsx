import { useState, useEffect, useCallback } from 'react';
import { ChevronDown, User } from 'lucide-react';
import * as Ariakit from '@ariakit/react';
import { DropdownPopup } from '@librechat/client';
import { saasApi } from '~/services/saasApi';
import { useParams } from 'react-router-dom';
import { Constants } from 'librechat-data-provider';

interface SavedPersona {
  name: string;
  description: string;
  detailedPrompt: string;
}

export default function PersonaSelector() {
  const [personas, setPersonas] = useState<SavedPersona[]>([]);
  const [loading, setLoading] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [selectedPersona, setSelectedPersona] = useState<string | null>(null);
  const { conversationId } = useParams<{ conversationId?: string }>();

  useEffect(() => {
    fetchData();
  }, []);

  // Get selected persona from localStorage to show in button
  const updateSelectedPersona = useCallback(() => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Try actual conversationId first, then fallback to NEW_CONVO
    let personaData = localStorage.getItem(`persona_data_${convoId}`);
    
    // If no data found for actual conversationId and we're not on NEW_CONVO, also check NEW_CONVO
    if (!personaData && convoId !== Constants.NEW_CONVO) {
      personaData = localStorage.getItem(`persona_data_${Constants.NEW_CONVO}`);
    }
    
    if (personaData) {
      try {
        const data = JSON.parse(personaData);
        setSelectedPersona(data.persona || data.name || null);
      } catch (e) {
        setSelectedPersona(null);
      }
    } else {
      setSelectedPersona(null);
    }
  }, [conversationId]);

  useEffect(() => {
    updateSelectedPersona();
    
    // Listen for custom events to update selection
    const handlePersonaUpdate = () => updateSelectedPersona();
    window.addEventListener('personaUpdated', handlePersonaUpdate);
    
    return () => {
      window.removeEventListener('personaUpdated', handlePersonaUpdate);
    };
  }, [updateSelectedPersona, isOpen]);

  const fetchData = async () => {
    setLoading(true);
    try {
      console.log('[PersonaSelector] Fetching personas from backend...');
      const response = await saasApi.getPersonas();
      console.log('[PersonaSelector] Raw API response:', response);
      
      // Handle paginated response format: {data: Array, page, limit, total, total_pages}
      let personasArray: any[] = [];
      if (response) {
        if (Array.isArray(response)) {
          personasArray = response;
        } else {
          const responseAny = response as any;
          if (responseAny.data && Array.isArray(responseAny.data)) {
            personasArray = responseAny.data;
          }
        }
      }
      
      if (personasArray.length > 0) {
        const parsedPersonas: SavedPersona[] = personasArray.map((item: any) => {
          // Try multiple fields to get detailedPrompt
          let detailedPrompt = '';
          if (item.content?.custom) {
            detailedPrompt = typeof item.content.custom === 'string' 
              ? item.content.custom 
              : JSON.stringify(item.content.custom);
          } else if (item.content && typeof item.content === 'string') {
            detailedPrompt = item.content;
          } else if (item.detailedPrompt) {
            detailedPrompt = item.detailedPrompt;
          } else if (item.description) {
            detailedPrompt = item.description;
          } else if (item.framework) {
            detailedPrompt = item.framework;
          }
          
          return {
            name: item.name || item.persona || 'Unnamed Persona',
            description: item.description || '',
            detailedPrompt: detailedPrompt || item.name || ''
          };
        });
        
        console.log('[PersonaSelector] Parsed personas:', parsedPersonas);
        setPersonas(parsedPersonas);
      } else {
        console.warn('[PersonaSelector] No personas found in response:', response);
        setPersonas([]);
      }
    } catch (error) {
      console.error('[PersonaSelector] Error fetching personas:', error);
      setPersonas([]);
    } finally {
      setLoading(false);
    }
  };

  const handleSelectPersona = async (persona: SavedPersona) => {
    const convoId = conversationId || Constants.NEW_CONVO;
    
    // Store persona data
    const personaData = {
      persona: persona.name,
      name: persona.name,
      description: persona.description || '',
      detailedPrompt: persona.detailedPrompt || persona.description || persona.name,
      content: { custom: persona.detailedPrompt || persona.description || persona.name }
    };
    localStorage.setItem(`persona_data_${convoId}`, JSON.stringify(personaData));
    // Dispatch custom event to notify other components (like SelectedPersona)
    window.dispatchEvent(new Event('personaUpdated'));
    setSelectedPersona(persona.name);
    console.log('✅ Persona selected and stored:', persona.name, personaData);
    setIsOpen(false);
  };

  const handleClearPersona = () => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Clear from both actual conversationId and NEW_CONVO to ensure it's removed
    localStorage.removeItem(`persona_data_${convoId}`);
    if (convoId !== Constants.NEW_CONVO) {
      localStorage.removeItem(`persona_data_${Constants.NEW_CONVO}`);
    }
    // Dispatch custom event to notify other components
    window.dispatchEvent(new Event('personaUpdated'));
    setSelectedPersona(null);
    console.log('🗑️ Persona cleared');
    setIsOpen(false);
  };

  const menuItems = [
    ...personas.map((persona) => {
      // Check if this persona is currently selected (by comparing with localStorage)
      const convoId = conversationId || Constants.NEW_CONVO;
      let isSelected = false;
      let personaDataStr = localStorage.getItem(`persona_data_${convoId}`);
      if (!personaDataStr && convoId !== Constants.NEW_CONVO) {
        personaDataStr = localStorage.getItem(`persona_data_${Constants.NEW_CONVO}`);
      }
      if (personaDataStr) {
        try {
          const personaData = JSON.parse(personaDataStr);
          isSelected = (personaData.persona || personaData.name) === persona.name;
        } catch (e) {
          // Ignore parse errors
        }
      }
      
      return {
        label: ` ${persona.name}`,
      onClick: () => handleSelectPersona(persona),
        icon: isSelected ? '✓' : '',
      key: `persona-${persona.name}`,
      };
    }),
    ...(selectedPersona ? [{
      separate: true,
      key: 'separator',
    }, {
      label: '🗑️ Clear',
      onClick: handleClearPersona,
      key: 'clear-persona',
    }] : []),
  ];

  if (loading) {
    return (
      <button
        type="button"
        disabled
        className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary opacity-50"
      >
        <User className="h-4 w-4" />
        <span>Loading...</span>
      </button>
    );
  }

  const buttonText = selectedPersona 
    ? ` ${selectedPersona}` 
    : personas.length > 0 
      ? 'Pick Persona' 
      : 'No Personas';

  if (menuItems.length === 0) {
    return (
      <button
        type="button"
        disabled
        className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary opacity-50"
        title="No personas available"
      >
        <User className="h-4 w-4" />
        <span>{buttonText}</span>
      </button>
    );
  }

  return (
    <div className="relative">
      <DropdownPopup
        portal={false}
        menuId="persona-selector"
        isOpen={isOpen}
        setIsOpen={setIsOpen}
        trigger={
          <Ariakit.MenuButton
            className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary transition-all hover:bg-surface-hover"
          >
            <User className="h-4 w-4" />
            <span>{buttonText}</span>
            <ChevronDown className="h-4 w-4" />
          </Ariakit.MenuButton>
        }
        items={menuItems}
        className="absolute left-0 top-full mt-2 min-w-[200px] z-50"
      />
    </div>
  );
}
