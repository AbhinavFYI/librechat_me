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

type PersonaRepository struct {
	db *database.DB
}

func NewPersonaRepository(db *database.DB) *PersonaRepository {
	return &PersonaRepository{db: db}
}

func (r *PersonaRepository) Create(ctx context.Context, persona *models.Persona) error {
	contentJSON, _ := json.Marshal(persona.Content)

	query := `
		INSERT INTO personas (
			id, org_id, template_id, name, description, content, is_custom_template, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		persona.ID, persona.OrgID, persona.TemplateID, persona.Name,
		persona.Description, contentJSON, persona.IsCustomTemplate, persona.CreatedBy,
	).Scan(&persona.CreatedAt, &persona.UpdatedAt)

	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to create persona", errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *PersonaRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Persona, error) {
	persona := &models.Persona{}
	var contentJSON []byte

	query := `
		SELECT p.id, p.org_id, p.template_id, p.name, p.description, p.content, p.is_custom_template,
			p.created_by, p.created_at, p.updated_at, p.updated_by, p.deleted_at, p.deleted_by,
			u.full_name AS created_by_name, t.name AS template_name, o.name AS org_name
		FROM personas p
		LEFT JOIN users u ON p.created_by = u.id
		LEFT JOIN templates t ON p.template_id = t.id
		LEFT JOIN organizations o ON p.org_id = o.id
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&persona.ID, &persona.OrgID, &persona.TemplateID, &persona.Name,
		&persona.Description, &contentJSON, &persona.IsCustomTemplate,
		&persona.CreatedBy, &persona.CreatedAt, &persona.UpdatedAt,
		&persona.UpdatedBy, &persona.DeletedAt, &persona.DeletedBy,
		&persona.CreatedByName, &persona.TemplateName, &persona.OrgName,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get persona", errors.ErrInternalServer.Status)
	}

	if contentJSON != nil {
		json.Unmarshal(contentJSON, &persona.Content)
	} else {
		persona.Content = make(map[string]interface{})
	}

	return persona, nil
}

func (r *PersonaRepository) List(ctx context.Context, orgID *uuid.UUID, page, limit int) ([]*models.Persona, int64, error) {
	var personas []*models.Persona
	var total int64

	offset := (page - 1) * limit

	// Count query
	countQuery := `SELECT COUNT(*) FROM personas WHERE deleted_at IS NULL`
	countArgs := []interface{}{}

	if orgID != nil {
		countQuery += ` AND org_id = $1`
		countArgs = append(countArgs, *orgID)
	}

	err := r.db.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to count personas", errors.ErrInternalServer.Status)
	}

	// List query
	query := `
		SELECT p.id, p.org_id, p.template_id, p.name, p.description, p.content, p.is_custom_template,
			p.created_by, p.created_at, p.updated_at, p.updated_by, p.deleted_at, p.deleted_by,
			u.full_name AS created_by_name, t.name AS template_name, o.name AS org_name
		FROM personas p
		LEFT JOIN users u ON p.created_by = u.id
		LEFT JOIN templates t ON p.template_id = t.id
		LEFT JOIN organizations o ON p.org_id = o.id
		WHERE p.deleted_at IS NULL
	`
	args := []interface{}{}
	argPos := 1

	if orgID != nil {
		query += fmt.Sprintf(` AND p.org_id = $%d`, argPos)
		args = append(args, *orgID)
		argPos++
	}

	query += fmt.Sprintf(` ORDER BY p.created_at DESC LIMIT $%d OFFSET $%d`, argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to list personas", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	for rows.Next() {
		persona := &models.Persona{}
		var contentJSON []byte

		err := rows.Scan(
			&persona.ID, &persona.OrgID, &persona.TemplateID, &persona.Name,
			&persona.Description, &contentJSON, &persona.IsCustomTemplate,
			&persona.CreatedBy, &persona.CreatedAt, &persona.UpdatedAt,
			&persona.UpdatedBy, &persona.DeletedAt, &persona.DeletedBy,
			&persona.CreatedByName, &persona.TemplateName, &persona.OrgName,
		)
		if err != nil {
			return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan persona", errors.ErrInternalServer.Status)
		}

		if contentJSON != nil {
			json.Unmarshal(contentJSON, &persona.Content)
		} else {
			persona.Content = make(map[string]interface{})
		}

		personas = append(personas, persona)
	}

	return personas, total, nil
}

func (r *PersonaRepository) Update(ctx context.Context, persona *models.Persona) error {
	contentJSON, _ := json.Marshal(persona.Content)

	query := `
		UPDATE personas
		SET name = COALESCE($1, name),
			description = COALESCE($2, description),
			template_id = COALESCE($3, template_id),
			content = COALESCE($4::jsonb, content),
			is_custom_template = COALESCE($5, is_custom_template),
			updated_by = $6,
			updated_at = NOW()
		WHERE id = $7 AND deleted_at IS NULL
		RETURNING updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		persona.Name, persona.Description, persona.TemplateID,
		contentJSON, persona.IsCustomTemplate, persona.UpdatedBy, persona.ID,
	).Scan(&persona.UpdatedAt)

	if err == pgx.ErrNoRows {
		return errors.ErrNotFound
	}
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to update persona", errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *PersonaRepository) Delete(ctx context.Context, id, deletedBy uuid.UUID) error {
	query := `
		UPDATE personas
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
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to delete persona", errors.ErrInternalServer.Status)
	}

	return nil
}
