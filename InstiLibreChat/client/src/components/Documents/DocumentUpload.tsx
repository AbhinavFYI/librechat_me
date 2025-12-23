import React, { useState, useCallback } from 'react';
import { Upload, FileText, AlertCircle, CheckCircle } from 'lucide-react';
import { useToastContext } from '@librechat/client';
import { useLocalize } from '~/hooks';
import { uploadDocument } from '~/data-provider/document-service';
import { cn } from '~/utils';

interface DocumentUploadProps {
  onUploadSuccess?: (filename: string) => void;
  className?: string;
}

export default function DocumentUpload({ onUploadSuccess, className }: DocumentUploadProps) {
  const localize = useLocalize();
  const { showToast } = useToastContext();
  const [isUploading, setIsUploading] = useState(false);
  const [dragActive, setDragActive] = useState(false);

  const handleUpload = useCallback(
    async (file: File) => {
      if (!file) return;

      // Validate file type (accept common document formats)
      const allowedTypes = [
        // Word documents
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document', // .docx
        'application/vnd.openxmlformats-officedocument.wordprocessingml.template', // .dotx
        'application/vnd.ms-word.document.macroEnabled.12', // .docm
        'application/vnd.ms-word.template.macroEnabled.12', // .dotm
        // PowerPoint
        'application/vnd.openxmlformats-officedocument.presentationml.presentation', // .pptx
        // PDF
        'application/pdf',
        // Markdown
        'text/markdown',
        'text/x-markdown',
        // HTML
        'text/html',
        'application/xhtml+xml',
        // Images
        'image/jpeg',
        'image/jpg',
        'image/png',
        'image/tiff',
        'image/bmp',
        'image/webp',
        // CSV
        'text/csv',
        'application/csv',
        // Excel
        'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', // .xlsx
        'application/vnd.ms-excel.sheet.macroEnabled.12', // .xlsm
        // Text
        'text/plain',
        // JSON
        'application/json',
        'text/json',
      ];

      if (!allowedTypes.includes(file.type)) {
        showToast({
          message: 'Please select a valid document file (DOCX, DOTX, DOCM, DOTM, PPTX, PDF, MD, HTML, JPG, PNG, TIFF, BMP, WEBP, CSV, XLSX, XLSM, TXT, JSON)',
          status: 'error',
        });
        return;
      }

      // Validate file size (max 50MB)
      const maxSize = 50 * 1024 * 1024; // 50MB
      if (file.size > maxSize) {
        showToast({
          message: 'File size must be less than 50MB',
          status: 'error',
        });
        return;
      }

      setIsUploading(true);
      try {
        const response = await uploadDocument(file);

        if (response.code === 200 || response.code === 202 || response.s === 'ok') {
          showToast({
            message: `Document "${file.name}" uploaded successfully and is being processed`,
            status: 'success',
          });
          onUploadSuccess?.(file.name);
        } else {
          throw new Error(response.message || 'Upload failed');
        }
      } catch (error) {
        console.error('Upload error:', error);
        showToast({
          message: `Failed to upload document: ${error instanceof Error ? error.message : 'Unknown error'}`,
          status: 'error',
        });
      } finally {
        setIsUploading(false);
      }
    },
    [showToast, onUploadSuccess],
  );

  const handleFileChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      if (file) {
        handleUpload(file);
      }
      // Reset input
      event.target.value = '';
    },
    [handleUpload],
  );

  const handleDrag = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  }, []);

  const handleDragIn = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.dataTransfer.items && e.dataTransfer.items.length > 0) {
      setDragActive(true);
    }
  }, []);

  const handleDragOut = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setDragActive(false);

      if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
        const file = e.dataTransfer.files[0];
        handleUpload(file);
      }
    },
    [handleUpload],
  );

  return (
    <div className={cn('w-full', className)}>
      <div
        className={cn(
          'relative border-2 border-dashed rounded-lg p-6 text-center transition-colors',
          dragActive
            ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
            : 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500',
          isUploading && 'pointer-events-none opacity-50',
        )}
        onDragEnter={handleDragIn}
        onDragLeave={handleDragOut}
        onDragOver={handleDrag}
        onDrop={handleDrop}
      >
        <input
          type="file"
          id="document-upload"
          className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
          onChange={handleFileChange}
          disabled={isUploading}
          accept=".docx,.dotx,.docm,.dotm,.pptx,.pdf,.md,.html,.htm,.xhtml,.jpg,.jpeg,.png,.tiff,.bmp,.webp,.csv,.xlsx,.xlsm,.txt,.json"
        />

        <div className="flex flex-col items-center justify-center space-y-3">
          {isUploading ? (
            <>
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
              <p className="text-sm text-gray-600 dark:text-gray-400">Uploading document...</p>
            </>
          ) : (
            <>
              <div className="flex items-center justify-center w-12 h-12 bg-gray-100 dark:bg-gray-800 rounded-full">
                <Upload className="w-6 h-6 text-gray-600 dark:text-gray-400" />
              </div>
              <div>
                <p className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  Upload Document
                </p>
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  Drag and drop a file here, or click to select
                </p>
              </div>
              <div className="text-xs text-gray-400 dark:text-gray-500">
                Supports PDF,DOCX, XLSX, TXT, CSV (Max 50MB)
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

