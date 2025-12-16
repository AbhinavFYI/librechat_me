import { useState, useEffect, useCallback } from 'react';
import { ChevronDown, FileText } from 'lucide-react';
import * as Ariakit from '@ariakit/react';
import { DropdownPopup } from '@librechat/client';
import { saasApi } from '~/services/saasApi';
import { useParams } from 'react-router-dom';
import { Constants } from 'librechat-data-provider';

interface SavedTemplate {
  name: string;
  description: string;
  detailedPrompt: string;
  framework?: string;
  content?: Record<string, any>;
}

export default function TemplateSelector() {
  const [templates, setTemplates] = useState<SavedTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<string | null>(null);
  const { conversationId } = useParams<{ conversationId?: string }>();

  useEffect(() => {
    fetchData();
  }, []);

  // Get selected template from localStorage to show in button
  const updateSelectedTemplate = useCallback(() => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Try actual conversationId first, then fallback to NEW_CONVO
    let templateData = localStorage.getItem(`template_data_${convoId}`);
    
    // If no data found for actual conversationId and we're not on NEW_CONVO, also check NEW_CONVO
    if (!templateData && convoId !== Constants.NEW_CONVO) {
      templateData = localStorage.getItem(`template_data_${Constants.NEW_CONVO}`);
    }
    
    if (templateData) {
      try {
        const data = JSON.parse(templateData);
        setSelectedTemplate(data.template || data.name || null);
      } catch (e) {
        setSelectedTemplate(null);
      }
    } else {
      setSelectedTemplate(null);
    }
  }, [conversationId]);

  useEffect(() => {
    updateSelectedTemplate();
    
    // Listen for custom events to update selection
    const handleTemplateUpdate = () => updateSelectedTemplate();
    window.addEventListener('templateUpdated', handleTemplateUpdate);
    
    return () => {
      window.removeEventListener('templateUpdated', handleTemplateUpdate);
    };
  }, [updateSelectedTemplate, isOpen]);

  const fetchData = async () => {
    setLoading(true);
    try {
      console.log('[TemplateSelector] Fetching templates from backend...');
      const response = await saasApi.getTemplates();
      console.log('[TemplateSelector] Raw API response:', response);
      
      // Handle paginated response format: {data: Array, page, limit, total, total_pages}
      let templatesArray: any[] = [];
      if (response) {
        if (Array.isArray(response)) {
          templatesArray = response;
        } else {
          const responseAny = response as any;
          if (responseAny.data && Array.isArray(responseAny.data)) {
            templatesArray = responseAny.data;
          }
        }
      }
      
      if (templatesArray.length > 0) {
        const parsedTemplates: SavedTemplate[] = templatesArray.map((item: any) => {
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
            name: item.name || item.template || 'Unnamed Template',
            description: item.description || '',
            detailedPrompt: detailedPrompt || item.name || '',
            framework: item.framework || '',
            content: item.content || {}
          };
        });
        
        console.log('[TemplateSelector] Parsed templates:', parsedTemplates);
        setTemplates(parsedTemplates);
      } else {
        console.warn('[TemplateSelector] No templates found in response:', response);
        setTemplates([]);
      }
    } catch (error) {
      console.error('[TemplateSelector] Error fetching templates:', error);
      setTemplates([]);
    } finally {
      setLoading(false);
    }
  };

  const handleSelectTemplate = async (template: SavedTemplate) => {
    const convoId = conversationId || Constants.NEW_CONVO;
    
    // Store template data
    const templateData = {
      template: template.name,
      name: template.name,
      description: template.description || '',
      framework: (template as any).framework || '',
      detailedPrompt: template.detailedPrompt || template.description || template.name,
      content: (template as any).content || { custom: template.detailedPrompt || template.description || template.name }
    };
    localStorage.setItem(`template_data_${convoId}`, JSON.stringify(templateData));
    // Dispatch custom event to notify other components (like SelectedTemplate)
    window.dispatchEvent(new Event('templateUpdated'));
    setSelectedTemplate(template.name);
    console.log('✅ Template selected and stored:', template.name, templateData);
    setIsOpen(false);
  };

  const handleClearTemplate = () => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Clear from both actual conversationId and NEW_CONVO to ensure it's removed
    localStorage.removeItem(`template_data_${convoId}`);
    if (convoId !== Constants.NEW_CONVO) {
      localStorage.removeItem(`template_data_${Constants.NEW_CONVO}`);
    }
    // Dispatch custom event to notify other components
    window.dispatchEvent(new Event('templateUpdated'));
    setSelectedTemplate(null);
    console.log('🗑️ Template cleared');
    setIsOpen(false);
  };

  const menuItems = [
    ...templates.map((template) => {
      // Show template structure/content instead of just name
      const templateContent = template.detailedPrompt || template.description || template.name;
      // Format template content to show structure in a compact way
      const formatTemplateContent = (content: string): string => {
        // Split by lines and extract key parts
        const lines = content.split('\n').filter(line => line.trim());
        if (lines.length === 0) return content;
        
        // Try to extract ROLE, TASK, FORMAT from the structure
        let role = '';
        let task = '';
        let format = '';
        
        lines.forEach((line, idx) => {
          const lowerLine = line.toLowerCase();
          if (lowerLine.includes('role') || lowerLine.includes('act as')) {
            role = line.replace(/.*(?:role|act as)[:\s]*/i, '').trim();
          } else if (lowerLine.includes('task') || lowerLine.includes('create')) {
            task = line.replace(/.*(?:task|create)[:\s]*/i, '').trim();
          } else if (lowerLine.includes('format') || lowerLine.includes('show as')) {
            format = line.replace(/.*(?:format|show as)[:\s]*/i, '').trim();
          }
        });
        
        // Build compact display
        const parts: string[] = [];
        if (role) parts.push(`Role: ${role.substring(0, 20)}`);
        if (task) parts.push(`Task: ${task.substring(0, 30)}`);
        if (format) parts.push(`Format: ${format}`);
        
        if (parts.length > 0) {
          return parts.join(' | ');
        }
        
        // Fallback: show first line or truncated content
        return lines[0]?.substring(0, 50) || content.substring(0, 50);
      };
      
      const formattedContent = formatTemplateContent(templateContent);
      const displayLabel = `${template.name}${selectedTemplate === template.name ? ' ✓' : ''} - ${formattedContent}`;
      
      // Check if this template is currently selected (by comparing with localStorage)
      const convoId = conversationId || Constants.NEW_CONVO;
      let isSelected = false;
      let templateDataStr = localStorage.getItem(`template_data_${convoId}`);
      if (!templateDataStr && convoId !== Constants.NEW_CONVO) {
        templateDataStr = localStorage.getItem(`template_data_${Constants.NEW_CONVO}`);
      }
      if (templateDataStr) {
        try {
          const templateData = JSON.parse(templateDataStr);
          isSelected = (templateData.template || templateData.name) === template.name;
        } catch (e) {
          // Ignore parse errors
        }
      }
      
      return {
        label: displayLabel,
      onClick: () => handleSelectTemplate(template),
        icon: isSelected ? '✓' : '',
      key: `template-${template.name}`,
      };
    }),
    ...(selectedTemplate ? [{
      separate: true,
      key: 'separator',
    }, {
      label: '🗑️ Clear',
      onClick: handleClearTemplate,
      key: 'clear-template',
    }] : []),
  ];

  if (loading) {
    return (
      <button
        type="button"
        disabled
        className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary opacity-50"
      >
        <FileText className="h-4 w-4" />
        <span>Loading...</span>
      </button>
    );
  }

  const buttonText = selectedTemplate 
    ? `${selectedTemplate}` 
    : templates.length > 0 
      ? 'Pick Template' 
      : 'No Templates';

  if (menuItems.length === 0) {
    return (
      <button
        type="button"
        disabled
        className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary opacity-50"
        title="No templates available"
      >
        <FileText className="h-4 w-4" />
        <span>{buttonText}</span>
      </button>
    );
  }

  return (
    <div className="relative">
      <DropdownPopup
        portal={false}
        menuId="template-selector"
        isOpen={isOpen}
        setIsOpen={setIsOpen}
        trigger={
          <Ariakit.MenuButton
            className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary transition-all hover:bg-surface-hover"
          >
            <FileText className="h-4 w-4" />
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
