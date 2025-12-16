import React, { useState, useEffect, useCallback } from 'react';
import { RefreshCw, X, FileText, User, Calendar, Check } from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, Button } from '@librechat/client';
import { useLocalize, useMCPServerManager } from '~/hooks';
import { fetchDocuments, type DocumentListItem } from '~/data-provider/document-service';
import { cn } from '~/utils';

interface DocumentSelectorProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (selectedDocuments: DocumentListItem[]) => void;
  conversationId?: string | null;
}

export default function DocumentSelector({
  isOpen,
  onOpenChange,
  onConfirm,
  conversationId,
}: DocumentSelectorProps) {
  const localize = useLocalize();
  const mcpServerManager = useMCPServerManager({ conversationId: conversationId || null });
  const [documents, setDocuments] = useState<DocumentListItem[]>([]);
  const [selectedDocuments, setSelectedDocuments] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadDocuments = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetchDocuments();
      console.log('DocumentSelector received response:', response);
      
      // Handle different response structures
      let documentsList: DocumentListItem[] = [];
      
      if (Array.isArray(response)) {
        documentsList = response;
      } else if (response.data) {
        // Check if data.documents exists (nested structure: {data: {documents: [...]}})
        const dataAny = response.data as any;
        if (dataAny.documents && Array.isArray(dataAny.documents)) {
          documentsList = dataAny.documents;
        } else if (Array.isArray(response.data)) {
          // Direct array in data: {data: [...]}
          documentsList = response.data;
        }
      } else if (typeof response === 'object' && response !== null) {
        // Try to find any array property (could be 'documents', 'data', etc.)
        const responseAny = response as any;
        if (responseAny.documents && Array.isArray(responseAny.documents)) {
          documentsList = responseAny.documents;
        } else {
          // Try to find any array property
          const arrayKeys = Object.keys(responseAny).filter(key => Array.isArray(responseAny[key]));
          if (arrayKeys.length > 0) {
            documentsList = responseAny[arrayKeys[0]];
          }
        }
      }
      
      if (documentsList.length > 0) {
        setDocuments(documentsList);
      } else {
        setError('No documents found');
      }
    } catch (err) {
      console.error('Error loading documents:', err);
      setError(err instanceof Error ? err.message : 'Failed to load documents');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isOpen) {
      loadDocuments();
    }
  }, [isOpen, loadDocuments]);

  const handleToggleSelection = useCallback((documentId: number) => {
    setSelectedDocuments((prev) => {
      const newSet = new Set(prev);
      const idStr = documentId.toString();
      if (newSet.has(idStr)) {
        newSet.delete(idStr);
      } else {
        newSet.add(idStr);
      }
      return newSet;
    });
  }, []);

  const handleConfirm = useCallback(() => {
    const selected = documents.filter((doc) => selectedDocuments.has(doc.document_id.toString()));
    
    // Store selected documents in localStorage for document_search MCP
    if (conversationId) {
      const documentsToStore = selected.map(doc => ({
        filename: doc.name,
        document_id: doc.document_id,
        file_path: doc.file_path,
        status: doc.status,
      }));
      
      console.log('[DocumentSelector] Storing documents in localStorage:', {
        conversationId,
        key: `persona_documents_${conversationId}`,
        documents: documentsToStore,
        document_ids: documentsToStore.map(d => d.document_id),
      });
      
      localStorage.setItem(
        `persona_documents_${conversationId}`,
        JSON.stringify({
          documents: documentsToStore,
          timestamp: Date.now(),
        }),
      );
      
      // Dispatch custom event to notify other components (like SelectedDocuments)
      window.dispatchEvent(new Event('documentsUpdated'));
      
      // Verify it was stored correctly
      const stored = localStorage.getItem(`persona_documents_${conversationId}`);
      if (stored) {
        const parsed = JSON.parse(stored);
        console.log('[DocumentSelector] ✅ Verified stored documents:', {
          stored_document_ids: parsed.documents?.map((d: any) => d.document_id),
        });
      }
      
      // Auto-select document-search MCP when documents are selected
      if (selected.length > 0) {
        const currentMCPValues = mcpServerManager.mcpValues || [];
        if (!currentMCPValues.includes('document_search')) {
          mcpServerManager.batchToggleServers([...currentMCPValues, 'document_search']);
        }
      } else {
        // Deselect document-search MCP if no documents are selected
        const currentMCPValues = mcpServerManager.mcpValues || [];
        if (currentMCPValues.includes('document_search')) {
          mcpServerManager.batchToggleServers(currentMCPValues.filter(s => s !== 'document_search'));
        }
      }
    }
    
    onConfirm(selected);
    onOpenChange(false);
  }, [documents, selectedDocuments, conversationId, onConfirm, onOpenChange, mcpServerManager]);

  const handleDismiss = useCallback(() => {
    setSelectedDocuments(new Set());
    onOpenChange(false);
  }, [onOpenChange]);

  const formatDate = (dateString: string) => {
    try {
      const date = new Date(dateString);
      return date.toLocaleDateString('en-GB', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
      });
    } catch {
      return dateString;
    }
  };

  const getFileExtension = (filename: string) => {
    const parts = filename.split('.');
    return parts.length > 1 ? parts[parts.length - 1].toUpperCase() : 'PDF';
  };

  const getStatusColor = (status: string) => {
    if (status === 'completed' || status === 'indexed') {
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400';
    } else if (status === 'failed' || status === 'error') {
      return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400';
    }
    return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300';
  };

  return (
    <Dialog open={isOpen} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl max-h-[90vh] flex flex-col p-0 overflow-hidden" showCloseButton={false}>
        <DialogHeader className="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
          <div className="flex items-center justify-between">
            <DialogTitle className="text-xl font-semibold">Select documents</DialogTitle>
            <div className="flex items-center gap-2">
              <button
                onClick={loadDocuments}
                disabled={loading}
                className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                aria-label="Refresh"
              >
                <RefreshCw className={cn('h-5 w-5', loading && 'animate-spin')} />
              </button>
              <button
                onClick={handleDismiss}
                className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                aria-label="Close"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
          </div>
        </DialogHeader>

        <div className="flex-1 overflow-hidden flex flex-col min-h-0">
          {error && (
            <div className="mx-6 mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-400 text-sm flex-shrink-0">
              {error}
            </div>
          )}

          <div className="flex-1 overflow-y-auto min-h-0">
            {loading ? (
              <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
              </div>
            ) : documents.length === 0 ? (
              <div className="flex items-center justify-center h-64 text-gray-500 dark:text-gray-400">
                No documents available
              </div>
            ) : (
              <table className="w-full table-fixed">
                <thead className="sticky top-0 bg-white dark:bg-gray-800 z-10 border-b border-gray-200 dark:border-gray-700 shadow-sm">
                  <tr>
                    <th className="text-left py-3 px-4 text-sm font-medium text-gray-700 dark:text-gray-300" style={{ width: '40%' }}>
                      Name
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-gray-700 dark:text-gray-300" style={{ width: '15%' }}>
                      Owner
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-gray-700 dark:text-gray-300" style={{ width: '12%' }}>
                      Format
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-gray-700 dark:text-gray-300" style={{ width: '18%' }}>
                      Upload date
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-gray-700 dark:text-gray-300" style={{ width: '15%' }}>
                      Status
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {documents.map((doc) => {
                    const isSelected = selectedDocuments.has(doc.document_id.toString());
                    return (
                      <tr
                        key={doc.document_id}
                        className={cn(
                          'border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50 cursor-pointer transition-colors',
                          isSelected && 'bg-blue-50 dark:bg-blue-900/20',
                        )}
                        onClick={() => handleToggleSelection(doc.document_id)}
                      >
                        <td className="py-3 px-4">
                          <div className="flex items-center gap-3">
                            <div
                              className={cn(
                                'w-5 h-5 border-2 rounded flex items-center justify-center flex-shrink-0',
                                isSelected
                                  ? 'border-blue-500 bg-blue-500'
                                  : 'border-gray-300 dark:border-gray-600',
                              )}
                            >
                              {isSelected && <Check className="h-3 w-3 text-white" />}
                            </div>
                            <div className="flex items-center gap-2 min-w-0">
                              <FileText className="h-4 w-4 text-gray-400 flex-shrink-0" />
                              <div className="min-w-0">
                                <div className="font-medium text-sm text-gray-900 dark:text-gray-100 truncate">
                                  {doc.name}
                                </div>
                                <div className="text-xs text-gray-500 dark:text-gray-400 truncate">
                                  {doc.file_path.split('/').pop() || doc.name}
                                </div>
                              </div>
                            </div>
                          </div>
                        </td>
                        <td className="py-3 px-4">
                          <div className="flex items-center gap-2">
                            <User className="h-4 w-4 text-gray-400" />
                            <span className="text-sm text-gray-700 dark:text-gray-300">
                              System
                            </span>
                          </div>
                        </td>
                        <td className="py-3 px-4">
                          <span className="text-sm text-gray-700 dark:text-gray-300">
                            {getFileExtension(doc.name)}
                          </span>
                        </td>
                        <td className="py-3 px-4">
                          <div className="flex items-center gap-2">
                            <Calendar className="h-4 w-4 text-gray-400" />
                            <span className="text-sm text-gray-700 dark:text-gray-300">
                              {formatDate(doc.uploaded_at)}
                            </span>
                          </div>
                        </td>
                        <td className="py-3 px-4">
                          <span
                            className={cn(
                              'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                              getStatusColor(doc.status),
                            )}
                          >
                            {doc.status}
                          </span>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>

        <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between bg-gray-50 dark:bg-gray-900/50 flex-shrink-0">
          <div className="text-sm text-gray-600 dark:text-gray-400">
            {selectedDocuments.size} document{selectedDocuments.size !== 1 ? 's' : ''} selected
          </div>
          <div className="flex items-center gap-3">
            <Button
              type="button"
              onClick={handleDismiss}
              variant="outline"
              className="px-4 py-2"
            >
              Dismiss
            </Button>
            <Button
              type="button"
              onClick={handleConfirm}
              disabled={selectedDocuments.size === 0}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Confirm selection
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

