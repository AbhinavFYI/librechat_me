package models

import (
	"time"

	"github.com/google/uuid"
)

// Organization models
type Organization struct {
	ID                   uuid.UUID              `json:"id"`
	Name                 string                 `json:"name"`
	LegalName            *string                `json:"legal_name,omitempty"`
	Slug                 string                 `json:"slug"`
	LogoURL              *string                `json:"logo_url,omitempty"`
	Website              *string                `json:"website,omitempty"`
	AddressLine1         *string                `json:"address_line1,omitempty"`
	AddressLine2         *string                `json:"address_line2,omitempty"`
	City                 *string                `json:"city,omitempty"`
	StateProvince        *string                `json:"state_province,omitempty"`
	PostalCode           *string                `json:"postal_code,omitempty"`
	Country              *string                `json:"country,omitempty"`
	PrimaryContactName   *string                `json:"primary_contact_name,omitempty"`
	PrimaryContactEmail  *string                `json:"primary_contact_email,omitempty"`
	PrimaryContactPhone  *string                `json:"primary_contact_phone,omitempty"`
	BillingEmail         *string                `json:"billing_email,omitempty"`
	SubscriptionPlan     string                 `json:"subscription_plan"`
	SubscriptionStatus   string                 `json:"subscription_status"`
	TrialEndsAt          *time.Time             `json:"trial_ends_at,omitempty"`
	SubscriptionStartsAt *time.Time             `json:"subscription_starts_at,omitempty"`
	SubscriptionEndsAt   *time.Time             `json:"subscription_ends_at,omitempty"`
	MaxUsers             int                    `json:"max_users"`
	MaxStorageGB         int                    `json:"max_storage_gb"`
	CurrentUsers         int                    `json:"current_users"`
	CurrentStorageGB     float64                `json:"current_storage_gb"`
	Timezone             string                 `json:"timezone"`
	DateFormat           string                 `json:"date_format"`
	Locale               string                 `json:"locale"`
	Settings             map[string]interface{} `json:"settings"`
	Status               string                 `json:"status"`
	DeletedAt            *time.Time             `json:"deleted_at,omitempty"`
	DeletedBy            *uuid.UUID             `json:"deleted_by,omitempty"`
	DeletedReason        *string                `json:"deleted_reason,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	CreatedBy            *uuid.UUID             `json:"created_by,omitempty"`
	UpdatedAt            time.Time              `json:"updated_at"`
	UpdatedBy            *uuid.UUID             `json:"updated_by,omitempty"`
	ParentOrgID          *uuid.UUID             `json:"parent_org_id,omitempty"`
	Metadata             map[string]interface{} `json:"metadata"`
}

type CreateOrganizationRequest struct {
	Name      string  `json:"name" binding:"required"`
	LegalName *string `json:"legal_name"`
	// Slug is auto-generated internally from name, not exposed in API
	LogoURL             *string                `json:"logo_url"`
	Website             *string                `json:"website"`
	AddressLine1        *string                `json:"address_line1"`
	City                *string                `json:"city"`
	StateProvince       *string                `json:"state_province"`
	PostalCode          *string                `json:"postal_code"`
	Country             *string                `json:"country"`
	PrimaryContactName  *string                `json:"primary_contact_name"`
	PrimaryContactEmail *string                `json:"primary_contact_email"`
	PrimaryContactPhone *string                `json:"primary_contact_phone"`
	BillingEmail        *string                `json:"billing_email"`
	SubscriptionPlan    *string                `json:"subscription_plan"`
	Settings            map[string]interface{} `json:"settings"`
}

type UpdateOrganizationRequest struct {
	Name                *string                `json:"name"`
	LegalName           *string                `json:"legal_name"`
	LogoURL             *string                `json:"logo_url"`
	Website             *string                `json:"website"`
	AddressLine1        *string                `json:"address_line1"`
	City                *string                `json:"city"`
	StateProvince       *string                `json:"state_province"`
	PostalCode          *string                `json:"postal_code"`
	Country             *string                `json:"country"`
	PrimaryContactName  *string                `json:"primary_contact_name"`
	PrimaryContactEmail *string                `json:"primary_contact_email"`
	PrimaryContactPhone *string                `json:"primary_contact_phone"`
	BillingEmail        *string                `json:"billing_email"`
	SubscriptionPlan    *string                `json:"subscription_plan"`
	Status              *string                `json:"status"`
	Settings            map[string]interface{} `json:"settings"`
}

// User models
type User struct {
	ID                  uuid.UUID              `json:"id"`
	OrgID               *uuid.UUID             `json:"org_id,omitempty"`
	Email               string                 `json:"email"`
	PasswordHash        string                 `json:"-"` // Never return in JSON
	FirstName           *string                `json:"first_name,omitempty"`
	LastName            *string                `json:"last_name,omitempty"`
	FullName            string                 `json:"full_name"`
	AvatarURL           *string                `json:"avatar_url,omitempty"`
	Phone               *string                `json:"phone,omitempty"`
	IsSuperAdmin        bool                   `json:"is_super_admin"`
	OrgRole             *string                `json:"org_role,omitempty"` // OrgRole: "admin", "user", "viewer" - determines login eligibility
	Status              string                 `json:"status"`
	EmailVerified       bool                   `json:"email_verified"`
	EmailVerifiedAt     *time.Time             `json:"email_verified_at,omitempty"`
	LastLoginAt         *time.Time             `json:"last_login_at,omitempty"`
	LastLoginIP         *string                `json:"last_login_ip,omitempty"`
	FailedLoginAttempts int                    `json:"failed_login_attempts"`
	LockedUntil         *time.Time             `json:"locked_until,omitempty"`
	OTPCode             *string                `json:"-"` // Never return in JSON
	OTPExpiresAt        *time.Time             `json:"-"`
	OTPAttempts         int                    `json:"-"`
	Timezone            string                 `json:"timezone"`
	Locale              string                 `json:"locale"`
	Metadata            map[string]interface{} `json:"metadata"`
	Roles               []Role                 `json:"roles,omitempty"` // User's roles (fetched separately) - determines permissions
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	DeletedAt           *time.Time             `json:"deleted_at,omitempty"`
}

type CreateUserRequest struct {
	Email     string     `json:"email" binding:"required,email"`
	Password  string     `json:"password" binding:"required,min=8"`
	FirstName *string    `json:"first_name"`
	LastName  *string    `json:"last_name"`
	Phone     *string    `json:"phone"`
	OrgID     *uuid.UUID `json:"org_id,omitempty"`    // Optional: specify org by UUID (Super Admin only)
	OrgRole   *string    `json:"org_role,omitempty"`  // Optional: OrgRole - "admin", "user", "viewer" (default: "user")
	RoleID    *uuid.UUID `json:"role_id,omitempty"`   // Optional: assign role by UUID during creation (determines permissions)
	RoleName  *string    `json:"role_name,omitempty"` // Optional: assign role by name (determines permissions)
}

type UpdateUserRequest struct {
	FirstName     *string `json:"first_name"`
	LastName      *string `json:"last_name"`
	Phone         *string `json:"phone"`
	AvatarURL     *string `json:"avatar_url"`
	Status        *string `json:"status"`
	OrgRole       *string `json:"org_role,omitempty"`       // Optional: Update OrgRole - "admin", "user", "viewer"
	EmailVerified *bool   `json:"email_verified,omitempty"` // Optional: Update email verification status
	Timezone      *string `json:"timezone"`
	Locale        *string `json:"locale"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int          `json:"expires_in"`
	User         *User        `json:"user"`
	Permissions  []Permission `json:"permissions,omitempty"` // User's permissions for frontend
}

