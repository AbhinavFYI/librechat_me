-- ============================================================================
-- Migration: Create documents table with BIGSERIAL primary key
-- ============================================================================
-- This migration creates the documents table with BIGSERIAL id
-- Run this ONLY if you're setting up a fresh database
-- ============================================================================

BEGIN;

-- Step 1: Create document_status enum if it doesn't exist
DO $$ BEGIN
    CREATE TYPE document_status AS ENUM ('pending', 'processing', 'embedding', 'completed', 'failed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Step 2: Drop documents table if it exists (WARNING: This deletes all data!)
DROP TABLE IF EXISTS documents CASCADE;

-- Step 3: Create documents table with BIGSERIAL id
CREATE TABLE documents (
  id BIGSERIAL PRIMARY KEY,
  
  -- Multi-tenant org scope (nullable for superadmin documents)
  org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  
  -- Hierarchy
  folder_id UUID REFERENCES folders(id) ON DELETE SET NULL,
  parent_id BIGINT REFERENCES documents(id) ON DELETE CASCADE,
  
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

-- Step 4: Create indexes
CREATE INDEX idx_documents_org ON documents(org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_folder ON documents(folder_id) WHERE folder_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_documents_parent ON documents(parent_id) WHERE parent_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_documents_status ON documents(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_created_by ON documents(created_by) WHERE created_by IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_documents_created ON documents(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_file_path ON documents(file_path) WHERE file_path IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_documents_content ON documents USING gin(content);
CREATE INDEX idx_documents_metadata ON documents USING gin(metadata);

-- Step 5: Enable RLS
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;

-- Step 6: Create RLS policy
DROP POLICY IF EXISTS documents_isolation_policy ON documents;
CREATE POLICY documents_isolation_policy ON documents
  FOR ALL
  USING (
    current_setting('app.is_super_admin', true)::boolean = true OR
    (org_id IS NOT NULL AND org_id = current_setting('app.current_org_id', true)::uuid) OR
    (org_id IS NULL AND current_setting('app.is_super_admin', true)::boolean = true)
  );

-- Step 7: Create trigger for updated_at
DROP TRIGGER IF EXISTS update_documents_updated_at ON documents;
CREATE TRIGGER update_documents_updated_at 
  BEFORE UPDATE ON documents
  FOR EACH ROW 
  EXECUTE FUNCTION update_updated_at_column();

-- Step 8: Add comments
COMMENT ON TABLE documents IS 'Unified table for files and documents with vector embeddings support (BIGSERIAL id)';
COMMENT ON COLUMN documents.id IS 'Auto-incrementing BIGINT primary key (document ID)';
COMMENT ON COLUMN documents.org_id IS 'Organization ID (nullable for superadmin documents)';
COMMENT ON COLUMN documents.file_path IS 'Physical file path relative to storage root (e.g., {org_id}/{folder_path}/{filename})';
COMMENT ON COLUMN documents.json_file_path IS 'Path to processed JSON chunks for vector embeddings';
COMMENT ON COLUMN documents.status IS 'Processing status: pending, processing, embedding, completed, failed';
COMMENT ON COLUMN documents.content IS 'JSONB containing file metadata (mime_type, size_bytes, version, checksum, etc.)';
COMMENT ON COLUMN documents.metadata IS 'JSONB for extensible metadata without schema changes';
COMMENT ON POLICY documents_isolation_policy ON documents IS 'RLS: Users can only see documents in their org, superadmins see all';

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
  ║  ✓ documents table created with BIGSERIAL id                  ║
  ║  ✓ doc_id column removed (id IS the document ID)              ║
  ║  ✓ org_id is nullable for superadmins                         ║
  ║  ✓ All indexes created                                         ║
  ║  ✓ RLS policies configured                                     ║
  ║  ✓ Triggers configured                                         ║
  ╚════════════════════════════════════════════════════════════════╝
  ';
END $$;
