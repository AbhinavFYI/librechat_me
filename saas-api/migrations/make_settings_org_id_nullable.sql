-- Migration: Make org_id nullable in settings table for superadmin support
-- Superadmins don't have an org_id, so we need to allow NULL values

DO $$
BEGIN
    -- Check if org_id is currently NOT NULL
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_schema = 'public' 
        AND table_name = 'settings' 
        AND column_name = 'org_id' 
        AND is_nullable = 'NO'
    ) THEN
        -- Drop the foreign key constraint temporarily
        ALTER TABLE settings DROP CONSTRAINT IF EXISTS settings_org_id_fkey;
        
        -- Make org_id nullable
        ALTER TABLE settings ALTER COLUMN org_id DROP NOT NULL;
        
        -- Re-add the foreign key constraint (allowing NULL)
        ALTER TABLE settings 
        ADD CONSTRAINT settings_org_id_fkey 
        FOREIGN KEY (org_id) 
        REFERENCES organizations(id) 
        ON DELETE CASCADE;
        
        RAISE NOTICE 'Settings.org_id is now nullable for superadmin support';
    ELSE
        RAISE NOTICE 'Settings.org_id is already nullable';
    END IF;
END $$;
