package repositories

import (
	"context"
	"fmt"
	"log"
	"saas-api/internal/database"
	"saas-api/internal/models"
	"saas-api/pkg/errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserRepository struct {
	db *database.DB
}

func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (
			id, org_id, email, password_hash, first_name, last_name,
			phone, is_super_admin, org_role, status, email_verified, timezone, locale
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		user.ID, user.OrgID, user.Email, user.PasswordHash,
		user.FirstName, user.LastName, user.Phone, user.IsSuperAdmin,
		user.OrgRole, user.Status, user.EmailVerified, user.Timezone, user.Locale,
	).Scan(&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err.Error() == "pq: duplicate key value violates unique constraint \"users_email_key\"" {
			return errors.WrapError(err, "CONFLICT", "Email already exists", errors.ErrConflict.Status)
		}
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to create user", errors.ErrInternalServer.Status)
	}

	return nil
}

// SetOTP sets OTP code and expiry for a user
func (r *UserRepository) SetOTP(ctx context.Context, userID uuid.UUID, otpCode string, expiresAt time.Time) error {
	query := `
		UPDATE users 
		SET otp_code = $1, otp_expires_at = $2, otp_attempts = 0, updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL
	`
	log.Printf("SetOTP: Updating OTP for userID=%s, otpCode=%s, expiresAt=%v", userID, otpCode, expiresAt)
	result, err := r.db.Pool.Exec(ctx, query, otpCode, expiresAt, userID)
	if err != nil {
		log.Printf("SetOTP: Database error: %v", err)
		log.Printf("SetOTP: Query: %s", query)
		log.Printf("SetOTP: Parameters: otpCode=%s, expiresAt=%v, userID=%s", otpCode, expiresAt, userID)
		return errors.WrapError(err, "INTERNAL_ERROR", fmt.Sprintf("Failed to set OTP: %v", err), errors.ErrInternalServer.Status)
	}
	rowsAffected := result.RowsAffected()
	log.Printf("SetOTP: OTP updated successfully, rows affected: %d", rowsAffected)
	if rowsAffected == 0 {
		log.Printf("SetOTP: Warning: No rows updated. User might not exist or is deleted.")
		return errors.NewError("USER_NOT_FOUND", "User not found or deleted", 404)
	}
	return nil
}

// VerifyOTP verifies OTP and increments attempts if invalid
func (r *UserRepository) VerifyOTP(ctx context.Context, email, otpCode string) error {
	// First, get OTP data
	query := `
		SELECT otp_code, otp_expires_at, otp_attempts
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`
	var storedOTP *string
	var expiresAt *time.Time
	var attempts int

	err := r.db.Pool.QueryRow(ctx, query, email).Scan(&storedOTP, &expiresAt, &attempts)
	if err == pgx.ErrNoRows {
		return errors.ErrNotFound
	}
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to verify OTP", errors.ErrInternalServer.Status)
	}

	// Check if OTP exists
	if storedOTP == nil {
		return errors.NewError("OTP_EXPIRED", "OTP not found or expired", 401)
	}

	// Check if expired
	if expiresAt == nil || expiresAt.Before(time.Now()) {
		// Clear expired OTP
		r.ClearOTP(ctx, email)
		return errors.NewError("OTP_EXPIRED", "OTP expired", 401)
	}

	// Check attempts (max 5 attempts)
	if attempts >= 5 {
		r.ClearOTP(ctx, email)
		return errors.NewError("OTP_EXPIRED", "OTP expired - too many failed attempts", 401)
	}

	// Verify OTP
	if *storedOTP != otpCode {
		// Increment attempts on invalid OTP
		updateQuery := `
			UPDATE users 
			SET otp_attempts = otp_attempts + 1, updated_at = NOW()
			WHERE email = $1 AND deleted_at IS NULL
		`
		r.db.Pool.Exec(ctx, updateQuery, email)
		return errors.NewError("INVALID_OTP", "Invalid OTP", 401)
	}

	// OTP verified - clear it
	r.ClearOTP(ctx, email)
	return nil
}

