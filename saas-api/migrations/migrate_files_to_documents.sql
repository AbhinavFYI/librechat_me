-- ============================================================================
-- Migration: Replace files table with unified documents table
-- ============================================================================
-- This migration:
-- 1. Creates the new documents table if it doesn't exist
-- 2. Migrates data from files table to documents table (if files table exists)
-- 3. Drops the old files table
-- 4. Updates RLS policies
-- ============================================================================

BEGIN;

-- ============================================================================
-- Step 1: Create document_status enum if it doesn't exist
-- ============================================================================
DO $$ BEGIN
  CREATE TYPE document_status AS ENUM ('pending', 'processing', 'embedding', 'completed', 'failed');
EXCEPTION
  WHEN duplicate_object THEN null;
END $$;

-- ============================================================================
-- Step 2: Create documents table if it doesn't exist
-- ============================================================================
CREATE TABLE IF NOT EXISTS documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- Multi-tenant org scope
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  
  -- Hierarchy
  folder_id UUID REFERENCES folders(id) ON DELETE SET NULL,
  parent_id UUID REFERENCES documents(id) ON DELETE CASCADE,
  doc_id UUID,
  
  -- Minimal identity
  name VARCHAR(512) NOT NULL,
  
  -- File storage pointers
  file_path VARCHAR(1024),
  json_file_path VARCHAR(1024),
  
  -- Status
  status document_status DEFAULT 'completed',
  
  -- JSONB metadata for flexible storage
  content JSONB DEFAULT '{
    "description": null,
    "path": null,
    "mime_type": null,
    "size_bytes": 0,
    "version": 1,
    "checksum": null,
    "is_folder": false,
    "error_message": null,
    "processing_data": {}
  }'::jsonb NOT NULL,
  metadata JSONB DEFAULT '{}' NOT NULL,
  
  -- Audit
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP DEFAULT NOW() NOT NULL,
  uploaded_at TIMESTAMP,
  processed_at TIMESTAMP,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_at TIMESTAMP DEFAULT NOW() NOT NULL,
  deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
  deleted_at TIMESTAMP
);

-- ============================================================================
-- Step 3: Migrate data from files table to documents table (if files exists)
-- ============================================================================
DO $$
BEGIN
  -- Check if files table exists
  IF EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'files') THEN
    RAISE NOTICE 'Migrating data from files table to documents table...';
    
    -- Insert data from files to documents
    INSERT INTO documents (
      id,
      org_id,
      folder_id,
      name,
      file_path,
      status,
      content,
      metadata,
      created_by,
      created_at,
      updated_by,
      updated_at
    )
    SELECT 
      f.id,
      f.org_id,
      f.folder_id,
      f.name,
      f.storage_key,  -- storage_key becomes file_path
      'completed'::document_status,
      jsonb_build_object(
        'description', null,
        'path', f.storage_key,
        'mime_type', f.mime_type,
        'size_bytes', f.size_bytes,
        'version', f.version,
        'checksum', f.checksum,
        'is_folder', false,
        'error_message', null,
        'processing_data', '{}'::jsonb
      ),
      '{}'::jsonb,  -- empty metadata
      f.created_by,
      f.created_at,
      f.updated_by,
      f.updated_at
    FROM files f
    ON CONFLICT (id) DO NOTHING;  -- Skip if already exists
    
    RAISE NOTICE 'Data migration completed.';
  ELSE
    RAISE NOTICE 'Files table does not exist. Skipping data migration.';
  END IF;
END $$;

-- ============================================================================
-- Step 4: Create indexes on documents table
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_documents_org ON documents(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_folder ON documents(folder_id) WHERE folder_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_parent ON documents(parent_id) WHERE parent_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_created_by ON documents(created_by) WHERE created_by IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_created ON documents(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_file_path ON documents(file_path) WHERE file_path IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_content ON documents USING gin(content);
CREATE INDEX IF NOT EXISTS idx_documents_metadata ON documents USING gin(metadata);

-- ============================================================================
-- Step 5: Enable RLS on documents table
-- ============================================================================
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;

-- Drop old files policy if exists
DROP POLICY IF EXISTS files_isolation_policy ON files;

-- Create documents RLS policy
DROP POLICY IF EXISTS documents_isolation_policy ON documents;
CREATE POLICY documents_isolation_policy ON documents
  FOR ALL
  USING (
    current_setting('app.is_super_admin', true)::boolean = true OR
    org_id = current_setting('app.current_org_id', true)::uuid
  );

-- ============================================================================
-- Step 6: Create/update trigger for updated_at
-- ============================================================================
DROP TRIGGER IF EXISTS update_documents_updated_at ON documents;
CREATE TRIGGER update_documents_updated_at 
  BEFORE UPDATE ON documents
  FOR EACH ROW 
  EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Step 7: Drop old files table (if exists)
-- ============================================================================
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'files') THEN
    RAISE NOTICE 'Dropping old files table...';
    DROP TABLE files CASCADE;
    RAISE NOTICE 'Files table dropped successfully.';
  ELSE
    RAISE NOTICE 'Files table does not exist. Nothing to drop.';
  END IF;
END $$;

-- ============================================================================
-- Step 8: Add comments
-- ============================================================================
COMMENT ON TABLE documents IS 'Unified table for files and documents with vector embeddings support';
COMMENT ON COLUMN documents.file_path IS 'Physical file path relative to storage root (e.g., {org_id}/{folder_path}/{filename})';
COMMENT ON COLUMN documents.json_file_path IS 'Path to processed JSON chunks for vector embeddings';
COMMENT ON COLUMN documents.status IS 'Processing status: pending, processing, embedding, completed, failed';
COMMENT ON COLUMN documents.content IS 'JSONB containing file metadata (mime_type, size_bytes, version, checksum, etc.)';
COMMENT ON COLUMN documents.metadata IS 'JSONB for extensible metadata without schema changes';
COMMENT ON POLICY documents_isolation_policy ON documents IS 'RLS: Users can only see documents in their org';

COMMIT;

-- ============================================================================
-- Completion message
-- ============================================================================
DO $$
BEGIN
  RAISE NOTICE '
  ╔════════════════════════════════════════════════════════════════╗
  ║  Migration Complete!                                           ║
  ╠════════════════════════════════════════════════════════════════╣
  ║  ✓ Documents table created                                     ║
  ║  ✓ Data migrated from files table (if existed)                ║
  ║  ✓ Old files table dropped                                     ║
  ║  ✓ Indexes and RLS policies updated                           ║
  ║  ✓ Triggers configured                                         ║
  ╚════════════════════════════════════════════════════════════════╝
  ';
END $$;
