-- Migration: Create settings table for screeners
-- This table stores screener configurations with multi-tenant security

-- Check if table exists, if not create it
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'settings') THEN
        CREATE TABLE settings (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            -- Multi-tenant security scopes
            -- org_id is nullable for superadmins who don't belong to an organization
            org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            -- Screener identification
            screener_name TEXT NOT NULL,
            "tableName" TEXT,
            -- Screener logic
            query TEXT NOT NULL,
            "universeList" TEXT,
            explainer TEXT,
            -- Flags
            is_active BOOLEAN DEFAULT TRUE,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW()
        );

        -- Indexes for optimized fetching
        CREATE INDEX idx_settings_org_user ON settings(org_id, user_id);
        CREATE INDEX idx_settings_user ON settings(user_id);
        CREATE INDEX idx_settings_org ON settings(org_id);
        CREATE INDEX idx_settings_type ON settings(screener_name);
        CREATE INDEX idx_settings_active ON settings(is_active);

        -- Add trigger for updated_at
        CREATE TRIGGER update_settings_updated_at 
            BEFORE UPDATE ON settings
            FOR EACH ROW 
            EXECUTE FUNCTION update_updated_at_column();

        COMMENT ON TABLE settings IS 'Stores screener configurations with multi-tenant security';
        COMMENT ON COLUMN settings.org_id IS 'Organization ID for multi-tenant isolation (nullable for superadmins)';
        COMMENT ON COLUMN settings.user_id IS 'User ID who created the screener';
        COMMENT ON COLUMN settings.screener_name IS 'Name/identifier of the screener';
        COMMENT ON COLUMN settings."tableName" IS 'Table name used in the query';
        COMMENT ON COLUMN settings.query IS 'SQL query for the screener';
        COMMENT ON COLUMN settings."universeList" IS 'Universe list for the screener';
        COMMENT ON COLUMN settings.explainer IS 'Human-readable explanation of the screener';
        COMMENT ON COLUMN settings.is_active IS 'Whether the screener is active';

        RAISE NOTICE 'Settings table created successfully';
    ELSE
        RAISE NOTICE 'Settings table already exists';
    END IF;
END $$;