// ClearOTP clears OTP from user record
func (r *UserRepository) ClearOTP(ctx context.Context, email string) error {
	query := `
		UPDATE users 
		SET otp_code = NULL, otp_expires_at = NULL, otp_attempts = 0, updated_at = NOW()
		WHERE email = $1 AND deleted_at IS NULL
	`
	_, err := r.db.Pool.Exec(ctx, query, email)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to clear OTP", errors.ErrInternalServer.Status)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, org_id, email, password_hash, first_name, last_name, full_name,
			avatar_url, phone, is_super_admin, org_role, status, email_verified, email_verified_at,
			last_login_at, last_login_ip::TEXT, failed_login_attempts, locked_until,
			timezone, locale, metadata, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.OrgID, &user.Email, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.FullName, &user.AvatarURL,
		&user.Phone, &user.IsSuperAdmin, &user.OrgRole, &user.Status, &user.EmailVerified,
		&user.EmailVerifiedAt, &user.LastLoginAt, &user.LastLoginIP,
		&user.FailedLoginAttempts, &user.LockedUntil, &user.Timezone,
		&user.Locale, &user.Metadata, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get user", errors.ErrInternalServer.Status)
	}

	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	// Use SECURITY DEFINER function to bypass RLS for authentication
	query := `
		SELECT id, org_id, email, password_hash, first_name, last_name, full_name,
			avatar_url, phone, is_super_admin, org_role, status, email_verified, email_verified_at,
			last_login_at, last_login_ip::TEXT, failed_login_attempts, locked_until,
			timezone, locale, metadata, created_at, updated_at, deleted_at
		FROM get_user_by_email_for_auth($1)
	`

	err := r.db.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.OrgID, &user.Email, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.FullName, &user.AvatarURL,
		&user.Phone, &user.IsSuperAdmin, &user.OrgRole, &user.Status, &user.EmailVerified,
		&user.EmailVerifiedAt, &user.LastLoginAt, &user.LastLoginIP,
		&user.FailedLoginAttempts, &user.LockedUntil, &user.Timezone,
		&user.Locale, &user.Metadata, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get user", errors.ErrInternalServer.Status)
	}

	return user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users
		SET first_name = COALESCE($1, first_name),
			last_name = COALESCE($2, last_name),
			phone = COALESCE($3, phone),
			avatar_url = COALESCE($4, avatar_url),
			status = COALESCE($5, status),
			org_role = COALESCE($6, org_role),
			email_verified = COALESCE($7, email_verified),
			email_verified_at = COALESCE($8, email_verified_at),
			timezone = COALESCE($9, timezone),
			locale = COALESCE($10, locale),
			updated_at = NOW()
		WHERE id = $11 AND deleted_at IS NULL
		RETURNING updated_at, email_verified_at
	`

	var emailVerifiedAt *time.Time
	err := r.db.Pool.QueryRow(ctx, query,
		user.FirstName, user.LastName, user.Phone, user.AvatarURL,
		user.Status, user.OrgRole, user.EmailVerified,
		user.EmailVerifiedAt, user.Timezone, user.Locale, user.ID,
	).Scan(&user.UpdatedAt, &emailVerifiedAt)

	if err == pgx.ErrNoRows {
		return errors.ErrNotFound
	}
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to update user", errors.ErrInternalServer.Status)
	}

	// Update the EmailVerifiedAt field in the user object
	user.EmailVerifiedAt = emailVerifiedAt

	return nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`

	result, err := r.db.Pool.Exec(ctx, query, passwordHash, userID)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to update password", errors.ErrInternalServer.Status)
	}

	if result.RowsAffected() == 0 {
		return errors.ErrNotFound
	}

	return nil
}

