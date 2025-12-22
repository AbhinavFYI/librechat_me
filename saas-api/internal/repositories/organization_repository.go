package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"saas-api/internal/database"
	"saas-api/internal/models"
	"saas-api/pkg/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OrganizationRepository struct {
	db *database.DB
}

func NewOrganizationRepository(db *database.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

func (r *OrganizationRepository) Create(ctx context.Context, org *models.Organization) error {
	// Check if slug exists in ACTIVE organizations only (deleted orgs free up their slugs)
	var existingOrgID uuid.UUID
	checkQuery := `SELECT id FROM organizations WHERE slug = $1 AND deleted_at IS NULL`
	err := r.db.Pool.QueryRow(ctx, checkQuery, org.Slug).Scan(&existingOrgID)

	if err == nil {
		// Slug exists in an active organization
		return errors.NewError(
			"CONFLICT",
			fmt.Sprintf("An organization with slug '%s' already exists. Please choose a different slug.", org.Slug),
			errors.ErrConflict.Status,
		)
	} else if err.Error() != "no rows in result set" {
		// Some other database error
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to check slug availability", errors.ErrInternalServer.Status)
	}
	// Slug is available (either never used or was used by a deleted org), proceed with creation

	query := `
		INSERT INTO organizations (
			id, name, legal_name, slug, logo_url, website,
			address_line1, address_line2, city, state_province, postal_code, country,
			primary_contact_name, primary_contact_email, primary_contact_phone, billing_email,
			subscription_plan, subscription_status, max_users, max_storage_gb,
			timezone, date_format, locale, settings, status, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26
		)
		RETURNING created_at, updated_at
	`

	settingsJSON, _ := json.Marshal(org.Settings)

	err = r.db.Pool.QueryRow(ctx, query,
		org.ID, org.Name, org.LegalName, org.Slug, org.LogoURL, org.Website,
		org.AddressLine1, org.AddressLine2, org.City, org.StateProvince, org.PostalCode, org.Country,
		org.PrimaryContactName, org.PrimaryContactEmail, org.PrimaryContactPhone, org.BillingEmail,
		org.SubscriptionPlan, org.SubscriptionStatus, org.MaxUsers, org.MaxStorageGB,
		org.Timezone, org.DateFormat, org.Locale, settingsJSON, org.Status, org.CreatedBy,
	).Scan(&org.CreatedAt, &org.UpdatedAt)

	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "duplicate key value violates unique constraint") && strings.Contains(errStr, "organizations_slug_key") {
			return errors.NewError("CONFLICT", fmt.Sprintf("An organization with slug '%s' already exists. Please choose a different slug.", org.Slug), errors.ErrConflict.Status)
		}
		if strings.Contains(errStr, "violates not-null constraint") {
			return errors.WrapError(err, "VALIDATION_ERROR", fmt.Sprintf("Required field missing: %v", err), http.StatusBadRequest)
		}
		return errors.WrapError(err, "INTERNAL_ERROR", fmt.Sprintf("Failed to create organization: %v", err), errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *OrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	org := &models.Organization{}
	var settingsJSON []byte
	var metadataJSON []byte

	query := `
		SELECT id, name, legal_name, slug, logo_url, website,
			address_line1, address_line2, city, state_province, postal_code, country,
			primary_contact_name, primary_contact_email, primary_contact_phone, billing_email,
			subscription_plan, subscription_status, trial_ends_at, subscription_starts_at, subscription_ends_at,
			max_users, max_storage_gb, current_users, current_storage_gb,
			timezone, date_format, locale, settings, status,
			deleted_at, deleted_by, deleted_reason,
			created_at, created_by, updated_at, updated_by, parent_org_id, metadata
		FROM organizations
		WHERE id = $1 AND deleted_at IS NULL
	`

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&org.ID, &org.Name, &org.LegalName, &org.Slug, &org.LogoURL, &org.Website,
		&org.AddressLine1, &org.AddressLine2, &org.City, &org.StateProvince, &org.PostalCode, &org.Country,
		&org.PrimaryContactName, &org.PrimaryContactEmail, &org.PrimaryContactPhone, &org.BillingEmail,
		&org.SubscriptionPlan, &org.SubscriptionStatus, &org.TrialEndsAt,
		&org.SubscriptionStartsAt, &org.SubscriptionEndsAt,
		&org.MaxUsers, &org.MaxStorageGB, &org.CurrentUsers, &org.CurrentStorageGB,
		&org.Timezone, &org.DateFormat, &org.Locale, &settingsJSON, &org.Status,
		&org.DeletedAt, &org.DeletedBy, &org.DeletedReason,
		&org.CreatedAt, &org.CreatedBy, &org.UpdatedAt, &org.UpdatedBy, &org.ParentOrgID, &metadataJSON,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get organization", errors.ErrInternalServer.Status)
	}

	if len(settingsJSON) > 0 {
		json.Unmarshal(settingsJSON, &org.Settings)
	} else {
		org.Settings = make(map[string]interface{})
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &org.Metadata)
	} else {
		org.Metadata = make(map[string]interface{})
	}

	return org, nil
}

