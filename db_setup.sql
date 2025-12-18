-- ============================================================================
-- Multi-Tenant SaaS User Management System - Database Setup
-- ============================================================================
-- PostgreSQL 14+ Required
-- Features: RBAC, Multi-tenancy, Audit Logging, Soft Deletes, RLS
-- ============================================================================

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- ENUMS (Optional but recommended for type safety)
-- ============================================================================

CREATE TYPE organization_status AS ENUM ('active', 'suspended', 'pending', 'deleted');
CREATE TYPE user_status AS ENUM ('active', 'suspended', 'pending', 'deleted');
CREATE TYPE subscription_plan AS ENUM ('free', 'starter', 'pro', 'enterprise', 'trial');
CREATE TYPE subscription_status AS ENUM ('active', 'past_due', 'cancelled', 'trialing', 'suspended');
CREATE TYPE role_type AS ENUM ('system', 'org_defined');

-- ============================================================================
-- TABLE: organizations
-- ============================================================================

CREATE TABLE organizations (
  -- Identity
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(255) NOT NULL,
  legal_name VARCHAR(255),
  slug VARCHAR(100) UNIQUE NOT NULL,
  
  -- Branding
  logo_url TEXT,  -- Store base64 encoded image data
  website VARCHAR(255),
  
  -- Address (structured for search/validation)
  address_line1 VARCHAR(255),
  address_line2 VARCHAR(255),
  city VARCHAR(100),
  state_province VARCHAR(100),
  postal_code VARCHAR(20),
  country CHAR(10), -- ISO 3166-1 alpha-2 (US, IN, GB, etc.)
  
  -- Contact Information
  primary_contact_name VARCHAR(255),
  primary_contact_email VARCHAR(255),
  primary_contact_phone VARCHAR(20),
  billing_email VARCHAR(255),
  
  -- Subscription & Billing
  subscription_plan subscription_plan DEFAULT 'free',
  subscription_status subscription_status DEFAULT 'active',
  trial_ends_at TIMESTAMP,
  subscription_starts_at TIMESTAMP DEFAULT NOW(),
  subscription_ends_at TIMESTAMP,
  
  -- Plan Limits (enforced by application)
  max_users INTEGER DEFAULT 5,
  max_storage_gb INTEGER DEFAULT 10,
  current_users INTEGER DEFAULT 0,
  current_storage_gb DECIMAL(10,2) DEFAULT 0.00,
  
  -- Preferences (frequently accessed)
  timezone VARCHAR(50) DEFAULT 'UTC',
  date_format VARCHAR(20) DEFAULT 'YYYY-MM-DD',
  locale VARCHAR(10) DEFAULT 'en-US',
  
  -- Flexible Settings (JSONB for extensibility)
  settings JSONB DEFAULT '{
    "theme": {
      "primary_color": "#3B82F6",
      "secondary_color": "#10B981"
    },
    "features": {
      "api_access": false,
      "sso_enabled": false,
      "custom_branding": false,
      "white_labeling": false
    },
    "notifications": {
      "digest_frequency": "weekly",
      "email_notifications": true,
      "slack_webhook": null
    },
    "security": {
      "password_policy": "standard",
      "mfa_required": false,
      "session_timeout_minutes": 60,
      "ip_whitelist": []
    }
  }'::jsonb,
  
  -- Status & Lifecycle
  status organization_status DEFAULT 'active',
  deleted_at TIMESTAMP,
  deleted_by UUID,
  deleted_reason TEXT,
  
  -- Audit Trail
  created_at TIMESTAMP DEFAULT NOW(),
  created_by UUID,
  updated_at TIMESTAMP DEFAULT NOW(),
  updated_by UUID,
  
  -- Future: Hierarchy support
  parent_org_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
  
  -- Metadata (for extensibility without migrations)
  metadata JSONB DEFAULT '{}',
  
  -- Constraints
  CONSTRAINT check_slug_format CHECK (slug ~ '^[a-z0-9-]+$'),
  CONSTRAINT check_trial_logic CHECK (
    (subscription_status = 'trialing' AND trial_ends_at IS NOT NULL) OR
    (subscription_status != 'trialing')
  ),
  CONSTRAINT check_user_limits CHECK (current_users >= 0 AND current_users <= max_users),
  CONSTRAINT check_email_format CHECK (
    primary_contact_email IS NULL OR primary_contact_email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'
  )
);

-- Indexes
CREATE INDEX idx_orgs_status ON organizations(status) WHERE status != 'deleted';
CREATE INDEX idx_orgs_subscription ON organizations(subscription_plan, subscription_status);
CREATE INDEX idx_orgs_country ON organizations(country) WHERE country IS NOT NULL;
CREATE INDEX idx_orgs_slug ON organizations(slug);
CREATE INDEX idx_orgs_created ON organizations(created_at DESC);
CREATE INDEX idx_orgs_parent ON organizations(parent_org_id) WHERE parent_org_id IS NOT NULL;
CREATE INDEX idx_orgs_settings ON organizations USING gin(settings);

COMMENT ON TABLE organizations IS 'Multi-tenant organizations with subscription and billing info';
COMMENT ON COLUMN organizations.slug IS 'URL-safe unique identifier for subdomains';
COMMENT ON COLUMN organizations.settings IS 'Flexible JSONB settings for org-specific configurations';

