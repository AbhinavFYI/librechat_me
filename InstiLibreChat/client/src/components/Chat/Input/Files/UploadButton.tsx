import React, { useState, useCallback } from 'react';
import { TooltipAnchor, Dialog, DialogContent, DialogHeader, DialogTitle } from '@librechat/client';
import { useLocalize } from '~/hooks';
import { cn } from '~/utils';
import DocumentUpload from '~/components/Documents/DocumentUpload';

interface UploadButtonProps {
  disabled?: boolean;
  conversationId?: string | null;
}

const UploadButton = ({ disabled }: UploadButtonProps) => {
  const localize = useLocalize();
  const isDisabled = disabled ?? false;
  const [showUploadDialog, setShowUploadDialog] = useState(false);

  const handleClick = useCallback(() => {
    if (isDisabled) return;
    setShowUploadDialog(true);
  }, [isDisabled]);

  const handleUploadSuccess = useCallback(
    (filename: string) => {
      console.log(`Document uploaded: ${filename}`);
      setShowUploadDialog(false);
    },
    [],
  );

  return (
    <>
      <TooltipAnchor
        render={
          <button
            disabled={isDisabled}
            onClick={handleClick}
            id="upload-document-button"
            aria-label="Upload Document"
            className={cn(
              'flex size-9 items-center justify-center rounded-full p-1 transition-colors hover:bg-surface-hover focus:outline-none focus:ring-2 focus:ring-primary focus:ring-opacity-50',
              isDisabled && 'opacity-50 cursor-not-allowed hover:bg-transparent',
            )}
          >
            <div className="flex w-full items-center justify-center gap-2">
              <img 
                src="/assets/Import.svg" 
                alt="Upload Document" 
                className="icon-md dark:invert"
                style={{ width: '20px', height: '20px' }}
              />
            </div>
          </button>
        }
        id="upload-document-button"
        description={localize('com_ui_upload')}
        disabled={isDisabled}
      />
      <Dialog open={showUploadDialog} onOpenChange={setShowUploadDialog}>
        <DialogContent className="max-w-md p-6" showCloseButton={true}>
          <DialogHeader className="mb-4">
            <DialogTitle className="text-xl font-semibold">
              {localize('com_ui_upload_files')}
            </DialogTitle>
          </DialogHeader>
          <DocumentUpload onUploadSuccess={handleUploadSuccess} />
        </DialogContent>
      </Dialog>
    </>
  );
};

export default React.memo(UploadButton);

