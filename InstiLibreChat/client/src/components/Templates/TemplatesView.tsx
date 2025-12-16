import { useState, useEffect, useRef } from 'react';
import { Button, Dialog, DialogContent, DialogHeader, DialogTitle, Input, TextareaAutosize } from '@librechat/client';
import { saasApi } from '~/services/saasApi';
import { createPortal } from 'react-dom';
import { FileText, User, Edit, Trash2, MoreVertical } from 'lucide-react';

export default function TemplatesView() {
  const [activeTab, setActiveTab] = useState<'personas' | 'templates'>('personas');
  const [templates, setTemplates] = useState<any[]>([]);
  const [personas, setPersonas] = useState<any[]>([]);
  const [templatesLoading, setTemplatesLoading] = useState(false);
  const [personasLoading, setPersonasLoading] = useState(false);
  const [selectedItem, setSelectedItem] = useState<{ type: 'template' | 'persona'; id: string } | null>(null);
  const [dropdownPosition, setDropdownPosition] = useState<{ top: number; right: number } | null>(null);
  const buttonRefs = useRef<Map<string, HTMLButtonElement>>(new Map());
  const [showCreateTemplateModal, setShowCreateTemplateModal] = useState(false);
  const [showCreatePersonaModal, setShowCreatePersonaModal] = useState(false);
  const [showEditTemplateModal, setShowEditTemplateModal] = useState(false);
  const [showEditPersonaModal, setShowEditPersonaModal] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<any>(null);
  const [selectedPersona, setSelectedPersona] = useState<any>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    fetchTemplates();
    fetchPersonas();
  }, []);

  const fetchTemplates = async () => {
    setTemplatesLoading(true);
    try {
      const data = await saasApi.getTemplates();
      const templatesList = Array.isArray((data as any).data) ? (data as any).data : Array.isArray(data) ? data : ((data as any).data || []);
      setTemplates(templatesList);
    } catch (error) {
      console.error('Error fetching templates:', error);
    } finally {
      setTemplatesLoading(false);
    }
  };

  const fetchPersonas = async () => {
    setPersonasLoading(true);
    try {
      const data = await saasApi.getPersonas();
      const personasList = Array.isArray((data as any).data) ? (data as any).data : Array.isArray(data) ? data : ((data as any).data || []);
      setPersonas(personasList);
    } catch (error) {
      console.error('Error fetching personas:', error);
    } finally {
      setPersonasLoading(false);
    }
  };

  const saveTemplate = async (template: any) => {
    try {
      if (template.id) {
        await saasApi.updateTemplate(template.id, template);
      } else {
        await saasApi.createTemplate(template);
      }
      await fetchTemplates();
    } catch (error) {
      throw error;
    }
  };

  const savePersona = async (persona: any) => {
    try {
      if (persona.id) {
        await saasApi.updatePersona(persona.id, persona);
      } else {
        await saasApi.createPersona(persona);
      }
      await fetchPersonas();
    } catch (error) {
      throw error;
    }
  };

  const deleteTemplate = async (id: string) => {
    try {
      await saasApi.deleteTemplate(id);
      await fetchTemplates();
    } catch (error) {
      throw error;
    }
  };

  const deletePersona = async (id: string) => {
    try {
      await saasApi.deletePersona(id);
      await fetchPersonas();
    } catch (error) {
      throw error;
    }
  };

  const handleEditTemplate = (template: any) => {
    setSelectedTemplate(template);
    setShowEditTemplateModal(true);
    setSelectedItem(null);
    setDropdownPosition(null);
  };

  const handleEditPersona = (persona: any) => {
    setSelectedPersona(persona);
    setShowEditPersonaModal(true);
    setSelectedItem(null);
    setDropdownPosition(null);
  };

  const handleDeleteTemplate = async (template: any) => {
    if (window.confirm(`Are you sure you want to delete "${template.name}"?`)) {
      setDeleting(true);
      try {
        await deleteTemplate(template.id);
      } finally {
        setDeleting(false);
      }
    }
    setSelectedItem(null);
    setDropdownPosition(null);
  };

  const handleDeletePersona = async (persona: any) => {
    if (window.confirm(`Are you sure you want to delete "${persona.name}"?`)) {
      setDeleting(true);
      try {
        await deletePersona(persona.id);
      } finally {
        setDeleting(false);
      }
    }
    setSelectedItem(null);
    setDropdownPosition(null);
  };

  const formatDate = (dateString: string) => {
    if (!dateString) return 'Unknown';
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  // Close dropdown when clicking outside
  useEffect(() => {
    if (!selectedItem) {
      setDropdownPosition(null);
      return;
    }

    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as HTMLElement;
      if (
        target.closest('.dropdown-trigger') ||
        target.closest('[role="dialog"]') ||
        target.closest('.modal') ||
        showCreateTemplateModal ||
        showCreatePersonaModal ||
        showEditTemplateModal ||
        showEditPersonaModal
      ) {
        return;
      }
      if (target.closest('.fixed.w-48')) {
        return;
      }
      setSelectedItem(null);
      setDropdownPosition(null);
    };

    const timeoutId = setTimeout(() => {
      document.addEventListener('mousedown', handleClickOutside);
    }, 100);

    return () => {
      clearTimeout(timeoutId);
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [selectedItem, showCreateTemplateModal, showCreatePersonaModal, showEditTemplateModal, showEditPersonaModal]);

  const currentItems = activeTab === 'templates' ? templates : personas;
  const isLoading = activeTab === 'templates' ? templatesLoading : personasLoading;

  return (
    <div className="h-full flex flex-col bg-white dark:bg-gray-850">
      {/* Tabs */}
      <div className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-6">
            <button
              onClick={() => {
                setActiveTab('personas');
                setSelectedItem(null);
                setDropdownPosition(null);
              }}
              className={`px-4 py-2 text-lg font-medium transition-colors ${
                activeTab === 'personas'
                  ? 'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600 dark:border-blue-400'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              Personas
            </button>
            <button
              onClick={() => {
                setActiveTab('templates');
                setSelectedItem(null);
                setDropdownPosition(null);
              }}
              className={`px-4 py-2 text-lg font-medium transition-colors ${
                activeTab === 'templates'
                  ? 'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600 dark:border-blue-400'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
              }`}
            >
              Templates
            </button>
          </div>
          <Button
            onClick={() => {
              if (activeTab === 'templates') {
                setShowCreateTemplateModal(true);
              } else {
                setShowCreatePersonaModal(true);
              }
            }}
            className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg font-medium"
          >
            + Create {activeTab === 'templates' ? 'template' : 'persona'}
          </Button>
        </div>
      </div>

      {/* Table */}
      <div className="flex-1 overflow-auto px-6 py-4">
        {isLoading ? (
          <div className="flex items-center justify-center h-64">
            <p className="text-gray-500 dark:text-gray-400">Loading...</p>
          </div>
        ) : currentItems.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64">
            <p className="text-gray-500 dark:text-gray-400 mb-4">
              No {activeTab === 'templates' ? 'templates' : 'personas'} created yet.
            </p>
            <Button
              onClick={() => {
                if (activeTab === 'templates') {
                  setShowCreateTemplateModal(true);
                } else {
                  setShowCreatePersonaModal(true);
                }
              }}
              className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg font-medium"
            >
              + Create {activeTab === 'templates' ? 'template' : 'persona'}
            </Button>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-800">
                <tr>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Name
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Short description
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Date created
                  </th>
                  <th scope="col" className="relative px-6 py-3">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {activeTab === 'templates' ? (
                  templates.map((template) => {
                    const isSelected = selectedItem?.type === 'template' && selectedItem.id === template.id;
                    return (
                      <tr
                        key={template.id}
                        className="hover:bg-gray-50 dark:hover:bg-gray-700/50"
                      >
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center gap-3">
                            <FileText className="h-5 w-5 text-blue-500 flex-shrink-0" />
                            <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                              {template.name}
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4">
                          <div className="text-sm text-gray-500 dark:text-gray-400 whitespace-pre-wrap max-w-md">
                            {(() => {
                              const templateContent = template.detailedPrompt || template.description || '';
                              if (!templateContent) return template.framework || 'No template content';
                              
                              // Format template content to show structure (Role, Task, Format)
                              const lines = templateContent.split('\n').filter(line => line.trim());
                              if (lines.length === 0) return templateContent;
                              
                              // Extract ROLE, TASK, FORMAT from the structure
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
                              
                              // Build formatted display showing the structure
                              if (role || task || format) {
                                const parts: string[] = [];
                                if (role) parts.push(`Role: ${role}`);
                                if (task) parts.push(`Task: ${task}`);
                                if (format) parts.push(`Format: ${format}`);
                                return parts.join('\n');
                              }
                              
                              // Fallback: show the full content (truncated if too long)
                              return templateContent.length > 200 
                                ? `${templateContent.substring(0, 200)}...` 
                                : templateContent;
                            })()}
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                          {formatDate(template.created_at)}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          <div className="relative inline-block text-left">
                            <button
                              ref={(el) => {
                                if (el) {
                                  buttonRefs.current.set(`template-${template.id}`, el);
                                } else {
                                  buttonRefs.current.delete(`template-${template.id}`);
                                }
                              }}
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                e.preventDefault();
                                const isCurrentlySelected = selectedItem?.type === 'template' && selectedItem.id === template.id;
                                if (isCurrentlySelected) {
                                  setSelectedItem(null);
                                  setDropdownPosition(null);
                                } else {
                                  const button = buttonRefs.current.get(`template-${template.id}`);
                                  if (button) {
                                    const rect = button.getBoundingClientRect();
                                    setDropdownPosition({
                                      top: rect.top - 8,
                                      right: window.innerWidth - rect.right,
                                    });
                                  }
                                  setSelectedItem({ type: 'template', id: template.id });
                                }
                              }}
                              className="dropdown-trigger p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
                              title="More options"
                            >
                              <MoreVertical className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                            </button>
                            {isSelected && dropdownPosition && createPortal(
                              <div
                                className="fixed w-48 bg-white dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 z-[9999]"
                                style={{
                                  top: `${dropdownPosition.top}px`,
                                  right: `${dropdownPosition.right}px`,
                                  transform: 'translateY(-100%)',
                                }}
                                onClick={(e) => e.stopPropagation()}
                              >
                                <div className="py-1">
                                  <button
                                    type="button"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      e.preventDefault();
                                      handleEditTemplate(template);
                                    }}
                                    className="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                                  >
                                    <Edit className="h-4 w-4" />
                                    Edit
                                  </button>
                                  <button
                                    type="button"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      e.preventDefault();
                                      handleDeleteTemplate(template);
                                    }}
                                    className="w-full text-left px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                                  >
                                    <Trash2 className="h-4 w-4" />
                                    Delete
                                  </button>
                                </div>
                              </div>,
                              document.body
                            )}
                          </div>
                        </td>
                      </tr>
                    );
                  })
                ) : (
                  personas.map((persona) => {
                    const isSelected = selectedItem?.type === 'persona' && selectedItem.id === persona.id;
                    return (
                      <tr
                        key={persona.id}
                        className="hover:bg-gray-50 dark:hover:bg-gray-700/50"
                      >
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center gap-3">
                            <User className="h-5 w-5 text-blue-500 flex-shrink-0" />
                            <div className="text-sm font-medium text-gray-900 dark:text-gray-100">
                              {persona.name}
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4">
                          <div className="text-sm text-gray-500 dark:text-gray-400">
                            {persona.description || 'No description'}
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                          {formatDate(persona.created_at)}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          <div className="relative inline-block text-left">
                            <button
                              ref={(el) => {
                                if (el) {
                                  buttonRefs.current.set(`persona-${persona.id}`, el);
                                } else {
                                  buttonRefs.current.delete(`persona-${persona.id}`);
                                }
                              }}
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                e.preventDefault();
                                const isCurrentlySelected = selectedItem?.type === 'persona' && selectedItem.id === persona.id;
                                if (isCurrentlySelected) {
                                  setSelectedItem(null);
                                  setDropdownPosition(null);
                                } else {
                                  const button = buttonRefs.current.get(`persona-${persona.id}`);
                                  if (button) {
                                    const rect = button.getBoundingClientRect();
                                    setDropdownPosition({
                                      top: rect.top - 8,
                                      right: window.innerWidth - rect.right,
                                    });
                                  }
                                  setSelectedItem({ type: 'persona', id: persona.id });
                                }
                              }}
                              className="dropdown-trigger p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
                              title="More options"
                            >
                              <MoreVertical className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                            </button>
                            {isSelected && dropdownPosition && createPortal(
                              <div
                                className="fixed w-48 bg-white dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 z-[9999]"
                                style={{
                                  top: `${dropdownPosition.top}px`,
                                  right: `${dropdownPosition.right}px`,
                                  transform: 'translateY(-100%)',
                                }}
                                onClick={(e) => e.stopPropagation()}
                              >
                                <div className="py-1">
                                  <button
                                    type="button"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      e.preventDefault();
                                      handleEditPersona(persona);
                                    }}
                                    className="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                                  >
                                    <Edit className="h-4 w-4" />
                                    Edit
                                  </button>
                                  <button
                                    type="button"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      e.preventDefault();
                                      handleDeletePersona(persona);
                                    }}
                                    className="w-full text-left px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                                  >
                                    <Trash2 className="h-4 w-4" />
                                    Delete
                                  </button>
                                </div>
                              </div>,
                              document.body
                            )}
                          </div>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Modals */}
      {showCreateTemplateModal && (
        <CreateTemplateModal
          onClose={() => setShowCreateTemplateModal(false)}
          onSave={async (template) => {
            try {
              await saveTemplate(template);
              setShowCreateTemplateModal(false);
            } catch (error) {
              // Error is handled in modal
            }
          }}
        />
      )}

      {showEditTemplateModal && selectedTemplate && (
        <EditTemplateModal
          template={selectedTemplate}
          onClose={() => {
            setShowEditTemplateModal(false);
            setSelectedTemplate(null);
          }}
          onSave={async (template) => {
            try {
              await saveTemplate({ ...template, id: selectedTemplate.id });
              setShowEditTemplateModal(false);
              setSelectedTemplate(null);
            } catch (error) {
              // Error is handled in modal
            }
          }}
        />
      )}

      {showCreatePersonaModal && (
        <CreatePersonaModal
          templates={templates}
          onClose={() => setShowCreatePersonaModal(false)}
          onSave={async (persona) => {
            try {
              await savePersona(persona);
              setShowCreatePersonaModal(false);
            } catch (error) {
              // Error is handled in modal
            }
          }}
        />
      )}

      {showEditPersonaModal && selectedPersona && (
        <EditPersonaModal
          persona={selectedPersona}
          templates={templates}
          onClose={() => {
            setShowEditPersonaModal(false);
            setSelectedPersona(null);
          }}
          onSave={async (persona) => {
            try {
              await savePersona({ ...persona, id: selectedPersona.id });
              setShowEditPersonaModal(false);
              setSelectedPersona(null);
            } catch (error) {
              // Error is handled in modal
            }
          }}
        />
      )}
    </div>
  );
}

// Create Template Modal
function CreateTemplateModal({
  onClose,
  onSave,
}: {
  onClose: () => void;
  onSave: (template: any) => Promise<void>;
}) {
  const [formData, setFormData] = useState({
    name: '',
    framework: '',
    customTemplate: false,
    fields: {} as Record<string, string>,
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const frameworks = {
    'R-T-F': {
      name: 'R-T-F Framework',
      fields: {
        R: 'Act as a [ROLE]',
        T: 'Create a [TASK]',
        F: 'Show as [FORMAT]',
      },
    },
    'T-A-G': {
      name: 'T-A-G Framework',
      fields: {
        T: 'Define the [TASK]',
        A: 'State the [ACTION]',
        G: 'Clarify the [GOAL]',
      },
    },
    'B-A-B': {
      name: 'B-A-B Framework',
      fields: {
        B1: 'Explain the problem [BEFORE]',
        A: 'State the outcome [AFTER]',
        B2: 'Ask ChatGPT to be the [BRIDGE] between the two',
      },
    },
    'C-A-R-E': {
      name: 'C-A-R-E Framework',
      fields: {
        C: 'Give the [CONTEXT]',
        A: 'Describe the [ACTION]',
        R: 'Clarify the [RESULT]',
        E: 'Give the [EXAMPLE]',
      },
    },
    'R-I-S-E': {
      name: 'R-I-S-E Framework',
      fields: {
        R: 'Specify the [ROLE]',
        I: 'Describe the [INPUT]',
        S: 'Ask for [STEPS]',
        E: 'Describe the [EXPECTATION]',
      },
    },
  };

  const handleFrameworkChange = (framework: string) => {
    if (framework === 'custom') {
      setFormData({
        ...formData,
        framework: '',
        customTemplate: true,
        fields: {},
      });
    } else {
      setFormData({
        ...formData,
        framework: framework,
        customTemplate: false,
        fields: (frameworks as any)[framework].fields,
      });
    }
  };

  const handleFieldChange = (key: string, value: string) => {
    setFormData({
      ...formData,
      fields: {
        ...formData.fields,
        [key]: value,
      },
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!formData.name.trim()) {
      setError('Template name is required');
      return;
    }

    if (formData.customTemplate) {
      if (!formData.fields.custom || !formData.fields.custom.trim()) {
        setError('Custom template content is required');
        return;
      }
    } else {
      const frameworkFields = (frameworks as any)[formData.framework]?.fields || {};
      const allFilled = Object.keys(frameworkFields).every((key) => {
        const value = formData.fields[key];
        return value && value.trim() !== '';
      });
      if (!allFilled) {
        setError('Please fill in all framework fields');
        return;
      }
    }

    const template = {
      name: formData.name,
      framework: formData.customTemplate ? 'custom' : formData.framework,
      is_custom: formData.customTemplate,
      content: formData.fields,
    };

    setLoading(true);
    try {
      await onSave(template);
      onClose();
    } catch (error: any) {
      setError(error.message || 'Failed to create template');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Create Template</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Template Name *
            </label>
            <Input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              required
              className="w-full"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Select Framework
            </label>
            <select
              value={formData.customTemplate ? 'custom' : formData.framework}
              onChange={(e) => handleFrameworkChange(e.target.value)}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            >
              <option value="">-- Select Framework --</option>
              {Object.keys(frameworks).map((key) => (
                <option key={key} value={key}>
                  {(frameworks as any)[key].name}
                </option>
              ))}
              <option value="custom">Create Custom Template</option>
            </select>
          </div>

          {formData.framework && !formData.customTemplate && (
            <div className="space-y-4 border-t border-gray-200 dark:border-gray-700 pt-5 mt-5">
              <h3 className="font-semibold text-gray-900 dark:text-gray-100 text-base">
                {(frameworks as any)[formData.framework].name}
              </h3>
              {Object.entries((frameworks as any)[formData.framework].fields).map(([key, label]) => (
                <div key={key}>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    {String(label)}
                  </label>
                  <TextareaAutosize
                    value={formData.fields[key] || ''}
                    onChange={(e) => handleFieldChange(key, e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                      }
                    }}
                    placeholder={`Enter ${String(label).toLowerCase()}`}
                    required
                    minRows={3}
                    maxRows={8}
                    aria-label={String(label)}
                    className="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 resize-none"
                  />
                </div>
              ))}
            </div>
          )}

          {formData.customTemplate && (
            <div className="space-y-4 border-t border-gray-200 dark:border-gray-700 pt-5 mt-5">
              <h3 className="font-semibold text-gray-900 dark:text-gray-100 text-base">Custom Template</h3>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Template Content *
                </label>
                <TextareaAutosize
                  value={formData.fields.custom || ''}
                  onChange={(e) => handleFieldChange('custom', e.target.value)}
                  minRows={8}
                  maxRows={15}
                  placeholder="Enter your custom template here..."
                  required
                  aria-label="Custom template content"
                  className="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 resize-none"
                />
              </div>
            </div>
          )}

          <div className="flex gap-3 pt-4 border-t border-gray-200 dark:border-gray-700 mt-6">
            <Button type="button" onClick={onClose} variant="outline" className="flex-1">
              Cancel
            </Button>
            <Button type="submit" disabled={loading} className="flex-1">
              {loading ? 'Creating...' : 'Save Template'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// Edit Template Modal
function EditTemplateModal({
  template,
  onClose,
  onSave,
}: {
  template: any;
  onClose: () => void;
  onSave: (template: any) => Promise<void>;
}) {
  const [formData, setFormData] = useState({
    name: template.name || '',
    framework: template.framework || '',
    customTemplate: template.is_custom || false,
    fields: template.content || {} as Record<string, string>,
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const frameworks = {
    'R-T-F': {
      name: 'R-T-F Framework',
      fields: {
        R: 'Act as a [ROLE]',
        T: 'Create a [TASK]',
        F: 'Show as [FORMAT]',
      },
    },
    'T-A-G': {
      name: 'T-A-G Framework',
      fields: {
        T: 'Define the [TASK]',
        A: 'State the [ACTION]',
        G: 'Clarify the [GOAL]',
      },
    },
    'B-A-B': {
      name: 'B-A-B Framework',
      fields: {
        B1: 'Explain the problem [BEFORE]',
        A: 'State the outcome [AFTER]',
        B2: 'Ask ChatGPT to be the [BRIDGE] between the two',
      },
    },
    'C-A-R-E': {
      name: 'C-A-R-E Framework',
      fields: {
        C: 'Give the [CONTEXT]',
        A: 'Describe the [ACTION]',
        R: 'Clarify the [RESULT]',
        E: 'Give the [EXAMPLE]',
      },
    },
    'R-I-S-E': {
      name: 'R-I-S-E Framework',
      fields: {
        R: 'Specify the [ROLE]',
        I: 'Describe the [INPUT]',
        S: 'Ask for [STEPS]',
        E: 'Describe the [EXPECTATION]',
      },
    },
  };

  const handleFrameworkChange = (framework: string) => {
    if (framework === 'custom') {
      setFormData({
        ...formData,
        framework: '',
        customTemplate: true,
        fields: formData.fields.custom ? { custom: formData.fields.custom } : {},
      });
    } else {
      setFormData({
        ...formData,
        framework: framework,
        customTemplate: false,
        fields: (frameworks as any)[framework].fields,
      });
    }
  };

  const handleFieldChange = (key: string, value: string) => {
    setFormData({
      ...formData,
      fields: {
        ...formData.fields,
        [key]: value,
      },
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!formData.name.trim()) {
      setError('Template name is required');
      return;
    }

    if (formData.customTemplate) {
      if (!formData.fields.custom || !formData.fields.custom.trim()) {
        setError('Custom template content is required');
        return;
      }
    } else {
      const frameworkFields = (frameworks as any)[formData.framework]?.fields || {};
      const allFilled = Object.keys(frameworkFields).every((key) => {
        const value = formData.fields[key];
        return value && value.trim() !== '';
      });
      if (!allFilled) {
        setError('Please fill in all framework fields');
        return;
      }
    }

    const updatedTemplate = {
      name: formData.name,
      framework: formData.customTemplate ? 'custom' : formData.framework,
      is_custom: formData.customTemplate,
      content: formData.fields,
    };

    setLoading(true);
    try {
      await onSave(updatedTemplate);
      onClose();
    } catch (error: any) {
      setError(error.message || 'Failed to update template');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Edit Template</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Template Name *
            </label>
            <Input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              required
              className="w-full"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Select Framework
            </label>
            <select
              value={formData.customTemplate ? 'custom' : formData.framework}
              onChange={(e) => handleFrameworkChange(e.target.value)}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            >
              <option value="">-- Select Framework --</option>
              {Object.keys(frameworks).map((key) => (
                <option key={key} value={key}>
                  {(frameworks as any)[key].name}
                </option>
              ))}
              <option value="custom">Create Custom Template</option>
            </select>
          </div>

          {formData.framework && !formData.customTemplate && (
            <div className="space-y-4 border-t border-gray-200 dark:border-gray-700 pt-5 mt-5">
              <h3 className="font-semibold text-gray-900 dark:text-gray-100 text-base">
                {(frameworks as any)[formData.framework].name}
              </h3>
              {Object.entries((frameworks as any)[formData.framework].fields).map(([key, label]) => (
                <div key={key}>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    {String(label)}
                  </label>
                  <TextareaAutosize
                    value={formData.fields[key] || ''}
                    onChange={(e) => handleFieldChange(key, e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                      }
                    }}
                    placeholder={`Enter ${String(label).toLowerCase()}`}
                    required
                    minRows={3}
                    maxRows={8}
                    aria-label={String(label)}
                    className="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 resize-none"
                  />
                </div>
              ))}
            </div>
          )}

          {formData.customTemplate && (
            <div className="space-y-4 border-t border-gray-200 dark:border-gray-700 pt-5 mt-5">
              <h3 className="font-semibold text-gray-900 dark:text-gray-100 text-base">Custom Template</h3>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Template Content *
                </label>
                <TextareaAutosize
                  value={formData.fields.custom || ''}
                  onChange={(e) => handleFieldChange('custom', e.target.value)}
                  minRows={8}
                  maxRows={15}
                  placeholder="Enter your custom template here..."
                  required
                  aria-label="Custom template content"
                  className="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 resize-none"
                />
              </div>
            </div>
          )}

          <div className="flex gap-3 pt-4 border-t border-gray-200 dark:border-gray-700 mt-6">
            <Button type="button" onClick={onClose} variant="outline" className="flex-1">
              Cancel
            </Button>
            <Button type="submit" disabled={loading} className="flex-1">
              {loading ? 'Updating...' : 'Update Template'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// Predefined Personas
const PREDEFINED_PERSONAS = [
  {
    name: 'Financial Advisor',
    description: 'Provide investment, market, and personal finance guidance.',
    template: `Act as a financial advisor specializing in {{focus_area}}.

User financial context: {{user_context}}

Provide:

1. Market overview  

2. Recommended actions  

3. Risks involved  

4. Clear reasoning behind each step  

Keep the explanation simple and actionable.`,
    variables: ['focus_area', 'user_context'],
  },
  {
    name: 'Business Consultant',
    description: 'Offer strategic, operational, or profitability advice for businesses.',
    template: `Act as a business consultant focusing on {{business_domain}}.

Problem to analyze: {{problem_statement}}

Provide:

- Root cause analysis  

- Strategic recommendations  

- Impact on revenue/operations  

- Steps to execute`,
    variables: ['business_domain', 'problem_statement'],
  },
  {
    name: 'Research Assistant',
    description: 'Gather structured information and present concise findings.',
    template: `Act as a research assistant.

Research topic: {{topic}}

Provide:

- Short summary  

- Key findings  

- Comparisons (if applicable)  

- Useful insights`,
    variables: ['topic'],
  },
  {
    name: 'Report Generator',
    description: 'Convert raw text into a structured professional report.',
    template: `Generate a structured report from the following input:

{{input_data}}

Format:

- Executive Summary  

- Key Insights  

- Supporting Details  

- Recommendations`,
    variables: ['input_data'],
  },
  {
    name: 'Risk Analyst',
    description: 'Identify threats, vulnerabilities, and mitigation strategies.',
    template: `Act as a risk analyst.

Context: {{context}}

Provide:

- Identified risks  

- Probability & impact  

- Mitigation strategies  

- Priority level`,
    variables: ['context'],
  },
  {
    name: 'Marketing Strategist',
    description: 'Develop campaign ideas, positioning, and messaging.',
    template: `Act as a marketing strategist.

Goal: {{marketing_goal}}

Target audience: {{target_audience}}

Provide:

- Positioning  

- Messaging  

- Campaign ideas  

- CTA suggestions`,
    variables: ['marketing_goal', 'target_audience'],
  },
  {
    name: 'Technical Explainer',
    description: 'Explain complex concepts in simple, intuitive ways.',
    template: `Act as a technical explainer.

Topic: {{topic}}

Explain:

- What it is  

- How it works  

- Why it matters  

- Simple example`,
    variables: ['topic'],
  },
  {
    name: 'Summarizer',
    description: 'Produce concise, clear summaries.',
    template: `Summarize the following text:

{{text}}

Keep it:

- Concise  

- Clear  

- Covering only the essential points`,
    variables: ['text'],
  },
];

// Create Persona Modal
function CreatePersonaModal({
  templates,
  onClose,
  onSave,
}: {
  templates: any[];
  onClose: () => void;
  onSave: (persona: any) => Promise<void>;
}) {
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    usePredefined: false,
    selectedPredefinedId: '',
    useTemplate: false,
    selectedTemplateId: '',
    customTemplate: '',
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  
  // Handle predefined persona selection
  const handlePredefinedSelect = (predefinedId: string) => {
    const predefined = PREDEFINED_PERSONAS.find((_, idx) => idx.toString() === predefinedId);
    if (predefined) {
      setFormData({
        ...formData,
        name: predefined.name,
        description: predefined.template, // Fill description with persona template (with variables like {{focus_area}})
        selectedPredefinedId: predefinedId,
        // Template selection remains separate - user can select template or write custom
      });
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.name.trim()) {
      setError('Persona name is required');
      return;
    }
    // Description is optional when using predefined persona (auto-filled)
    if (!formData.selectedPredefinedId && !formData.description.trim()) {
      setError('Description is required');
      return;
    }
    if (formData.useTemplate) {
      if (!formData.selectedTemplateId) {
        setError('Please select a template');
        return;
      }
    } else {
      if (!formData.customTemplate.trim()) {
        setError('Custom template content is required');
        return;
      }
    }
    const persona = {
      name: formData.name,
      description: formData.description || null,
      template_id: formData.useTemplate && formData.selectedTemplateId ? formData.selectedTemplateId : null,
      is_custom_template: !formData.useTemplate,
      content: formData.useTemplate ? {} : { custom: formData.customTemplate },
    };

    setLoading(true);
    try {
      await onSave(persona);
      onClose();
    } catch (error: any) {
      setError(error.message || 'Failed to create persona');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Create Persona</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Name *</label>
            <Input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              required
              className="w-full"
            />
          </div>

          {/* Predefined Personas - Above description */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Select Predefined Persona (optional)
            </label>
            <select
              value={formData.selectedPredefinedId}
              onChange={(e) => handlePredefinedSelect(e.target.value)}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            >
              <option value="">-- Select Predefined Persona (Optional) --</option>
              {PREDEFINED_PERSONAS.map((persona, idx) => (
                <option key={idx} value={idx.toString()}>
                  {persona.name}
                </option>
              ))}
            </select>
            {formData.selectedPredefinedId && (
              <p className="mt-1 text-xs text-blue-600 dark:text-blue-400">
                Persona template will be auto-filled below. Just edit the variables like {`{{variable_name}}`} with your values.
              </p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Description {formData.selectedPredefinedId ? '' : '*'}
            </label>
            <TextareaAutosize
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              minRows={5}
              maxRows={10}
              required={!formData.selectedPredefinedId}
              aria-label="Persona description"
              className="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 resize-none"
              placeholder={formData.selectedPredefinedId ? "Edit variables like {{focus_area}} with your values" : "Enter persona description..."}
            />
            {formData.selectedPredefinedId && (
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Variables to edit: {PREDEFINED_PERSONAS[parseInt(formData.selectedPredefinedId)]?.variables.map(v => `{{${v}}}`).join(', ')}
              </p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Template</label>
            <div className="space-y-3">

              {/* Option 1: Select from created templates */}
              <label className="flex items-center">
                <input
                  type="radio"
                  checked={formData.useTemplate}
                  onChange={() => setFormData({ 
                    ...formData, 
                    useTemplate: true,
                    customTemplate: '',
                  })}
                  className="mr-2"
                />
                <span className="text-gray-700 dark:text-gray-300">Select from created templates</span>
              </label>
              {formData.useTemplate && (
                <select
                  value={formData.selectedTemplateId}
                  onChange={(e) => setFormData({ ...formData, selectedTemplateId: e.target.value })}
                  className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 ml-6"
                >
                  <option value="">-- Select Template --</option>
                  {templates.map((template) => (
                    <option key={template.id} value={template.id}>
                      {template.name} ({template.framework})
                    </option>
                  ))}
                </select>
              )}

              {/* Option 2: Write your own template */}
              <label className="flex items-center">
                <input
                  type="radio"
                  checked={!formData.useTemplate}
                  onChange={() => setFormData({ 
                    ...formData, 
                    useTemplate: false,
                    selectedTemplateId: '',
                  })}
                  className="mr-2"
                />
                <span className="text-gray-700 dark:text-gray-300">Write your own template</span>
              </label>
              {!formData.useTemplate && (
                <TextareaAutosize
                  value={formData.customTemplate}
                  onChange={(e) => setFormData({ ...formData, customTemplate: e.target.value })}
                  minRows={6}
                  maxRows={12}
                  aria-label="Custom persona template"
                  className="ml-6 w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 resize-none"
                  placeholder="Enter your custom template here..."
                />
              )}
            </div>
          </div>

          <div className="flex gap-3 pt-4 border-t border-gray-200 dark:border-gray-700 mt-6">
            <Button type="button" onClick={onClose} variant="outline" className="flex-1">
              Cancel
            </Button>
            <Button type="submit" disabled={loading} className="flex-1">
              {loading ? 'Creating...' : 'Save Persona'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// Edit Persona Modal
function EditPersonaModal({
  persona,
  templates,
  onClose,
  onSave,
}: {
  persona: any;
  templates: any[];
  onClose: () => void;
  onSave: (persona: any) => Promise<void>;
}) {
  // Initialize formData based on persona's current state
  const getInitialFormData = () => {
    // Check is_custom_template first - this is the definitive flag
    // If is_custom_template is true, use custom template mode (second option)
    // If is_custom_template is false and template_id exists, use template selection mode (first option)
    const useTemplateMode = !persona.is_custom_template && !!persona.template_id;
    
    // Extract custom template content
    let customTemplateContent = '';
    if (persona.is_custom_template && persona.content) {
      if (persona.content.custom) {
        customTemplateContent = typeof persona.content.custom === 'string' 
          ? persona.content.custom 
          : JSON.stringify(persona.content.custom);
      } else if (typeof persona.content === 'string') {
        customTemplateContent = persona.content;
      } else if (typeof persona.content === 'object') {
        // Try to extract custom content from object
        const custom = (persona.content as any).custom;
        if (custom) {
          customTemplateContent = typeof custom === 'string' ? custom : JSON.stringify(custom);
        }
      }
    }
    
    return {
    name: persona.name || '',
    description: persona.description || '',
    usePredefined: false,
    selectedPredefinedId: '',
      useTemplate: useTemplateMode, // false if is_custom_template is true, true if template_id exists
      selectedTemplateId: useTemplateMode ? (persona.template_id || '') : '',
      customTemplate: customTemplateContent,
    };
  };
  
  const [formData, setFormData] = useState(getInitialFormData());
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  
  // Handle predefined persona selection
  const handlePredefinedSelect = (predefinedId: string) => {
    const predefined = PREDEFINED_PERSONAS.find((_, idx) => idx.toString() === predefinedId);
    if (predefined) {
      setFormData({
        ...formData,
        name: predefined.name,
        description: predefined.template, // Fill description with persona template (with variables like {{focus_area}})
        selectedPredefinedId: predefinedId,
        // Template selection remains separate - user can select template or write custom
      });
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.name.trim()) {
      setError('Persona name is required');
      return;
    }
    // Description is optional when using predefined persona (auto-filled)
    if (!formData.selectedPredefinedId && !formData.description.trim()) {
      setError('Description is required');
      return;
    }
    if (formData.useTemplate) {
      if (!formData.selectedTemplateId) {
        setError('Please select a template');
        return;
      }
    } else {
      if (!formData.customTemplate.trim()) {
        setError('Custom template content is required');
        return;
      }
    }
    const updatedPersona = {
      name: formData.name,
      description: formData.description || null,
      template_id: formData.useTemplate && formData.selectedTemplateId ? formData.selectedTemplateId : null,
      is_custom_template: !formData.useTemplate,
      content: formData.useTemplate ? {} : { custom: formData.customTemplate.trim() },
    };
    
    console.log('Updating persona with:', updatedPersona);

    setLoading(true);
    try {
      await onSave(updatedPersona);
      onClose();
    } catch (error: any) {
      setError(error.message || 'Failed to update persona');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Edit Persona</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Name *</label>
            <Input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              required
              className="w-full"
            />
          </div>

          {/* Predefined Personas - Above description */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Select Predefined Persona (optional)
            </label>
            <select
              value={formData.selectedPredefinedId}
              onChange={(e) => handlePredefinedSelect(e.target.value)}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            >
              <option value="">-- Select Predefined Persona (Optional) --</option>
              {PREDEFINED_PERSONAS.map((persona, idx) => (
                <option key={idx} value={idx.toString()}>
                  {persona.name}
                </option>
              ))}
            </select>
            {formData.selectedPredefinedId && (
              <p className="mt-1 text-xs text-blue-600 dark:text-blue-400">
                Persona template will be auto-filled below. Just edit the variables like {`{{variable_name}}`} with your values.
              </p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Description {formData.selectedPredefinedId ? '' : '*'}
            </label>
            <TextareaAutosize
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              minRows={5}
              maxRows={10}
              required={!formData.selectedPredefinedId}
              aria-label="Persona description"
              className="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 resize-none"
              placeholder={formData.selectedPredefinedId ? "Edit variables like {{focus_area}} with your values" : "Enter persona description..."}
            />
            {formData.selectedPredefinedId && (
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Variables to edit: {PREDEFINED_PERSONAS[parseInt(formData.selectedPredefinedId)]?.variables.map(v => `{{${v}}}`).join(', ')}
              </p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Template</label>
            <div className="space-y-3">

              {/* Option 1: Select from created templates */}
              <label className="flex items-center">
                <input
                  type="radio"
                  checked={formData.useTemplate}
                  onChange={() => setFormData({ 
                    ...formData, 
                    useTemplate: true,
                    customTemplate: '',
                  })}
                  className="mr-2"
                />
                <span className="text-gray-700 dark:text-gray-300">Select from created templates</span>
              </label>
              {formData.useTemplate && (
                <select
                  value={formData.selectedTemplateId}
                  onChange={(e) => setFormData({ ...formData, selectedTemplateId: e.target.value })}
                  className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 ml-6"
                >
                  <option value="">-- Select Template --</option>
                  {templates.map((template) => (
                    <option key={template.id} value={template.id}>
                      {template.name} ({template.framework})
                    </option>
                  ))}
                </select>
              )}

              {/* Option 2: Write your own template */}
              <label className="flex items-center">
                <input
                  type="radio"
                  checked={!formData.useTemplate}
                  onChange={() => setFormData({ 
                    ...formData, 
                    useTemplate: false,
                    selectedTemplateId: '',
                  })}
                  className="mr-2"
                />
                <span className="text-gray-700 dark:text-gray-300">Write your own template</span>
              </label>
              {!formData.useTemplate && (
                <TextareaAutosize
                  value={formData.customTemplate}
                  onChange={(e) => setFormData({ ...formData, customTemplate: e.target.value })}
                  minRows={6}
                  maxRows={12}
                  aria-label="Custom persona template"
                  className="ml-6 w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 resize-none"
                  placeholder="Enter your custom template here..."
                />
              )}
            </div>
          </div>

          <div className="flex gap-3 pt-4 border-t border-gray-200 dark:border-gray-700 mt-6">
            <Button type="button" onClick={onClose} variant="outline" className="flex-1">
              Cancel
            </Button>
            <Button type="submit" disabled={loading} className="flex-1">
              {loading ? 'Updating...' : 'Update Persona'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