-- ============================================================================
-- TABLE: users
-- ============================================================================

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  
  -- Authentication
  email VARCHAR(255) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  
  -- User Info
  first_name VARCHAR(100),
  last_name VARCHAR(100),
  full_name VARCHAR(255) GENERATED ALWAYS AS (
    CASE 
      WHEN first_name IS NOT NULL AND last_name IS NOT NULL 
      THEN first_name || ' ' || last_name
      ELSE COALESCE(first_name, last_name, email)
    END
  ) STORED,
  avatar_url VARCHAR(500),
  phone VARCHAR(20),
  
  -- Special Privileges
  is_super_admin BOOLEAN DEFAULT false,
  org_role VARCHAR(20) DEFAULT 'user',  -- OrgRole: 'admin' (can login via OTP), 'user' (default), 'viewer' (read-only). Separate from Roles which determine permissions.
  
  -- Status & Security
  status user_status DEFAULT 'pending',
  email_verified BOOLEAN DEFAULT false,
  email_verified_at TIMESTAMP,
  last_login_at TIMESTAMP,
  last_login_ip INET,
  failed_login_attempts INTEGER DEFAULT 0,
  locked_until TIMESTAMP,
  
  -- OTP for login (email-based authentication)
  otp_code VARCHAR(6),              -- 6-digit OTP code
  otp_expires_at TIMESTAMP,         -- OTP expiration timestamp (typically 10 minutes)
  otp_attempts INTEGER DEFAULT 0,    -- Number of failed OTP verification attempts (max 5)
  
  -- Preferences
  timezone VARCHAR(50) DEFAULT 'UTC',
  locale VARCHAR(10) DEFAULT 'en-US',
  
  -- Metadata
  metadata JSONB DEFAULT '{}',
  
  -- Audit
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  deleted_at TIMESTAMP,
  
  -- Constraints
  CONSTRAINT check_super_admin_org CHECK (
    (is_super_admin = true AND org_id IS NULL) OR
    (is_super_admin = false AND org_id IS NOT NULL)
  ),
  CONSTRAINT check_email_format CHECK (
    email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'
  ),
  CONSTRAINT check_org_role CHECK (
    org_role IS NULL OR org_role IN ('admin', 'user', 'viewer')
  )
);

-- Indexes
CREATE INDEX idx_users_org ON users(org_id) WHERE org_id IS NOT NULL;
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status) WHERE status = 'active';
CREATE INDEX idx_users_super_admin ON users(is_super_admin) WHERE is_super_admin = true;
CREATE INDEX idx_users_created ON users(created_at DESC);
CREATE INDEX idx_users_last_login ON users(last_login_at DESC NULLS LAST);

COMMENT ON TABLE users IS 'Users with org association. Super admins have org_id = NULL';
COMMENT ON COLUMN users.is_super_admin IS 'Platform-level admin with cross-org access';
COMMENT ON COLUMN users.org_role IS 'OrgRole: admin (can login via OTP), user (default), viewer (read-only). Separate from Roles which determine permissions.';
COMMENT ON COLUMN users.locked_until IS 'Account locked due to failed login attempts until this timestamp';
COMMENT ON COLUMN users.otp_code IS '6-digit OTP code for email-based login (stored temporarily)';
COMMENT ON COLUMN users.otp_expires_at IS 'OTP expiration timestamp (typically 10 minutes from generation)';
COMMENT ON COLUMN users.otp_attempts IS 'Number of failed OTP verification attempts (max 5 before OTP is invalidated)';

-- ============================================================================
-- TABLE: permissions
-- ============================================================================

CREATE TABLE permissions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- Permission definition
  resource VARCHAR(50) NOT NULL,
  action VARCHAR(20) NOT NULL,
  description TEXT,
  
  -- Scope
  is_system BOOLEAN DEFAULT false,
  
  -- Metadata
  created_at TIMESTAMP DEFAULT NOW(),
  
  -- Constraints
  UNIQUE(resource, action)
);

-- Indexes
CREATE INDEX idx_permissions_resource ON permissions(resource);
CREATE INDEX idx_permissions_system ON permissions(is_system) WHERE is_system = true;

COMMENT ON TABLE permissions IS 'System-wide permission definitions (resource + action pairs)';
COMMENT ON COLUMN permissions.is_system IS 'System permissions cannot be deleted';

-- ============================================================================
-- TABLE: roles
-- ============================================================================

CREATE TABLE roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  
  -- Role definition
  name VARCHAR(100) NOT NULL,
  type role_type NOT NULL,
  description TEXT,
  
  -- Metadata
  is_default BOOLEAN DEFAULT false,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  created_by UUID REFERENCES users(id),
  
  -- Constraints
  CONSTRAINT check_role_org CHECK (
    (type = 'system' AND org_id IS NULL) OR
    (type = 'org_defined' AND org_id IS NOT NULL)
  ),
  UNIQUE(org_id, name)
);

-- Indexes
CREATE INDEX idx_roles_org ON roles(org_id) WHERE org_id IS NOT NULL;
CREATE INDEX idx_roles_type ON roles(type);
CREATE INDEX idx_roles_default ON roles(is_default) WHERE is_default = true;

COMMENT ON TABLE roles IS 'Roles define collections of permissions. System roles have org_id = NULL';
COMMENT ON COLUMN roles.is_default IS 'Auto-assigned to new users in this org';

-- ============================================================================
-- TABLE: role_permissions
-- ============================================================================

CREATE TABLE role_permissions (
  role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
  permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
  
  -- Metadata
  granted_at TIMESTAMP DEFAULT NOW(),
  granted_by UUID REFERENCES users(id),
  
  PRIMARY KEY (role_id, permission_id)
);

-- Indexes
CREATE INDEX idx_role_perms_role ON role_permissions(role_id);
CREATE INDEX idx_role_perms_perm ON role_permissions(permission_id);

COMMENT ON TABLE role_permissions IS 'Many-to-many mapping of roles to permissions';

-- ============================================================================
-- TABLE: user_roles
-- ============================================================================

CREATE TABLE user_roles (
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
  
  -- Audit
  assigned_by UUID REFERENCES users(id) NOT NULL,
  assigned_at TIMESTAMP DEFAULT NOW(),
  expires_at TIMESTAMP,
  
  PRIMARY KEY (user_id, role_id),
  
  -- Constraints
  CONSTRAINT check_role_expiration CHECK (
    expires_at IS NULL OR expires_at > assigned_at
  )
);

-- Indexes
CREATE INDEX idx_user_roles_user ON user_roles(user_id);
CREATE INDEX idx_user_roles_role ON user_roles(role_id);
CREATE INDEX idx_user_roles_assigned_by ON user_roles(assigned_by);
CREATE INDEX idx_user_roles_expires ON user_roles(expires_at) WHERE expires_at IS NOT NULL;

COMMENT ON TABLE user_roles IS 'Many-to-many mapping of users to roles with audit trail';
COMMENT ON COLUMN user_roles.expires_at IS 'Optional expiration for temporary role assignments';

-- ============================================================================
-- TABLE: refresh_tokens
-- ============================================================================

CREATE TABLE refresh_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE CASCADE NOT NULL,
  
  -- Token info
  token_hash VARCHAR(255) UNIQUE NOT NULL,
  device_info JSONB DEFAULT '{}',
  ip_address INET,
  user_agent TEXT,
  
  -- Lifecycle
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  last_used_at TIMESTAMP DEFAULT NOW(),
  revoked_at TIMESTAMP,
  revoked_by UUID REFERENCES users(id),
  revoked_reason VARCHAR(100),
  
  -- Constraints
  CONSTRAINT check_token_expiry CHECK (expires_at > created_at)
);

