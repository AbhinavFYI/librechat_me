package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"saas-api/internal/database"
	"saas-api/internal/models"
	"saas-api/pkg/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TemplateRepository struct {
	db *database.DB
}

func NewTemplateRepository(db *database.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) Create(ctx context.Context, template *models.Template) error {
	contentJSON, _ := json.Marshal(template.Content)

	query := `
		INSERT INTO templates (
			id, org_id, name, description, framework, is_custom, content, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		template.ID, template.OrgID, template.Name, template.Description,
		template.Framework, template.IsCustom, contentJSON, template.CreatedBy,
	).Scan(&template.CreatedAt, &template.UpdatedAt)

	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to create template", errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *TemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Template, error) {
	template := &models.Template{}
	var contentJSON []byte

	query := `
		SELECT t.id, t.org_id, t.name, t.description, t.framework, t.is_custom, t.content,
			t.created_by, t.created_at, t.updated_at, t.updated_by, t.deleted_at, t.deleted_by,
			u.full_name AS created_by_name, o.name AS org_name
		FROM templates t
		LEFT JOIN users u ON t.created_by = u.id
		LEFT JOIN organizations o ON t.org_id = o.id
		WHERE t.id = $1 AND t.deleted_at IS NULL
	`

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&template.ID, &template.OrgID, &template.Name, &template.Description,
		&template.Framework, &template.IsCustom, &contentJSON,
		&template.CreatedBy, &template.CreatedAt, &template.UpdatedAt,
		&template.UpdatedBy, &template.DeletedAt, &template.DeletedBy,
		&template.CreatedByName, &template.OrgName,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get template", errors.ErrInternalServer.Status)
	}

	if contentJSON != nil {
		json.Unmarshal(contentJSON, &template.Content)
	} else {
		template.Content = make(map[string]interface{})
	}

	return template, nil
}

func (r *TemplateRepository) List(ctx context.Context, orgID *uuid.UUID, page, limit int) ([]*models.Template, int64, error) {
	var templates []*models.Template
	var total int64

	offset := (page - 1) * limit

	// Count query
	countQuery := `SELECT COUNT(*) FROM templates WHERE deleted_at IS NULL`
	countArgs := []interface{}{}

	if orgID != nil {
		countQuery += ` AND org_id = $1`
		countArgs = append(countArgs, *orgID)
	}

	err := r.db.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to count templates", errors.ErrInternalServer.Status)
	}

	// List query
	query := `
		SELECT t.id, t.org_id, t.name, t.description, t.framework, t.is_custom, t.content,
			t.created_by, t.created_at, t.updated_at, t.updated_by, t.deleted_at, t.deleted_by,
			u.full_name AS created_by_name, o.name AS org_name
		FROM templates t
		LEFT JOIN users u ON t.created_by = u.id
		LEFT JOIN organizations o ON t.org_id = o.id
		WHERE t.deleted_at IS NULL
	`
	args := []interface{}{}
	argPos := 1

	if orgID != nil {
		query += fmt.Sprintf(` AND t.org_id = $%d`, argPos)
		args = append(args, *orgID)
		argPos++
	}

	query += fmt.Sprintf(` ORDER BY t.created_at DESC LIMIT $%d OFFSET $%d`, argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to list templates", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	for rows.Next() {
		template := &models.Template{}
		var contentJSON []byte

		err := rows.Scan(
			&template.ID, &template.OrgID, &template.Name, &template.Description,
			&template.Framework, &template.IsCustom, &contentJSON,
			&template.CreatedBy, &template.CreatedAt, &template.UpdatedAt,
			&template.UpdatedBy, &template.DeletedAt, &template.DeletedBy,
			&template.CreatedByName, &template.OrgName,
		)
		if err != nil {
			return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan template", errors.ErrInternalServer.Status)
		}

		if contentJSON != nil {
			json.Unmarshal(contentJSON, &template.Content)
		} else {
			template.Content = make(map[string]interface{})
		}

		templates = append(templates, template)
	}

	return templates, total, nil
}

func (r *TemplateRepository) Update(ctx context.Context, template *models.Template) error {
	contentJSON, _ := json.Marshal(template.Content)

	query := `
		UPDATE templates
		SET name = COALESCE($1, name),
			description = COALESCE($2, description),
			framework = COALESCE($3, framework),
			is_custom = COALESCE($4, is_custom),
			content = COALESCE($5::jsonb, content),
			updated_by = $6,
			updated_at = NOW()
		WHERE id = $7 AND deleted_at IS NULL
		RETURNING updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		template.Name, template.Description, template.Framework,
		template.IsCustom, contentJSON, template.UpdatedBy, template.ID,
	).Scan(&template.UpdatedAt)

	if err == pgx.ErrNoRows {
		return errors.ErrNotFound
	}
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to update template", errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *TemplateRepository) Delete(ctx context.Context, id, deletedBy uuid.UUID) error {
	query := `
		UPDATE templates
		SET deleted_at = NOW(), deleted_by = $1
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING id
	`

	var deletedID uuid.UUID
	err := r.db.Pool.QueryRow(ctx, query, deletedBy, id).Scan(&deletedID)

	if err == pgx.ErrNoRows {
		return errors.ErrNotFound
	}
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to delete template", errors.ErrInternalServer.Status)
	}

	return nil
}
