import { useState, useEffect, useCallback } from 'react';
import { ChevronDown, FolderOpen } from 'lucide-react';
import * as Ariakit from '@ariakit/react';
import { DropdownPopup } from '@librechat/client';
import { saasApi } from '~/services/saasApi';
import { useParams } from 'react-router-dom';
import { Constants } from 'librechat-data-provider';

// Default personas as per guide
const defaultPersonas = [
  {
    name: 'FIA (default)',
    description: 'A helpful AI assistant that provides balanced and informative responses.',
    detailedPrompt: 'You are a helpful AI assistant that provides balanced and informative responses. Focus on being accurate, clear, and helpful in all interactions.'
  },
  {
    name: 'Risk manager',
    description: 'An expert in risk assessment and management, focusing on identifying, analyzing, and mitigating potential risks in various scenarios.',
    detailedPrompt: 'You are a seasoned risk manager with 15+ years of experience in enterprise risk management. You excel at identifying potential risks, analyzing their impact and probability, and developing comprehensive mitigation strategies. Focus on quantitative risk assessment, regulatory compliance, and strategic risk planning. Always consider both financial and operational risks in your analysis.'
  },
  {
    name: 'Investment advisor',
    description: 'An experienced investment advisor providing comprehensive guidance on portfolio management, asset allocation, and investment strategies.',
    detailedPrompt: 'You are an experienced investment advisor with deep expertise in portfolio management, asset allocation, and market analysis. You provide comprehensive guidance on investment strategies, risk-adjusted returns, and long-term wealth building. Focus on fundamental analysis, diversification strategies, and client-specific investment goals. Always consider risk tolerance, time horizon, and financial objectives in your recommendations.'
  },
  {
    name: 'Technical analyst',
    description: 'A skilled technical analyst specializing in chart patterns, technical indicators, and quantitative trading strategies.',
    detailedPrompt: 'You are a skilled technical analyst with expertise in chart pattern recognition, technical indicators, and quantitative trading strategies. You excel at analyzing price movements, volume trends, and market momentum. Focus on identifying entry/exit points, support/resistance levels, and trend analysis. Use technical indicators like RSI, MACD, Moving Averages, and Fibonacci retracements in your analysis.'
  },
  {
    name: 'ESG specialist',
    description: 'An ESG specialist with comprehensive knowledge of Environmental, Social, and Governance factors in investment decisions.',
    detailedPrompt: 'You are an ESG specialist with comprehensive knowledge of Environmental, Social, and Governance factors in investment decisions. You analyze companies based on their sustainability practices, social responsibility, and corporate governance. Focus on ESG metrics, sustainability reports, impact investing, and responsible business practices. Always consider long-term sustainability and stakeholder value in your analysis.'
  },
];

// Default templates as per guide
const defaultTemplates = [
  {
    name: 'Quarterly report',
    description: 'Template for quarterly financial reporting',
    detailedPrompt: 'Generate a comprehensive quarterly financial report including: Executive Summary, Financial Highlights, Revenue Analysis, Expense Breakdown, Key Performance Indicators, Market Outlook, Risk Factors, and Management Commentary. Use clear formatting with tables and charts where appropriate.'
  },
  {
    name: 'Annual report',
    description: 'Template for annual financial reporting',
    detailedPrompt: 'Create a detailed annual report with: Letter to Shareholders, Business Overview, Financial Performance Review, Market Analysis, Strategic Initiatives, Risk Management, Corporate Governance, and Forward-Looking Statements. Include year-over-year comparisons and detailed financial metrics.'
  },
  {
    name: 'Investment thesis',
    description: 'Template for investment thesis development',
    detailedPrompt: 'Develop a comprehensive investment thesis including: Company Overview, Industry Analysis, Competitive Positioning, Financial Analysis, Valuation, Key Catalysts, Risk Factors, and Investment Recommendation. Support arguments with data and market research.'
  },
  {
    name: 'Risk assessment',
    description: 'Template for risk analysis and assessment',
    detailedPrompt: 'Conduct a thorough risk assessment covering: Risk Identification, Risk Analysis (probability and impact), Risk Evaluation, Risk Mitigation Strategies, Monitoring Plan, and Contingency Planning. Use risk matrices and quantitative methods where applicable.'
  },
  {
    name: 'Market summary',
    description: 'Template for market overview and summary',
    detailedPrompt: 'Provide a comprehensive market summary including: Market Overview, Key Developments, Sector Performance, Economic Indicators, Major News Impact, Trading Volume Analysis, and Market Outlook. Focus on actionable insights and market implications.'
  }
];

interface SavedPersona {
  name: string;
  description: string;
  detailedPrompt: string;
}

interface SavedTemplate {
  name: string;
  description: string;
  detailedPrompt: string;
  framework?: string;
  content?: Record<string, any>;
}