// Role models
type Role struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       *uuid.UUID `json:"org_id,omitempty"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Description *string    `json:"description,omitempty"`
	IsDefault   bool       `json:"is_default"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
}

type CreateRoleRequest struct {
	Name          string      `json:"name" binding:"required"`
	Description   *string     `json:"description"`
	IsDefault     *bool       `json:"is_default"`
	OrgID         *uuid.UUID  `json:"org_id,omitempty"` // Optional: Super admin can specify org_id
	PermissionIDs []uuid.UUID `json:"permission_ids,omitempty"`
}

type UpdateRoleRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *bool   `json:"is_default"`
}

// Permission models
type Permission struct {
	ID          uuid.UUID `json:"id"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	Description *string   `json:"description,omitempty"`
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserRole models
type UserRole struct {
	UserID     uuid.UUID  `json:"user_id"`
	RoleID     uuid.UUID  `json:"role_id"`
	AssignedBy uuid.UUID  `json:"assigned_by"`
	AssignedAt time.Time  `json:"assigned_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type AssignRoleRequest struct {
	RoleID    uuid.UUID  `json:"role_id" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// RefreshToken models
type RefreshToken struct {
	ID            uuid.UUID              `json:"id"`
	UserID        uuid.UUID              `json:"user_id"`
	TokenHash     string                 `json:"-"`
	DeviceInfo    map[string]interface{} `json:"device_info"`
	IPAddress     *string                `json:"ip_address,omitempty"`
	UserAgent     *string                `json:"user_agent,omitempty"`
	ExpiresAt     time.Time              `json:"expires_at"`
	CreatedAt     time.Time              `json:"created_at"`
	LastUsedAt    time.Time              `json:"last_used_at"`
	RevokedAt     *time.Time             `json:"revoked_at,omitempty"`
	RevokedBy     *uuid.UUID             `json:"revoked_by,omitempty"`
	RevokedReason *string                `json:"revoked_reason,omitempty"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"` // Optional - can also come from cookie
}

