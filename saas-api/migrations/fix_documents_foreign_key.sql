-- ============================================================================
-- Fix: Update documents table foreign key constraint
-- ============================================================================
-- This fixes the folder_id foreign key to reference folders table instead of
-- documents table (self-reference)
-- ============================================================================

BEGIN;

-- Drop the incorrect foreign key constraint
ALTER TABLE documents 
  DROP CONSTRAINT IF EXISTS documents_folder_id_fkey;

-- Add the correct foreign key constraint
ALTER TABLE documents 
  ADD CONSTRAINT documents_folder_id_fkey 
  FOREIGN KEY (folder_id) 
  REFERENCES folders(id) 
  ON DELETE SET NULL;

-- Verify the constraint
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 
    FROM information_schema.table_constraints 
    WHERE constraint_name = 'documents_folder_id_fkey' 
    AND table_name = 'documents'
  ) THEN
    RAISE NOTICE '✓ Foreign key constraint fixed successfully';
  ELSE
    RAISE EXCEPTION '✗ Failed to create foreign key constraint';
  END IF;
END $$;

COMMIT;

-- ============================================================================
-- Completion message
-- ============================================================================
DO $$
BEGIN
  RAISE NOTICE '
  ╔════════════════════════════════════════════════════════════════╗
  ║  Foreign Key Constraint Fixed!                                 ║
  ╠════════════════════════════════════════════════════════════════╣
  ║  ✓ documents.folder_id now references folders(id)             ║
  ║  ✓ ON DELETE SET NULL behavior configured                     ║
  ╚════════════════════════════════════════════════════════════════╝
  ';
END $$;