func (r *UserRepository) UpdateLoginInfo(ctx context.Context, userID uuid.UUID, ipAddress string) error {
	query := `
		UPDATE users
		SET last_login_at = NOW(), last_login_ip = $1, failed_login_attempts = 0, locked_until = NULL
		WHERE id = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, ipAddress, userID)
	return err
}

func (r *UserRepository) IncrementFailedLoginAttempts(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1,
			locked_until = CASE
				WHEN failed_login_attempts + 1 >= 5 THEN NOW() + INTERVAL '30 minutes'
				ELSE locked_until
			END
		WHERE id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query, userID)
	return err
}

func (r *UserRepository) List(ctx context.Context, orgID *uuid.UUID, page, limit int) ([]*models.User, int64, error) {
	var users []*models.User
	var total int64

	offset := (page - 1) * limit

	// Count query
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	countArgs := []interface{}{}

	if orgID != nil {
		countQuery += ` AND org_id = $1`
		countArgs = append(countArgs, *orgID)
	}

	err := r.db.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to count users", errors.ErrInternalServer.Status)
	}

	// List query
	query := `
		SELECT id, org_id, email, first_name, last_name, full_name,
			avatar_url, phone, is_super_admin, org_role, status, email_verified,
			last_login_at, timezone, locale, created_at, updated_at
		FROM users
		WHERE deleted_at IS NULL
	`
	args := []interface{}{}
	argPos := 1

	if orgID != nil {
		query += fmt.Sprintf(` AND org_id = $%d`, argPos)
		args = append(args, *orgID)
		argPos++
	}

	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to list users", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID, &user.OrgID, &user.Email, &user.FirstName, &user.LastName,
			&user.FullName, &user.AvatarURL, &user.Phone, &user.IsSuperAdmin,
			&user.OrgRole, &user.Status, &user.EmailVerified, &user.LastLoginAt,
			&user.Timezone, &user.Locale, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan user", errors.ErrInternalServer.Status)
		}

		// Fetch roles for this user
		roles, err := r.GetUserRoles(ctx, user.ID)
		if err == nil {
			// Convert []*models.Role to []models.Role for JSON serialization
			user.Roles = make([]models.Role, len(roles))
			for i, role := range roles {
				user.Roles[i] = *role
			}
		}

		users = append(users, user)
	}

	return users, total, nil
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to delete user", errors.ErrInternalServer.Status)
	}

	if result.RowsAffected() == 0 {
		return errors.ErrNotFound
	}

	return nil
}

func (r *UserRepository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*models.Role, error) {
	query := `
		SELECT r.id, r.org_id, r.name, r.type, r.description, r.is_default,
			r.created_at, r.updated_at, r.created_by
		FROM roles r
		INNER JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
		ORDER BY r.created_at
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get user roles", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	var roles []*models.Role
	for rows.Next() {
		role := &models.Role{}
		err := rows.Scan(
			&role.ID, &role.OrgID, &role.Name, &role.Type, &role.Description,
			&role.IsDefault, &role.CreatedAt, &role.UpdatedAt, &role.CreatedBy,
		)
		if err != nil {
			return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan role", errors.ErrInternalServer.Status)
		}
		roles = append(roles, role)
	}

	return roles, nil
}

func (r *UserRepository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]*models.Permission, error) {
	query := `
		SELECT DISTINCT p.id, p.resource, p.action, p.description, p.is_system, p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		INNER JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.user_id = $1 AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
		ORDER BY p.resource, p.action
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return []*models.Permission{}, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get user permissions", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	permissions := make([]*models.Permission, 0) // Initialize as empty slice, not nil
	for rows.Next() {
		perm := &models.Permission{}
		err := rows.Scan(
			&perm.ID, &perm.Resource, &perm.Action, &perm.Description,
			&perm.IsSystem, &perm.CreatedAt,
		)
		if err != nil {
			return []*models.Permission{}, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan permission", errors.ErrInternalServer.Status)
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// HasPermission checks if a user has a specific permission
// It also checks permission dependencies:
// - "create" requires "read", "update", and "create"
// - "delete" requires "read" and "delete"
// - "update" requires "read" and "update"
// - "read" is standalone
func (r *UserRepository) HasPermission(ctx context.Context, userID uuid.UUID, resource, action string) (bool, error) {
	// Get all user permissions for this resource
	permissions, err := r.GetUserPermissions(ctx, userID)
	if err != nil {
		return false, err
	}

	// Build a map of user's permissions for this resource
	resourcePerms := make(map[string]bool)
	for _, perm := range permissions {
		if perm.Resource == resource {
			resourcePerms[perm.Action] = true
		}
	}

	// Check if user has the required action
	if resourcePerms[action] {
		return true, nil
	}

	// Check dependencies based on action type
	switch action {
	case "read":
		// Read is standalone, no dependencies
		return false, nil
	case "update":
		// Update requires read + update
		if resourcePerms["read"] && resourcePerms["update"] {
			return true, nil
		}
		return false, nil
	case "create":
		// Create requires read + update + create
		if resourcePerms["read"] && resourcePerms["update"] && resourcePerms["create"] {
			return true, nil
		}
		return false, nil
	case "delete":
		// Delete requires read + delete
		if resourcePerms["read"] && resourcePerms["delete"] {
			return true, nil
		}
		return false, nil
	default:
		// For other actions, just check if user has it
		return resourcePerms[action], nil
	}
}
