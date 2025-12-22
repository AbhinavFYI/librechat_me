-- Backfill uploaded_at for existing documents that have NULL uploaded_at
-- Set uploaded_at to created_at for documents where uploaded_at is NULL
-- This ensures all documents have a valid upload date

UPDATE documents
SET uploaded_at = created_at
WHERE uploaded_at IS NULL;

-- Verify the update (optional - for logging)
-- SELECT COUNT(*) as updated_count FROM documents WHERE uploaded_at IS NOT NULL;