// OTP models
type SendOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type SendOTPResponse struct {
	Message string `json:"message"`
	Email   string `json:"email"`
}

type VerifyOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required,len=6"`
}

type ResendOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// AuditLog models
type AuditLog struct {
	ID           int64                  `json:"id"`
	UserID       *uuid.UUID             `json:"user_id,omitempty"`
	OrgID        *uuid.UUID             `json:"org_id,omitempty"`
	IPAddress    *string                `json:"ip_address,omitempty"`
	UserAgent    *string                `json:"user_agent,omitempty"`
	Action       string                 `json:"action"`
	ResourceType *string                `json:"resource_type,omitempty"`
	ResourceID   *uuid.UUID             `json:"resource_id,omitempty"`
	Status       string                 `json:"status"`
	Metadata     map[string]interface{} `json:"metadata"`
	ErrorMessage *string                `json:"error_message,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// OrgInvitation models
type OrgInvitation struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	Email       string     `json:"email"`
	RoleID      uuid.UUID  `json:"role_id"`
	TokenHash   string     `json:"-"`
	InvitedBy   uuid.UUID  `json:"invited_by"`
	InvitedAt   time.Time  `json:"invited_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
	AcceptedBy  *uuid.UUID `json:"accepted_by,omitempty"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
	CancelledBy *uuid.UUID `json:"cancelled_by,omitempty"`
}

type CreateInvitationRequest struct {
	Email     string     `json:"email" binding:"required,email"`
	RoleID    uuid.UUID  `json:"role_id" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type AcceptInvitationRequest struct {
	Token     string  `json:"token" binding:"required"`
	Password  string  `json:"password" binding:"required,min=8"`
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
}

// Common response models
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

type PaginationParams struct {
	Page  int `form:"page" binding:"min=1"`
	Limit int `form:"limit" binding:"min=1,max=100"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
}

// Template models
type Template struct {
	ID          uuid.UUID              `json:"id"`
	OrgID       *uuid.UUID             `json:"org_id,omitempty"`
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Framework   *string                `json:"framework,omitempty"` // 'R-T-F', 'T-A-G', 'B-A-B', 'C-A-R-E', 'R-I-S-E', or 'custom'
	IsCustom    bool                   `json:"is_custom"`
	Content     map[string]interface{} `json:"content"`
	CreatedBy   *uuid.UUID             `json:"created_by,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	UpdatedBy   *uuid.UUID             `json:"updated_by,omitempty"`
	DeletedAt   *time.Time             `json:"deleted_at,omitempty"`
	DeletedBy   *uuid.UUID             `json:"deleted_by,omitempty"`
	// Joined fields for display
	CreatedByName *string `json:"created_by_name,omitempty"`
	OrgName       *string `json:"org_name,omitempty"`
}

type CreateTemplateRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description *string                `json:"description"`
	Framework   *string                `json:"framework"` // 'R-T-F', 'T-A-G', 'B-A-B', 'C-A-R-E', 'R-I-S-E', or 'custom'
	IsCustom    *bool                  `json:"is_custom"`
	Content     map[string]interface{} `json:"content"`
}

type UpdateTemplateRequest struct {
	Name        *string                `json:"name"`
	Description *string                `json:"description"`
	Framework   *string                `json:"framework"`
	IsCustom    *bool                  `json:"is_custom"`
	Content     map[string]interface{} `json:"content"`
}