export default function TemplatePersonaSelector() {
  const [templates, setTemplates] = useState<SavedTemplate[]>([]);
  const [personas, setPersonas] = useState<SavedPersona[]>([]);
  const [loading, setLoading] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<string | null>(null);
  const [selectedPersona, setSelectedPersona] = useState<string | null>(null);
  const { conversationId } = useParams<{ conversationId?: string }>();

  useEffect(() => {
    fetchData();
  }, []);

  // Get selected template and persona from localStorage to show in button
  const updateSelectedItems = useCallback(() => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Try actual conversationId first, then fallback to NEW_CONVO
    let templateData = localStorage.getItem(`template_data_${convoId}`);
    let personaData = localStorage.getItem(`persona_data_${convoId}`);
    
    // If no data found for actual conversationId and we're not on NEW_CONVO, also check NEW_CONVO
    if (!templateData && convoId !== Constants.NEW_CONVO) {
      templateData = localStorage.getItem(`template_data_${Constants.NEW_CONVO}`);
    }
    if (!personaData && convoId !== Constants.NEW_CONVO) {
      personaData = localStorage.getItem(`persona_data_${Constants.NEW_CONVO}`);
    }
    
    // Get selected template
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
    
    // Get selected persona (can be selected alongside template)
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
    updateSelectedItems();
    
    // Listen for custom events to update selections
    const handleTemplateUpdate = () => updateSelectedItems();
    const handlePersonaUpdate = () => updateSelectedItems();
    
    window.addEventListener('templateUpdated', handleTemplateUpdate);
    window.addEventListener('personaUpdated', handlePersonaUpdate);
    
    return () => {
      window.removeEventListener('templateUpdated', handleTemplateUpdate);
      window.removeEventListener('personaUpdated', handlePersonaUpdate);
    };
  }, [updateSelectedItems, isOpen]); // Update when dropdown opens/closes

  const fetchData = async () => {
    setLoading(true);
    try {
      console.log('[TemplatePersonaSelector] Fetching templates and personas from backend API...');
      
      // Fetch ONLY from backend API (database)
      const [templatesData, personasData] = await Promise.all([
        saasApi.getTemplates(),
        saasApi.getPersonas(),
      ]);

      console.log('[TemplatePersonaSelector] Raw templates response:', templatesData);
      console.log('[TemplatePersonaSelector] Raw personas response:', personasData);

      // Parse templates from backend
      let backendTemplates: any[] = [];
      if (templatesData) {
        if (Array.isArray((templatesData as any).data)) {
          backendTemplates = (templatesData as any).data;
        } else if (Array.isArray(templatesData)) {
          backendTemplates = templatesData;
        } else if ((templatesData as any).data) {
          backendTemplates = Array.isArray((templatesData as any).data) ? (templatesData as any).data : [];
        }
      }

      // Parse personas from backend
      let backendPersonas: any[] = [];
      if (personasData) {
        if (Array.isArray((personasData as any).data)) {
          backendPersonas = (personasData as any).data;
        } else if (Array.isArray(personasData)) {
          backendPersonas = personasData;
        } else if ((personasData as any).data) {
          backendPersonas = Array.isArray((personasData as any).data) ? (personasData as any).data : [];
        }
      }

      console.log('[TemplatePersonaSelector] Parsed templates:', backendTemplates.length, backendTemplates);
      console.log('[TemplatePersonaSelector] Parsed personas:', backendPersonas.length, backendPersonas);

      // Convert backend format to our format
      const formattedTemplates: SavedTemplate[] = backendTemplates.map((t: any) => {
        // Extract detailedPrompt from various possible fields
        let detailedPrompt = '';
        if (t.content?.custom) {
          detailedPrompt = t.content.custom;
        } else if (typeof t.content === 'string') {
          detailedPrompt = t.content;
        } else if (t.detailedPrompt) {
          detailedPrompt = t.detailedPrompt;
        } else if (t.description) {
          detailedPrompt = t.description;
        } else if (t.framework) {
          detailedPrompt = `Template: ${t.name} (${t.framework})`;
        } else {
          detailedPrompt = t.name;
        }

        return {
          name: t.name || 'Unnamed Template',
          description: t.description || t.framework || '',
          detailedPrompt: detailedPrompt,
          framework: t.framework || '',
          content: t.content || {}
        };
      });

      const formattedPersonas: SavedPersona[] = backendPersonas.map((p: any) => {
        // Extract detailedPrompt from various possible fields
        let detailedPrompt = '';
        if (p.content?.custom) {
          detailedPrompt = p.content.custom;
        } else if (typeof p.content === 'string') {
          detailedPrompt = p.content;
        } else if (p.detailedPrompt) {
          detailedPrompt = p.detailedPrompt;
        } else if (p.description) {
          detailedPrompt = p.description;
        } else {
          detailedPrompt = p.name || 'Unnamed Persona';
        }

        return {
          name: p.name || 'Unnamed Persona',
          description: p.description || '',
          detailedPrompt: detailedPrompt
        };
      });

      console.log('[TemplatePersonaSelector] Formatted templates:', formattedTemplates);
      console.log('[TemplatePersonaSelector] Formatted personas:', formattedPersonas);

      setTemplates(formattedTemplates);
      setPersonas(formattedPersonas);
    } catch (error) {
      console.error('[TemplatePersonaSelector] Error fetching from backend:', error);
      // On error, set empty arrays (don't use defaults)
      setTemplates([]);
      setPersonas([]);
    } finally {
      setLoading(false);
    }
  };

  const handleSelectTemplate = async (template: SavedTemplate) => {
    const convoId = conversationId || Constants.NEW_CONVO;
    
    // Store template data - preserve original content and framework fields
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
    // Update selected template immediately (don't clear persona - they can coexist)
    setSelectedTemplate(template.name);
    console.log('✅ Template selected and stored:', template.name, templateData);
    // Don't close dropdown - allow selecting persona too
  };

  const handleSelectPersona = async (persona: SavedPersona) => {
    const convoId = conversationId || Constants.NEW_CONVO;
    
    // Store persona data - use detailedPrompt as the main content
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
    // Update selected persona immediately (don't clear template - they can coexist)
    setSelectedPersona(persona.name);
    console.log('✅ Persona selected and stored:', persona.name, personaData);
    // Don't close dropdown - allow selecting template too
  };

  const handleClearAll = () => {
    const convoId = conversationId || Constants.NEW_CONVO;
    // Clear from both actual conversationId and NEW_CONVO to ensure it's removed
    localStorage.removeItem(`template_data_${convoId}`);
    localStorage.removeItem(`persona_data_${convoId}`);
    localStorage.removeItem(`persona_documents_${convoId}`);
    if (convoId !== Constants.NEW_CONVO) {
      localStorage.removeItem(`template_data_${Constants.NEW_CONVO}`);
      localStorage.removeItem(`persona_data_${Constants.NEW_CONVO}`);
      localStorage.removeItem(`persona_documents_${Constants.NEW_CONVO}`);
    }
    setSelectedTemplate(null);
    setSelectedPersona(null);
    // Dispatch events to notify all components
    window.dispatchEvent(new Event('templateUpdated'));
    window.dispatchEvent(new Event('personaUpdated'));
    window.dispatchEvent(new Event('documentsUpdated'));
    setIsOpen(false);
    console.log('✅ Cleared all selections');
  };

  // Build menu items with sections: Templates, Separator, Personas, Clear All
  const menuItems = [
    // Templates section - show template structure/content
    ...templates.map((template) => {
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
        
        lines.forEach((line) => {
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
      const displayLabel = `${template.name}${isSelected ? ' ✓' : ''} - ${formattedContent}`;
      
      return {
        label: displayLabel,
      onClick: () => {
        console.log('[TemplatePersonaSelector] Template clicked:', template);
        handleSelectTemplate(template);
      },
        icon: isSelected ? '✓' : '',
        key: `template-${template.name}`,
      };
    }),
    // Separator between templates and personas
    ...(templates.length > 0 && personas.length > 0 ? [{ separate: true }] : []),
    // Personas section
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
        label: ` ${persona.name}${isSelected ? ' ✓' : ''}`,
      onClick: () => {
        console.log('[TemplatePersonaSelector] Persona clicked:', persona);
        handleSelectPersona(persona);
      },
        icon: isSelected ? '✓' : '',
        key: `persona-${persona.name}`,
      };
    }),
    // Separator before clear option
    ...((templates.length > 0 || personas.length > 0) && (selectedTemplate || selectedPersona) ? [{ separate: true }] : []),
    // Clear all option (only show if something is selected)
    ...((selectedTemplate || selectedPersona) ? [{
      label: '🗑️ Clear All',
      onClick: handleClearAll,
    }] : []),
  ];

  console.log('[TemplatePersonaSelector] Menu items count:', menuItems.length, menuItems);

  if (loading) {
    return (
      <button
        type="button"
        disabled
        className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary opacity-50"
      >
        <FolderOpen className="h-4 w-4" />
        <span>Loading...</span>
      </button>
    );
  }

  // Build button text showing both template and persona if selected
  const getButtonText = () => {
    if (selectedTemplate && selectedPersona) {
      return `${selectedTemplate} + ${selectedPersona}`;
    } else if (selectedTemplate) {
      return ` ${selectedTemplate}`;
    } else if (selectedPersona) {
      return `${selectedPersona}`;
    } else {
      return templates.length > 0 || personas.length > 0
        ? 'Pick Template/Persona'
        : 'No Templates';
    }
  };

  const buttonText = getButtonText();

  // If no items, show disabled button with message
  if (menuItems.length === 0) {
    return (
      <button
        type="button"
        disabled
        className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary opacity-50"
        title="No templates or personas available"
      >
        <FolderOpen className="h-4 w-4" />
        <span>{buttonText}</span>
      </button>
    );
  }

  return (
    <div className="relative">
      <DropdownPopup
        portal={false}
        menuId="template-persona-selector"
        isOpen={isOpen}
        setIsOpen={setIsOpen}
        trigger={
          <Ariakit.MenuButton
            className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary transition-all hover:bg-surface-hover"
          >
            <FolderOpen className="h-4 w-4" />
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