-- Indexes
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_refresh_tokens_revoked ON refresh_tokens(revoked_at) WHERE revoked_at IS NOT NULL;

COMMENT ON TABLE refresh_tokens IS 'JWT refresh tokens for session management';
COMMENT ON COLUMN refresh_tokens.token_hash IS 'SHA-256 hash of the refresh token';

-- ============================================================================
-- TABLE: audit_logs
-- ============================================================================

CREATE TABLE audit_logs (
  id BIGSERIAL PRIMARY KEY,
  
  -- Who & Where
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  org_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
  ip_address INET,
  user_agent TEXT,
  
  -- What happened
  action VARCHAR(100) NOT NULL,
  resource_type VARCHAR(50),
  resource_id UUID,
  
  -- Details
  status VARCHAR(20) DEFAULT 'success',
  metadata JSONB DEFAULT '{}',
  error_message TEXT,
  
  -- When
  created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes (partitioning by time recommended for production)
CREATE INDEX idx_audit_org_time ON audit_logs(org_id, created_at DESC) WHERE org_id IS NOT NULL;
CREATE INDEX idx_audit_user_time ON audit_logs(user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX idx_audit_action ON audit_logs(action, created_at DESC);
CREATE INDEX idx_audit_resource ON audit_logs(resource_type, resource_id) WHERE resource_id IS NOT NULL;
CREATE INDEX idx_audit_status ON audit_logs(status) WHERE status != 'success';
CREATE INDEX idx_audit_created ON audit_logs(created_at DESC);

COMMENT ON TABLE audit_logs IS 'Comprehensive audit trail of all system actions';
COMMENT ON COLUMN audit_logs.metadata IS 'JSON containing before/after values, request params, etc.';

-- ============================================================================
-- TABLE: password_resets
-- ============================================================================

CREATE TABLE password_resets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE CASCADE NOT NULL,
  
  token_hash VARCHAR(255) UNIQUE NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used_at TIMESTAMP,
  ip_address INET,
  
  created_at TIMESTAMP DEFAULT NOW(),
  
  CONSTRAINT check_password_reset_expiry CHECK (expires_at > created_at)
);

-- Indexes
CREATE INDEX idx_password_resets_user ON password_resets(user_id);
CREATE INDEX idx_password_resets_token ON password_resets(token_hash) WHERE used_at IS NULL;
CREATE INDEX idx_password_resets_expires ON password_resets(expires_at) WHERE used_at IS NULL;

COMMENT ON TABLE password_resets IS 'Password reset tokens with expiration tracking';

-- ============================================================================
-- TABLE: email_verification_tokens
-- ============================================================================

CREATE TABLE email_verification_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE CASCADE NOT NULL,
  
  token_hash VARCHAR(255) UNIQUE NOT NULL,
  email VARCHAR(255) NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  verified_at TIMESTAMP,
  
  created_at TIMESTAMP DEFAULT NOW(),
  
  CONSTRAINT check_email_verify_expiry CHECK (expires_at > created_at)
);

-- Indexes
CREATE INDEX idx_email_verify_user ON email_verification_tokens(user_id);
CREATE INDEX idx_email_verify_token ON email_verification_tokens(token_hash) WHERE verified_at IS NULL;

COMMENT ON TABLE email_verification_tokens IS 'Email verification tokens for new users';

-- ============================================================================
-- TABLE: org_invitations
-- ============================================================================

CREATE TABLE org_invitations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID REFERENCES organizations(id) ON DELETE CASCADE NOT NULL,
  
  email VARCHAR(255) NOT NULL,
  role_id UUID REFERENCES roles(id) ON DELETE CASCADE NOT NULL,
  token_hash VARCHAR(255) UNIQUE NOT NULL,
  
  invited_by UUID REFERENCES users(id) NOT NULL,
  invited_at TIMESTAMP DEFAULT NOW(),
  expires_at TIMESTAMP NOT NULL,
  
  accepted_at TIMESTAMP,
  accepted_by UUID REFERENCES users(id),
  
  cancelled_at TIMESTAMP,
  cancelled_by UUID REFERENCES users(id),
  
  CONSTRAINT check_invitation_expiry CHECK (expires_at > invited_at)
);

-- Indexes
CREATE INDEX idx_invitations_org ON org_invitations(org_id);
CREATE INDEX idx_invitations_email ON org_invitations(email);
CREATE INDEX idx_invitations_token ON org_invitations(token_hash) WHERE accepted_at IS NULL AND cancelled_at IS NULL;
CREATE INDEX idx_invitations_pending ON org_invitations(org_id, invited_at) WHERE accepted_at IS NULL AND cancelled_at IS NULL;

COMMENT ON TABLE org_invitations IS 'Pending invitations to join organizations';

-- ============================================================================
-- TABLE: templates
-- ============================================================================

CREATE TABLE templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  
  -- Template Info
  name VARCHAR(255) NOT NULL,
  description TEXT,
  framework VARCHAR(50),  -- 'R-T-F', 'T-A-G', 'B-A-B', 'C-A-R-E', 'R-I-S-E', or 'custom'
  is_custom BOOLEAN DEFAULT false,
  
  -- Template Content (JSONB for flexible framework fields)
  content JSONB DEFAULT '{}',
  
  -- Audit
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
  deleted_at TIMESTAMP,
  deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
  
  -- Constraints
  CONSTRAINT check_template_framework CHECK (
    framework IS NULL OR framework IN ('R-T-F', 'T-A-G', 'B-A-B', 'C-A-R-E', 'R-I-S-E', 'custom')
  )
);