// Persona models
type Persona struct {
	ID               uuid.UUID              `json:"id"`
	OrgID            *uuid.UUID             `json:"org_id,omitempty"`
	TemplateID       *uuid.UUID             `json:"template_id,omitempty"`
	Name             string                 `json:"name"`
	Description      *string                `json:"description,omitempty"`
	Content          map[string]interface{} `json:"content"`
	IsCustomTemplate bool                   `json:"is_custom_template"`
	CreatedBy        *uuid.UUID             `json:"created_by,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	UpdatedBy        *uuid.UUID             `json:"updated_by,omitempty"`
	DeletedAt        *time.Time             `json:"deleted_at,omitempty"`
	DeletedBy        *uuid.UUID             `json:"deleted_by,omitempty"`
	// Joined fields for display
	CreatedByName *string `json:"created_by_name,omitempty"`
	TemplateName  *string `json:"template_name,omitempty"`
	OrgName       *string `json:"org_name,omitempty"`
}

type CreatePersonaRequest struct {
	TemplateID       *uuid.UUID             `json:"template_id,omitempty"`
	Name             string                 `json:"name" binding:"required"`
	Description      *string                `json:"description"`
	Content          map[string]interface{} `json:"content"`
	IsCustomTemplate *bool                  `json:"is_custom_template"`
}

type UpdatePersonaRequest struct {
	Name             *string                `json:"name"`
	Description      *string                `json:"description"`
	TemplateID       *uuid.UUID             `json:"template_id,omitempty"`
	Content          map[string]interface{} `json:"content"`
	IsCustomTemplate *bool                  `json:"is_custom_template"`
}

// Folder models
type Folder struct {
	ID        uuid.UUID  `json:"id"`
	OrgID     uuid.UUID  `json:"org_id"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	Name      string     `json:"name"`
	Path      string     `json:"path"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	UpdatedBy *uuid.UUID `json:"updated_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	// Joined fields
	CreatedByName *string            `json:"created_by_name,omitempty"`
	UpdatedByName *string            `json:"updated_by_name,omitempty"`
	Children      []*Folder          `json:"children,omitempty"` // For tree structure
	Files         []*File            `json:"files,omitempty"`    // Files in this folder
	Permissions   []FolderPermission `json:"permissions,omitempty"`
}

type CreateFolderRequest struct {
	Name     string     `json:"name" binding:"required"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	OrgID    *uuid.UUID `json:"org_id,omitempty"` // Optional: for super admins to specify org
}

type UpdateFolderRequest struct {
	Name     *string    `json:"name"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

type FolderPermission struct {
	ID         uuid.UUID `json:"id"`
	FolderID   uuid.UUID `json:"folder_id"`
	RoleID     uuid.UUID `json:"role_id"`
	Permission string    `json:"permission"` // read, write, delete, move, share
	CreatedAt  time.Time `json:"created_at"`
	// Joined fields
	RoleName *string `json:"role_name,omitempty"`
}

type AssignFolderPermissionRequest struct {
	RoleID     uuid.UUID `json:"role_id" binding:"required"`
	Permission string    `json:"permission" binding:"required,oneof=read write delete move share"`
}

// File models
type File struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"org_id"`
	FolderID   *uuid.UUID `json:"folder_id,omitempty"`
	Name       string     `json:"name"`
	Extension  *string    `json:"extension,omitempty"`
	MimeType   *string    `json:"mime_type,omitempty"`
	SizeBytes  *int64     `json:"size_bytes,omitempty"`
	Checksum   *string    `json:"checksum,omitempty"`
	StorageKey string     `json:"storage_key"`
	Version    int        `json:"version"`
	CreatedBy  *uuid.UUID `json:"created_by,omitempty"`
	UpdatedBy  *uuid.UUID `json:"updated_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	// Joined fields
	CreatedByName *string `json:"created_by_name,omitempty"`
	UpdatedByName *string `json:"updated_by_name,omitempty"`
	FolderName    *string `json:"folder_name,omitempty"`
}

type CreateFileRequest struct {
	Name     string     `json:"name" binding:"required"`
	FolderID *uuid.UUID `json:"folder_id,omitempty"`
	OrgID    *uuid.UUID `json:"org_id,omitempty"` // Optional: for super admins to specify org
	// File upload will be handled separately via multipart/form-data
}

type UpdateFileRequest struct {
	Name     *string    `json:"name"`
	FolderID *uuid.UUID `json:"folder_id,omitempty"`
}

type FileUploadResponse struct {
	File      *File   `json:"file"`
	UploadURL *string `json:"upload_url,omitempty"` // For presigned URLs if using S3
	Message   string  `json:"message"`
}

// Screener (Settings) models
type Screener struct {
	ID           uuid.UUID  `json:"id"`
	OrgID        *uuid.UUID `json:"org_id,omitempty"` // Nullable for superadmins
	UserID       uuid.UUID  `json:"user_id"`
	ScreenerName string     `json:"screener_name"`
	TableName    *string    `json:"table_name,omitempty"`
	Query        string     `json:"query"`
	UniverseList *string    `json:"universe_list,omitempty"`
	Explainer    *string    `json:"explainer,omitempty"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type CreateScreenerRequest struct {
	ScreenerName string  `json:"screener_name" binding:"required"`
	TableName    *string `json:"tableName,omitempty"`
	Query        string  `json:"query" binding:"required"`
	UniverseList *string `json:"universeList,omitempty"`
	Explainer    *string `json:"explainer,omitempty"`
}

type SaveScreenerResponse struct {
	Status string    `json:"status"`
	Data   *Screener `json:"data"`
}

type ListScreenersResponse struct {
	Status string      `json:"status"`
	Data   []*Screener `json:"data"`
}
