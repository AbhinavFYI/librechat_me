import { useState, useRef, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@librechat/client';
import { Button } from '@librechat/client';
import { useToastContext } from '@librechat/client';
import { uploadDocument } from '~/data-provider/document-service';
import { saasApi } from '~/services/saasApi';
import { Upload, File as FileIcon } from 'lucide-react';

interface UploadFileModalProps {
  folderId?: string;
  orgId?: string | null;
  folders?: any[]; // Folder tree for selection
  onClose: () => void;
  onSuccess: () => void;
}

interface FlatFolder {
  id: string;
  name: string;
  path: string;
  level: number;
}

export default function UploadFileModal({ folderId, orgId, folders = [], onClose, onSuccess }: UploadFileModalProps) {
  const { showToast } = useToastContext();
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [selectedFolderId, setSelectedFolderId] = useState<string>(folderId || '');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Flatten folder tree for dropdown
  const flattenFolders = (folderNodes: any[], level = 0): FlatFolder[] => {
    let result: FlatFolder[] = [];
    folderNodes.forEach((folder) => {
      result.push({
        id: folder.id,
        name: folder.name,
        path: folder.path || folder.name,
        level,
      });
      if (folder.children && folder.children.length > 0) {
        result = result.concat(flattenFolders(folder.children, level + 1));
      }
    });
    return result;
  };

  const flatFolders = flattenFolders(folders);

  useEffect(() => {
    if (folderId) {
      setSelectedFolderId(folderId);
    }
  }, [folderId]);

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setSelectedFile(file);
      setError(null);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedFile) {
      setError('Please select a file');
      return;
    }

    // Validate file type (accept common document formats)
    const allowedTypes = [
      'application/pdf',
      'application/msword',
      'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
      'application/vnd.ms-excel',
      'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      'text/plain',
      'text/csv',
    ];

    if (!allowedTypes.includes(selectedFile.type)) {
      setError('Please select a valid document file (PDF, DOC, DOCX, XLS, XLSX, TXT, CSV)');
      showToast({
        message: 'Please select a valid document file (PDF, DOC, DOCX, XLS, XLSX, TXT, CSV)',
        status: 'error',
      });
      return;
    }

    // Validate file size (max 50MB)
    const maxSize = 50 * 1024 * 1024; // 50MB
    if (selectedFile.size > maxSize) {
      setError('File size must be less than 50MB');
      showToast({
        message: 'File size must be less than 50MB',
        status: 'error',
      });
      return;
    }

    setLoading(true);
    setError(null);

    try {
      // Step 1: Upload document to document service (same as chat upload)
      const response = await uploadDocument(selectedFile);

      if (response.code === 202 || response.s === 'ok') {
        // Step 2: Store folder association in our database if folder is selected
        if (selectedFolderId) {
          try {
            // Get document data from response - handle different response structures
            const responseData = response.data || response;
            const documentId = responseData.document_id || responseData.id || responseData.documentId;
            const filePath = responseData.file_path || responseData.filePath || responseData.path || '';
            
            // Create file record in our database with folder association
            await saasApi.createFile({
              name: selectedFile.name,
              file_path: filePath,
              document_id: documentId,
              folder_id: selectedFolderId,
              org_id: orgId || undefined,
              size_bytes: selectedFile.size,
              mime_type: selectedFile.type,
            });
            
            showToast({
              message: `Document "${selectedFile.name}" uploaded successfully and associated with folder`,
              status: 'success',
            });
          } catch (dbError: any) {
            // Document uploaded but folder association failed
            console.warn('Failed to store folder association:', dbError);
            showToast({
              message: `Document uploaded but failed to associate with folder: ${dbError.message || 'Unknown error'}`,
              status: 'warning',
            });
          }
        } else {
          // No folder selected, just upload the document
          showToast({
            message: `Document "${selectedFile.name}" uploaded successfully and is being processed`,
            status: 'success',
          });
        }
        
        onSuccess();
        onClose();
      } else {
        throw new Error(response.message || 'Upload failed');
      }
    } catch (err: any) {
      const errorMessage = err.message || 'Failed to upload file';
      setError(errorMessage);
      showToast({
        message: `Failed to upload document: ${errorMessage}`,
        status: 'error',
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-md p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Upload Document</DialogTitle>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            Select a document file to upload and store in the database
          </p>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Select File
            </label>
            <div className="border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-6 text-center hover:border-blue-500 transition-colors">
              <input
                ref={fileInputRef}
                type="file"
                onChange={handleFileSelect}
                className="hidden"
                id="file-upload"
                accept=".pdf,.doc,.docx,.xls,.xlsx,.txt,.csv"
              />
              <label
                htmlFor="file-upload"
                className="cursor-pointer flex flex-col items-center gap-2"
              >
                {selectedFile ? (
                  <>
                    <FileIcon className="h-12 w-12 text-blue-500" />
                    <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                      {selectedFile.name}
                    </span>
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      {(selectedFile.size / 1024).toFixed(1)} KB
                    </span>
                  </>
                ) : (
                  <>
                    <Upload className="h-12 w-12 text-gray-400" />
                    <span className="text-sm text-gray-600 dark:text-gray-400">
                      Click to select a file
                    </span>
                  </>
                )}
              </label>
            </div>
            {selectedFile && (
              <button
                type="button"
                onClick={() => {
                  setSelectedFile(null);
                  if (fileInputRef.current) {
                    fileInputRef.current.value = '';
                  }
                }}
                className="mt-2 text-sm text-red-600 dark:text-red-400 hover:underline"
              >
                Remove file
              </button>
            )}
          </div>

          <div className="text-xs text-gray-500 dark:text-gray-400 mt-2">
            Supports PDF, DOC, DOCX, XLS, XLSX, TXT, CSV (Max 50MB)
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Select Folder (Optional)
            </label>
            <select
              value={selectedFolderId}
              onChange={(e) => setSelectedFolderId(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            >
              <option value="">Root (No folder)</option>
              {flatFolders.map((folder) => (
                <option key={folder.id} value={folder.id}>
                  {'  '.repeat(folder.level)}
                  {folder.level > 0 ? '└─ ' : ''}
                  {folder.name}
                </option>
              ))}
            </select>
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
              Select a folder to associate this document with in the database.
            </p>
          </div>

          <div className="flex justify-end gap-3 pt-4">
            <Button type="button" onClick={onClose} variant="outline">
              Cancel
            </Button>
            <Button type="submit" disabled={loading || !selectedFile}>
              {loading ? 'Uploading...' : 'Upload File'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