-- Indexes
CREATE INDEX idx_templates_org ON templates(org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_templates_created_by ON templates(created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_templates_framework ON templates(framework) WHERE deleted_at IS NULL;
CREATE INDEX idx_templates_created ON templates(created_at DESC) WHERE deleted_at IS NULL;

COMMENT ON TABLE templates IS 'Templates for creating personas and prompts with various frameworks';
COMMENT ON COLUMN templates.org_id IS 'Organization that owns this template (NULL for system templates)';
COMMENT ON COLUMN templates.framework IS 'Template framework type: R-T-F, T-A-G, B-A-B, C-A-R-E, R-I-S-E, or custom';
COMMENT ON COLUMN templates.content IS 'JSONB containing framework-specific fields and values';
COMMENT ON COLUMN templates.is_custom IS 'Whether this is a custom template created by the user';

-- ============================================================================
-- TABLE: personas
-- ============================================================================

CREATE TABLE personas (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  template_id UUID REFERENCES templates(id) ON DELETE SET NULL,
  
  -- Persona Info
  name VARCHAR(255) NOT NULL,
  description TEXT,
  
  -- Persona Content (can use template or custom)
  content JSONB DEFAULT '{}',
  is_custom_template BOOLEAN DEFAULT false,  -- If true, uses custom template content instead of template_id
  
  -- Audit
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
  deleted_at TIMESTAMP,
  deleted_by UUID REFERENCES users(id) ON DELETE SET NULL
);

-- Indexes
CREATE INDEX idx_personas_org ON personas(org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_personas_template ON personas(template_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_personas_created_by ON personas(created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_personas_created ON personas(created_at DESC) WHERE deleted_at IS NULL;

COMMENT ON TABLE personas IS 'Personas created from templates or custom templates';
COMMENT ON COLUMN personas.template_id IS 'Template used to create this persona (NULL if using custom template)';
COMMENT ON COLUMN personas.is_custom_template IS 'If true, persona uses custom template content instead of a template_id';
COMMENT ON COLUMN personas.content IS 'JSONB containing persona-specific data and template fields';

-- ============================================================================
-- TABLE: folders
-- ============================================================================

CREATE TABLE folders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID REFERENCES organizations(id) ON DELETE CASCADE NOT NULL,
  parent_id UUID REFERENCES folders(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  path TEXT NOT NULL, -- canonical path like /root/team/reports
  created_by UUID REFERENCES users(id),
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_folders_org_path ON folders(org_id, path);
CREATE INDEX idx_folders_org ON folders(org_id);
CREATE INDEX idx_folders_parent ON folders(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_folders_created_by ON folders(created_by) WHERE created_by IS NOT NULL;

COMMENT ON TABLE folders IS 'Folder structure for organizing files within organizations';
COMMENT ON COLUMN folders.path IS 'Canonical path like /root/team/reports';
COMMENT ON COLUMN folders.parent_id IS 'Parent folder ID (NULL for root folders)';

-- ============================================================================
-- TABLE: documents (Unified files and documents table)
-- ============================================================================

-- Create document_status enum if it doesn't exist
DO $$ BEGIN
  CREATE TYPE document_status AS ENUM ('pending', 'processing', 'embedding', 'completed', 'failed');
EXCEPTION
  WHEN duplicate_object THEN null;
END $$;

CREATE TABLE documents (
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

-- Indexes
CREATE INDEX idx_documents_org ON documents(org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_folder ON documents(folder_id) WHERE folder_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_documents_parent ON documents(parent_id) WHERE parent_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_documents_status ON documents(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_created_by ON documents(created_by) WHERE created_by IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_documents_created ON documents(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_file_path ON documents(file_path) WHERE file_path IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_documents_content ON documents USING gin(content);
CREATE INDEX idx_documents_metadata ON documents USING gin(metadata);

COMMENT ON TABLE documents IS 'Unified table for files and documents with vector embeddings support';
COMMENT ON COLUMN documents.file_path IS 'Physical file path relative to storage root (e.g., {org_id}/{folder_path}/{filename})';
COMMENT ON COLUMN documents.json_file_path IS 'Path to processed JSON chunks for vector embeddings';
COMMENT ON COLUMN documents.status IS 'Processing status: pending, processing, embedding, completed, failed';
COMMENT ON COLUMN documents.content IS 'JSONB containing file metadata (mime_type, size_bytes, version, checksum, etc.)';
COMMENT ON COLUMN documents.metadata IS 'JSONB for extensible metadata without schema changes';

-- ============================================================================
-- TABLE: folder_permissions
-- ============================================================================

CREATE TABLE folder_permissions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  folder_id UUID REFERENCES folders(id) ON DELETE CASCADE,
  role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
  permission TEXT NOT NULL,   -- read, write, delete, move, share
  created_at TIMESTAMPTZ DEFAULT now(),
  
  CONSTRAINT check_folder_permission CHECK (
    permission IN ('read', 'write', 'delete', 'move', 'share')
  )
);

CREATE INDEX idx_folder_permissions_folder ON folder_permissions(folder_id);
CREATE INDEX idx_folder_permissions_role ON folder_permissions(role_id);
CREATE UNIQUE INDEX idx_folder_permissions_unique ON folder_permissions(folder_id, role_id);

COMMENT ON TABLE folder_permissions IS 'Permissions for roles on specific folders';
COMMENT ON COLUMN folder_permissions.permission IS 'Permission type: read, write, delete, move, share';

-- ============================================================================
-- TABLE: settings (Screener configurations)
-- ============================================================================

CREATE TABLE settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Multi-tenant security scopes
    -- org_id is nullable for superadmins who don't belong to an organization
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Screener identification
    screener_name TEXT NOT NULL,
    tableName TEXT,
    -- Screener logic
    query TEXT NOT NULL,
    universeList TEXT,
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


COMMENT ON TABLE settings IS 'Stores screener configurations with multi-tenant security';
COMMENT ON COLUMN settings.org_id IS 'Organization ID for multi-tenant isolation (nullable for superadmins)';
COMMENT ON COLUMN settings.user_id IS 'User ID who created the screener';
COMMENT ON COLUMN settings.screener_name IS 'Name/identifier of the screener';
COMMENT ON COLUMN settings.tableName IS 'Table name used in the query';
COMMENT ON COLUMN settings.query IS 'SQL query for the screener';
COMMENT ON COLUMN settings.universeList IS 'Universe list for the screener';
COMMENT ON COLUMN settings.explainer IS 'Human-readable explanation of the screener';
COMMENT ON COLUMN settings.is_active IS 'Whether the screener is active';


-- ============================================================================
-- TRIGGERS: updated_at automation
-- ============================================================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_orgs_updated_at 
  BEFORE UPDATE ON organizations
  FOR EACH ROW 
  EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at 
  BEFORE UPDATE ON users
  FOR EACH ROW 
  EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_roles_updated_at 
  BEFORE UPDATE ON roles
  FOR EACH ROW 
  EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_templates_updated_at 
  BEFORE UPDATE ON templates
  FOR EACH ROW 
  EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_personas_updated_at 
  BEFORE UPDATE ON personas
  FOR EACH ROW 
  EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_folders_updated_at 
  BEFORE UPDATE ON folders
  FOR EACH ROW 
  EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_documents_updated_at 
  BEFORE UPDATE ON documents
  FOR EACH ROW 
  EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_settings_updated_at 
  BEFORE UPDATE ON settings
  FOR EACH ROW 
  EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- TRIGGERS: Auto-increment current_users on org
-- ============================================================================

CREATE OR REPLACE FUNCTION update_org_user_count()
RETURNS TRIGGER AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    UPDATE organizations 
    SET current_users = current_users + 1
    WHERE id = NEW.org_id AND NEW.org_id IS NOT NULL;
  ELSIF TG_OP = 'DELETE' THEN
    UPDATE organizations 
    SET current_users = GREATEST(0, current_users - 1)
    WHERE id = OLD.org_id AND OLD.org_id IS NOT NULL;
  ELSIF TG_OP = 'UPDATE' AND OLD.org_id IS DISTINCT FROM NEW.org_id THEN
    UPDATE organizations 
    SET current_users = GREATEST(0, current_users - 1)
    WHERE id = OLD.org_id AND OLD.org_id IS NOT NULL;
    
    UPDATE organizations 
    SET current_users = current_users + 1
    WHERE id = NEW.org_id AND NEW.org_id IS NOT NULL;
  END IF;
  
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_org_user_count
  AFTER INSERT OR UPDATE OR DELETE ON users
  FOR EACH ROW
  EXECUTE FUNCTION update_org_user_count();

-- ============================================================================
-- ROW LEVEL SECURITY (RLS) POLICIES
-- ============================================================================

-- Enable RLS on tenant-scoped tables
ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE folders ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE folder_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE settings ENABLE ROW LEVEL SECURITY;

-- Organizations: Super admins see all, users see their org
CREATE POLICY org_isolation_policy ON organizations
  FOR ALL
  USING (
    current_setting('app.is_super_admin', true)::boolean = true OR
    id = current_setting('app.current_org_id', true)::uuid
  );

-- Users: Super admins see all, users see their org
CREATE POLICY user_isolation_policy ON users
  FOR ALL
  USING (
    current_setting('app.is_super_admin', true)::boolean = true OR
    org_id = current_setting('app.current_org_id', true)::uuid OR
    id = current_setting('app.current_user_id', true)::uuid
  );

-- Roles: Super admins see all, users see system roles + their org roles
CREATE POLICY role_isolation_policy ON roles
  FOR ALL
  USING (
    current_setting('app.is_super_admin', true)::boolean = true OR
    org_id IS NULL OR
    org_id = current_setting('app.current_org_id', true)::uuid
  );

-- User Roles: Users see roles in their org
CREATE POLICY user_roles_isolation_policy ON user_roles
  FOR ALL
  USING (
    EXISTS (
      SELECT 1 FROM users 
      WHERE users.id = user_roles.user_id 
      AND (
        users.org_id = current_setting('app.current_org_id', true)::uuid OR
        current_setting('app.is_super_admin', true)::boolean = true
      )
    )
  );

-- Audit Logs: Users see their org's logs, super admins see all
CREATE POLICY audit_isolation_policy ON audit_logs
  FOR SELECT
  USING (
    current_setting('app.is_super_admin', true)::boolean = true OR
    org_id = current_setting('app.current_org_id', true)::uuid OR
    user_id = current_setting('app.current_user_id', true)::uuid
  );

-- Folders: Users see folders in their org
CREATE POLICY folders_isolation_policy ON folders
  FOR ALL
  USING (
    current_setting('app.is_super_admin', true)::boolean = true OR
    org_id = current_setting('app.current_org_id', true)::uuid
  );

-- Documents: Users see documents in their org
CREATE POLICY documents_isolation_policy ON documents
  FOR ALL
  USING (
    current_setting('app.is_super_admin', true)::boolean = true OR
    org_id = current_setting('app.current_org_id', true)::uuid
  );

-- Folder Permissions: Users see permissions for folders in their org
CREATE POLICY folder_permissions_isolation_policy ON folder_permissions
  FOR ALL
  USING (
    EXISTS (
      SELECT 1 FROM folders 
      WHERE folders.id = folder_permissions.folder_id 
      AND (
        folders.org_id = current_setting('app.current_org_id', true)::uuid OR
        current_setting('app.is_super_admin', true)::boolean = true
      )
    )
  );

-- Settings: Super admins see all, org admins see org screeners, users see only their own
CREATE POLICY settings_isolation_policy ON settings
  FOR ALL
  USING (
    current_setting('app.is_super_admin', true)::boolean = true OR
    org_id = current_setting('app.current_org_id', true)::uuid OR
    user_id = current_setting('app.current_user_id', true)::uuid
  );

COMMENT ON POLICY org_isolation_policy ON organizations IS 'RLS: Enforce multi-tenant isolation';
COMMENT ON POLICY user_isolation_policy ON users IS 'RLS: Users can only see users in their org';
COMMENT ON POLICY folders_isolation_policy ON folders IS 'RLS: Users can only see folders in their org';
COMMENT ON POLICY documents_isolation_policy ON documents IS 'RLS: Users can only see documents in their org';
COMMENT ON POLICY settings_isolation_policy ON settings IS 'RLS: Super admins see all, org admins see org screeners, users see only their own';

-- ============================================================================
-- HELPER FUNCTIONS
-- ============================================================================

-- Function: Get user by email for authentication (bypasses RLS)
-- This function is used during login when RLS session variables are not yet set
CREATE OR REPLACE FUNCTION get_user_by_email_for_auth(p_email VARCHAR)
RETURNS TABLE (
  id UUID,
  org_id UUID,
  email VARCHAR,
  password_hash VARCHAR,
  first_name VARCHAR,
  last_name VARCHAR,
  full_name VARCHAR,
  avatar_url VARCHAR,
  phone VARCHAR,
  is_super_admin BOOLEAN,
  org_role VARCHAR,
  status TEXT,  -- Cast enum to TEXT
  email_verified BOOLEAN,
  email_verified_at TIMESTAMP,
  last_login_at TIMESTAMP,
  last_login_ip TEXT,  -- Cast INET to TEXT
  failed_login_attempts INTEGER,
  locked_until TIMESTAMP,
  otp_code VARCHAR,
  otp_expires_at TIMESTAMP,
  otp_attempts INTEGER,
  timezone VARCHAR,
  locale VARCHAR,
  metadata JSONB,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  deleted_at TIMESTAMP
) 
SECURITY DEFINER
SET search_path = public
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN QUERY
  SELECT 
    u.id, 
    u.org_id, 
    u.email, 
    u.password_hash, 
    u.first_name, 
    u.last_name, 
    u.full_name,
    u.avatar_url, 
    u.phone, 
    u.is_super_admin, 
    u.org_role,
    u.status::TEXT,  -- Cast enum to TEXT
    u.email_verified, 
    u.email_verified_at,
    u.last_login_at, 
    u.last_login_ip::TEXT,  -- Cast INET to TEXT
    u.failed_login_attempts, 
    u.locked_until,
    u.otp_code,
    u.otp_expires_at,
    u.otp_attempts,
    u.timezone, 
    u.locale, 
    u.metadata, 
    u.created_at, 
    u.updated_at, 
    u.deleted_at
  FROM users u
  WHERE u.email = p_email AND u.deleted_at IS NULL;
END;
$$;

GRANT EXECUTE ON FUNCTION get_user_by_email_for_auth(VARCHAR) TO PUBLIC;

COMMENT ON FUNCTION get_user_by_email_for_auth(VARCHAR) IS 'Bypasses RLS to allow user lookup during authentication when session variables are not set';

-- Function: Create organization with default roles
CREATE OR REPLACE FUNCTION create_organization_with_defaults(
  p_org_name VARCHAR,
  p_org_slug VARCHAR,
  p_admin_email VARCHAR,
  p_admin_password_hash VARCHAR,
  p_admin_first_name VARCHAR DEFAULT NULL,
  p_admin_last_name VARCHAR DEFAULT NULL,
  p_created_by_user_id UUID DEFAULT NULL
) RETURNS TABLE (
  org_id UUID,
  admin_user_id UUID,
  admin_role_id UUID
) AS $$
DECLARE
  v_org_id UUID;
  v_admin_role_id UUID;
  v_user_role_id UUID;
  v_viewer_role_id UUID;
  v_admin_user_id UUID;
BEGIN
  -- Create organization
  INSERT INTO organizations (name, slug, created_by, status)
  VALUES (p_org_name, p_org_slug, p_created_by_user_id, 'active')
  RETURNING id INTO v_org_id;
  
  -- Create default "Org Admin" role
  INSERT INTO roles (org_id, name, type, description, is_default)
  VALUES (v_org_id, 'Org Admin', 'org_defined', 'Full administrative access to organization', false)
  RETURNING id INTO v_admin_role_id;
  
  -- Create default "User" role
  INSERT INTO roles (org_id, name, type, description, is_default)
  VALUES (v_org_id, 'User', 'org_defined', 'Standard user access', true)
  RETURNING id INTO v_user_role_id;
  
  -- Create default "Viewer" role
  INSERT INTO roles (org_id, name, type, description, is_default)
  VALUES (v_org_id, 'Viewer', 'org_defined', 'Read-only access', false)
  RETURNING id INTO v_viewer_role_id;
  
  -- Grant all permissions to Org Admin role
  INSERT INTO role_permissions (role_id, permission_id, granted_by)
  SELECT v_admin_role_id, p.id, p_created_by_user_id
  FROM permissions p;
  
  -- Create admin user
  INSERT INTO users (
    org_id, 
    email, 
    password_hash, 
    first_name,
    last_name,
    is_super_admin, 
    status,
    email_verified
  )
  VALUES (
    v_org_id, 
    p_admin_email, 
    p_admin_password_hash,
    p_admin_first_name,
    p_admin_last_name,
    false, 
    'active',
    true
  )
  RETURNING id INTO v_admin_user_id;
  
  -- Assign admin role to user
  INSERT INTO user_roles (user_id, role_id, assigned_by, assigned_at)
  VALUES (v_admin_user_id, v_admin_role_id, COALESCE(p_created_by_user_id, v_admin_user_id), NOW());
  
  -- Log the creation
  INSERT INTO audit_logs (
    user_id, 
    org_id, 
    action, 
    resource_type, 
    resource_id,
    status,
    metadata
  ) VALUES (
    p_created_by_user_id,
    v_org_id,
    'organization.created',
    'organization',
    v_org_id,
    'success',
    jsonb_build_object(
      'org_name', p_org_name,
      'admin_email', p_admin_email
    )
  );
  
  -- Return values
  RETURN QUERY SELECT v_org_id, v_admin_user_id, v_admin_role_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION create_organization_with_defaults IS 'Creates org with default roles and admin user in single transaction';

-- Function: Check if user has permission
CREATE OR REPLACE FUNCTION user_has_permission(
  p_user_id UUID,
  p_resource VARCHAR,
  p_action VARCHAR
) RETURNS BOOLEAN AS $$
BEGIN
  RETURN EXISTS (
    SELECT 1
    FROM user_roles ur
    JOIN role_permissions rp ON ur.role_id = rp.role_id
    JOIN permissions p ON rp.permission_id = p.id
    WHERE ur.user_id = p_user_id
      AND p.resource = p_resource
      AND p.action = p_action
      AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
  );
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION user_has_permission IS 'Quick permission check for a user';

-- Function: Get user permissions
CREATE OR REPLACE FUNCTION get_user_permissions(p_user_id UUID)
RETURNS TABLE (
  resource VARCHAR,
  action VARCHAR,
  permission_id UUID
) AS $$
BEGIN
  RETURN QUERY
  SELECT DISTINCT 
    p.resource,
    p.action,
    p.id
  FROM user_roles ur
  JOIN role_permissions rp ON ur.role_id = rp.role_id
  JOIN permissions p ON rp.permission_id = p.id
  WHERE ur.user_id = p_user_id
    AND (ur.expires_at IS NULL OR ur.expires_at > NOW());
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION get_user_permissions IS 'Returns all permissions for a user';

-- Function: Get refresh token by hash (bypasses RLS if needed)
CREATE OR REPLACE FUNCTION get_refresh_token_by_hash(p_token_hash VARCHAR)
RETURNS TABLE (
  id UUID,
  user_id UUID,
  token_hash VARCHAR,
  device_info JSONB,
  ip_address TEXT,  -- Cast INET to TEXT
  user_agent TEXT,
  expires_at TIMESTAMP,
  created_at TIMESTAMP,
  last_used_at TIMESTAMP,
  revoked_at TIMESTAMP,
  revoked_by UUID,
  revoked_reason VARCHAR
)
SECURITY DEFINER
SET search_path = public
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN QUERY
  SELECT 
    rt.id,
    rt.user_id,
    rt.token_hash,
    rt.device_info,
    rt.ip_address::TEXT,  -- Cast INET to TEXT
    rt.user_agent,
    rt.expires_at,
    rt.created_at,
    rt.last_used_at,
    rt.revoked_at,
    rt.revoked_by,
    rt.revoked_reason
  FROM refresh_tokens rt
  WHERE rt.token_hash = p_token_hash AND rt.revoked_at IS NULL;
END;
$$;

GRANT EXECUTE ON FUNCTION get_refresh_token_by_hash(VARCHAR) TO PUBLIC;

COMMENT ON FUNCTION get_refresh_token_by_hash(VARCHAR) IS 'Bypasses RLS to allow refresh token lookup during token refresh';

-- Function: Clean expired tokens
CREATE OR REPLACE FUNCTION cleanup_expired_tokens()
RETURNS INTEGER AS $$
DECLARE
  deleted_count INTEGER;
BEGIN
  -- Delete expired refresh tokens
  DELETE FROM refresh_tokens 
  WHERE expires_at < NOW() AND revoked_at IS NULL;
  
  GET DIAGNOSTICS deleted_count = ROW_COUNT;
  
  -- Delete expired password resets
  DELETE FROM password_resets 
  WHERE expires_at < NOW() AND used_at IS NULL;
  
  -- Delete expired email verification tokens
  DELETE FROM email_verification_tokens 
  WHERE expires_at < NOW() AND verified_at IS NULL;
  
  -- Delete expired invitations
  DELETE FROM org_invitations 
  WHERE expires_at < NOW() AND accepted_at IS NULL AND cancelled_at IS NULL;
  
  RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION cleanup_expired_tokens IS 'Cleanup job to remove expired tokens (run daily via cron)';

-- ============================================================================
-- SEED DATA: System Permissions
-- ============================================================================

INSERT INTO permissions (resource, action, description, is_system) VALUES
  -- User Management
  ('users', 'create', 'Create new users', true),
  ('users', 'read', 'View user details', true),
  ('users', 'update', 'Update user information', true),
  ('users', 'delete', 'Delete users', true),
  ('users', 'list', 'List all users', true),
  
  -- Role Management
  ('roles', 'create', 'Create new roles', true),
  ('roles', 'read', 'View role details', true),
  ('roles', 'update', 'Update role information', true),
  ('roles', 'delete', 'Delete roles', true),
  ('roles', 'list', 'List all roles', true),
  ('roles', 'assign', 'Assign roles to users', true),
  
  -- Permission Management
  ('permissions', 'read', 'View permissions', true),
  ('permissions', 'list', 'List all permissions', true),
  ('permissions', 'grant', 'Grant permissions to roles', true),
  ('permissions', 'revoke', 'Revoke permissions from roles', true),
  
  -- Organization Management
  ('organizations', 'create', 'Create new organizations', true),
  ('organizations', 'read', 'View organization details', true),
  ('organizations', 'update', 'Update organization information', true),
  ('organizations', 'delete', 'Delete organizations', true),
  ('organizations', 'list', 'List all organizations', true),
  
  -- Audit Logs
  ('audit_logs', 'read', 'View audit logs', true),
  ('audit_logs', 'list', 'List audit logs', true),
  ('audit_logs', 'export', 'Export audit logs', true),
  
  -- Invitations
  ('invitations', 'create', 'Send organization invitations', true),
  ('invitations', 'read', 'View invitations', true),
  ('invitations', 'delete', 'Cancel invitations', true),
  ('invitations', 'list', 'List all invitations', true),
  
  -- Settings
  ('settings', 'read', 'View organization settings', true),
  ('settings', 'update', 'Update organization settings', true),
  
  -- Billing (future)
  ('billing', 'read', 'View billing information', true),
  ('billing', 'update', 'Update billing information', true),
  
  -- Reports (example resource)
  ('reports', 'create', 'Create reports', true),
  ('reports', 'read', 'View reports', true),
  ('reports', 'update', 'Update reports', true),
  ('reports', 'delete', 'Delete reports', true),
  ('reports', 'list', 'List all reports', true),
  

  -- Upload File
  ('upload_file', 'read', 'View uploaded files', true),
  ('upload_file', 'update', 'Update uploaded files', true),
  ('upload_file', 'create', 'Upload new files', true),
  ('upload_file', 'delete', 'Delete uploaded files', true),
  
  -- View Reports
  ('view_reports', 'read', 'View reports', true),
  ('view_reports', 'update', 'Update reports', true),
  ('view_reports', 'create', 'Create reports', true),
  ('view_reports', 'delete', 'Delete reports', true),
  
  -- Admin Section
  ('admin', 'read', 'View admin section', true),
  ('admin', 'update', 'Update admin settings', true),
  ('admin', 'create', 'Create admin resources', true),
  ('admin', 'delete', 'Delete admin resources', true),
  
  -- Folders Management
  ('folders', 'create', 'Create new folders', true),
  ('folders', 'read', 'View folder details', true),
  ('folders', 'update', 'Update folder information', true),
  ('folders', 'delete', 'Delete folders', true),
  ('folders', 'list', 'List all folders', true),
  
  -- Documents Management (unified files and documents)
  ('documents', 'create', 'Create new documents/files', true),
  ('documents', 'read', 'View document/file details', true),
  ('documents', 'update', 'Update document/file information', true),
  ('documents', 'delete', 'Delete documents/files', true),
  ('documents', 'list', 'List all documents/files', true),
  
  -- Legacy Files permissions (for backward compatibility)
  ('files', 'create', 'Create new files', true),
  ('files', 'read', 'View file details', true),
  ('files', 'update', 'Update file information', true),
  ('files', 'delete', 'Delete files', true),
  ('files', 'list', 'List all files', true)
ON CONFLICT (resource, action) DO NOTHING;

-- ============================================================================
-- SEED DATA: System Roles
-- ============================================================================

-- Create Super Admin role (no org_id = system role)
INSERT INTO roles (name, type, description, org_id)
VALUES ('Super Admin', 'system', 'Platform super administrator with all permissions', NULL)
ON CONFLICT DO NOTHING;

-- Grant all permissions to Super Admin role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'Super Admin' AND r.type = 'system'
ON CONFLICT DO NOTHING;

-- ============================================================================
-- SEED DATA: Super Admin User
-- ============================================================================

-- Insert super admin (password: "superadmin123" - CHANGE THIS IN PRODUCTION!)
-- Super admins have org_role = NULL since they don't belong to an organization
INSERT INTO users (
  email,
  password_hash,
  is_super_admin,
  org_role,
  first_name,
  last_name,
  status,
  email_verified
)
VALUES (
  'superadmin@yourapp.com',
  '$2b$12$DDMSxNUpJzAdosX2o5KcKeiF79st6JM0eR4CT1K2PsWlB678AcCbK',
  true,
  NULL,  -- Super admins don't have org_role
  'Super',
  'Admin',
  'active',
  true
)
ON CONFLICT (email) DO NOTHING;

-- Assign Super Admin role
INSERT INTO user_roles (user_id, role_id, assigned_by)
SELECT u.id, r.id, u.id
FROM users u
CROSS JOIN roles r
WHERE u.email = 'superadmin@yourapp.com'
  AND r.name = 'Super Admin'
  AND r.type = 'system'
ON CONFLICT (user_id, role_id) DO NOTHING;

-- ============================================================================
-- VIEWS: Helpful queries
-- ============================================================================

-- View: User with roles and permissions
CREATE OR REPLACE VIEW vw_user_details AS
SELECT 
  u.id AS user_id,
  u.email,
  u.full_name,
  u.status AS user_status,
  u.is_super_admin,
  u.org_id,
  o.name AS org_name,
  o.status AS org_status,
  json_agg(DISTINCT jsonb_build_object(
    'role_id', r.id,
    'role_name', r.name,
    'role_type', r.type
  )) FILTER (WHERE r.id IS NOT NULL) AS roles,
  json_agg(DISTINCT jsonb_build_object(
    'resource', p.resource,
    'action', p.action
  )) FILTER (WHERE p.id IS NOT NULL) AS permissions
FROM users u
LEFT JOIN organizations o ON u.org_id = o.id
LEFT JOIN user_roles ur ON u.id = ur.user_id
LEFT JOIN roles r ON ur.role_id = r.id
LEFT JOIN role_permissions rp ON r.id = rp.role_id
LEFT JOIN permissions p ON rp.permission_id = p.id
GROUP BY u.id, u.email, u.full_name, u.status, u.is_super_admin, u.org_id, o.name, o.status;

COMMENT ON VIEW vw_user_details IS 'Complete user information with roles and permissions';

-- View: Organization statistics
CREATE OR REPLACE VIEW vw_org_statistics AS
SELECT 
  o.id,
  o.name,
  o.status,
  o.subscription_plan,
  o.subscription_status,
  o.current_users,
  o.max_users,
  COUNT(DISTINCT u.id) FILTER (WHERE u.status = 'active') AS active_users,
  COUNT(DISTINCT r.id) AS total_roles,
  o.created_at
FROM organizations o
LEFT JOIN users u ON o.id = u.org_id
LEFT JOIN roles r ON o.id = r.org_id
GROUP BY o.id;

COMMENT ON VIEW vw_org_statistics IS 'Organization metrics and statistics';

-- ============================================================================
-- INDEXES: Performance optimization
-- ============================================================================

-- Composite indexes for common queries
CREATE INDEX idx_users_org_status ON users(org_id, status);
CREATE INDEX idx_users_org_email ON users(org_id, email);
CREATE INDEX idx_roles_org_type ON roles(org_id, type);

-- Partial indexes for soft deletes
CREATE INDEX idx_orgs_active ON organizations(id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX idx_users_active ON users(id) WHERE status = 'active' AND deleted_at IS NULL;

-- ============================================================================
-- GRANTS: Set appropriate permissions (customize based on your app user)
-- ============================================================================

-- Example: Grant permissions to application role
-- CREATE ROLE app_user WITH LOGIN PASSWORD 'your_secure_password';
-- GRANT USAGE ON SCHEMA public TO app_user;
-- GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
-- GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_user;
-- GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO app_user;

-- ============================================================================
-- MAINTENANCE: Scheduled jobs (setup with pg_cron or external scheduler)
-- ============================================================================

-- Daily cleanup of expired tokens
-- SELECT cron.schedule('cleanup-tokens', '0 2 * * *', 'SELECT cleanup_expired_tokens();');

-- Weekly audit log archival (implement based on retention policy)
-- SELECT cron.schedule('archive-logs', '0 3 * * 0', 'SELECT archive_old_audit_logs();');

-- ============================================================================
-- COMPLETION MESSAGE
-- ============================================================================

DO $$
BEGIN
  RAISE NOTICE '
  ╔════════════════════════════════════════════════════════════════╗
  ║  Database Setup Complete!                                      ║
  ╠════════════════════════════════════════════════════════════════╣
  ║  ✓ Tables created with constraints                            ║
  ║  ✓ Indexes optimized for common queries                       ║
  ║  ✓ Row-Level Security (RLS) policies enabled                  ║
  ║  ✓ Triggers for audit automation                              ║
  ║  ✓ Helper functions for common operations                     ║
  ║  ✓ System permissions and roles seeded                        ║
  ║                                                                ║
  ║  Next Steps:                                                   ║
  ║  1. Create your first super admin user                        ║
  ║  2. Configure RLS session variables in your app               ║
  ║  3. Set up scheduled cleanup jobs                             ║
  ║  4. Review and customize permission grants                    ║
  ╚════════════════════════════════════════════════════════════════╝
  ';
END $$;

-- ============================================================================
-- HELPFUL QUERIES TO GET STARTED
-- ============================================================================

-- Check all tables
-- SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename;

-- Check all permissions
-- SELECT * FROM permissions ORDER BY resource, action;

-- Check system roles
-- SELECT * FROM roles WHERE type = 'system';

-- Test organization creation
-- SELECT * FROM create_organization_with_defaults(
--   'Acme Corp',
--   'acme-corp',
--   'admin@acme.com',
--   '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewY5GyYk3H.eFiyC',
--   'John',
--   'Doe',
--   NULL
-- );