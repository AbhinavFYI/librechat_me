-- Migration: Add error_message column to documents table
-- This moves error_message from content JSONB to a dedicated column for better querying

ALTER TABLE documents
ADD COLUMN IF NOT EXISTS error_message TEXT;

-- Create index for efficient querying of failed documents
CREATE INDEX IF NOT EXISTS idx_documents_error_message 
ON documents(error_message) 
WHERE error_message IS NOT NULL AND deleted_at IS NULL;

-- Migrate existing error messages from content JSONB to new column (if any exist)
UPDATE documents
SET error_message = content->>'error_message'
WHERE content->>'error_message' IS NOT NULL 
  AND content->>'error_message' != 'null'
  AND error_message IS NULL;

-- Optional: Remove error_message from content JSONB after migration
-- UPDATE documents SET content = content - 'error_message';

COMMENT ON COLUMN documents.error_message IS 'Error message if document processing failed';