func (r *OrganizationRepository) GetBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	org := &models.Organization{}
	var settingsJSON []byte
	var metadataJSON []byte

	query := `
		SELECT id, name, legal_name, slug, logo_url, website,
			address_line1, address_line2, city, state_province, postal_code, country,
			primary_contact_name, primary_contact_email, primary_contact_phone, billing_email,
			subscription_plan, subscription_status, trial_ends_at, subscription_starts_at, subscription_ends_at,
			max_users, max_storage_gb, current_users, current_storage_gb,
			timezone, date_format, locale, settings, status,
			deleted_at, deleted_by, deleted_reason,
			created_at, created_by, updated_at, updated_by, parent_org_id, metadata
		FROM organizations
		WHERE slug = $1 AND deleted_at IS NULL
	`

	err := r.db.Pool.QueryRow(ctx, query, slug).Scan(
		&org.ID, &org.Name, &org.LegalName, &org.Slug, &org.LogoURL, &org.Website,
		&org.AddressLine1, &org.AddressLine2, &org.City, &org.StateProvince, &org.PostalCode, &org.Country,
		&org.PrimaryContactName, &org.PrimaryContactEmail, &org.PrimaryContactPhone, &org.BillingEmail,
		&org.SubscriptionPlan, &org.SubscriptionStatus, &org.TrialEndsAt,
		&org.SubscriptionStartsAt, &org.SubscriptionEndsAt,
		&org.MaxUsers, &org.MaxStorageGB, &org.CurrentUsers, &org.CurrentStorageGB,
		&org.Timezone, &org.DateFormat, &org.Locale, &settingsJSON, &org.Status,
		&org.DeletedAt, &org.DeletedBy, &org.DeletedReason,
		&org.CreatedAt, &org.CreatedBy, &org.UpdatedAt, &org.UpdatedBy, &org.ParentOrgID, &metadataJSON,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get organization", errors.ErrInternalServer.Status)
	}

	if len(settingsJSON) > 0 {
		json.Unmarshal(settingsJSON, &org.Settings)
	} else {
		org.Settings = make(map[string]interface{})
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &org.Metadata)
	} else {
		org.Metadata = make(map[string]interface{})
	}

	return org, nil
}

func (r *OrganizationRepository) Update(ctx context.Context, org *models.Organization) error {
	query := `
		UPDATE organizations
		SET name = COALESCE($1, name),
			legal_name = COALESCE($2, legal_name),
			logo_url = COALESCE($3, logo_url),
			website = COALESCE($4, website),
			address_line1 = COALESCE($5, address_line1),
			address_line2 = COALESCE($6, address_line2),
			city = COALESCE($7, city),
			state_province = COALESCE($8, state_province),
			postal_code = COALESCE($9, postal_code),
			country = COALESCE($10, country),
			primary_contact_name = COALESCE($11, primary_contact_name),
			primary_contact_email = COALESCE($12, primary_contact_email),
			primary_contact_phone = COALESCE($13, primary_contact_phone),
			billing_email = COALESCE($14, billing_email),
			subscription_plan = COALESCE($15, subscription_plan),
			status = COALESCE($16, status),
			settings = COALESCE($17::jsonb, settings),
			updated_at = NOW(),
			updated_by = $18
		WHERE id = $19 AND deleted_at IS NULL
		RETURNING updated_at
	`

	var settingsJSON []byte
	if org.Settings != nil {
		settingsJSON, _ = json.Marshal(org.Settings)
	} else {
		settingsJSON = []byte("{}")
	}

	err := r.db.Pool.QueryRow(ctx, query,
		org.Name, org.LegalName, org.LogoURL, org.Website,
		org.AddressLine1, org.AddressLine2, org.City, org.StateProvince, org.PostalCode, org.Country,
		org.PrimaryContactName, org.PrimaryContactEmail, org.PrimaryContactPhone, org.BillingEmail,
		org.SubscriptionPlan, org.Status, settingsJSON, org.UpdatedBy, org.ID,
	).Scan(&org.UpdatedAt)

	if err == pgx.ErrNoRows {
		return errors.ErrNotFound
	}
	if err != nil {
		// Log detailed error information
		fmt.Printf("Organization Update Error - ID: %s, Error: %v\n", org.ID, err)
		fmt.Printf("Update values - Name: %s, LogoURL: %v, Status: %s\n", org.Name, org.LogoURL, org.Status)
		return errors.WrapError(err, "INTERNAL_ERROR", fmt.Sprintf("Failed to update organization: %v", err), errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *OrganizationRepository) List(ctx context.Context, page, limit int) ([]*models.Organization, int64, error) {
	var orgs []*models.Organization
	var total int64

	offset := (page - 1) * limit

	countQuery := `SELECT COUNT(*) FROM organizations WHERE deleted_at IS NULL`
	err := r.db.Pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to count organizations", errors.ErrInternalServer.Status)
	}

	query := `
		SELECT id, name, slug, logo_url, subscription_plan, subscription_status,
			current_users, max_users, status, created_at
		FROM organizations
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to list organizations", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	for rows.Next() {
		org := &models.Organization{}
		err := rows.Scan(
			&org.ID, &org.Name, &org.Slug, &org.LogoURL, &org.SubscriptionPlan,
			&org.SubscriptionStatus, &org.CurrentUsers, &org.MaxUsers,
			&org.Status, &org.CreatedAt,
		)
		if err != nil {
			return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan organization", errors.ErrInternalServer.Status)
		}
		orgs = append(orgs, org)
	}

	return orgs, total, nil
}

func (r *OrganizationRepository) Delete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID, reason string) error {
	query := `
		UPDATE organizations
		SET deleted_at = NOW(), deleted_by = $1, deleted_reason = $2, status = 'deleted'
		WHERE id = $3 AND deleted_at IS NULL
	`

	result, err := r.db.Pool.Exec(ctx, query, deletedBy, reason, id)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to delete organization", errors.ErrInternalServer.Status)
	}

	if result.RowsAffected() == 0 {
		return errors.ErrNotFound
	}

	return nil
}
