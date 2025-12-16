package repositories

import (
	"context"
	"saas-api/internal/database"
	"saas-api/internal/models"
	"saas-api/pkg/errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ScreenerRepository struct {
	db *database.DB
}

func NewScreenerRepository(db *database.DB) *ScreenerRepository {
	return &ScreenerRepository{db: db}
}

func (r *ScreenerRepository) Create(ctx context.Context, screener *models.Screener) error {
	// Clean query - remove \n and format properly
	cleanedQuery := strings.ReplaceAll(screener.Query, "\\n", " ")
	cleanedQuery = strings.ReplaceAll(cleanedQuery, "\n", " ")
	cleanedQuery = strings.Join(strings.Fields(cleanedQuery), " ")
	cleanedQuery = strings.TrimSpace(cleanedQuery)

	query := `
		INSERT INTO settings (
			org_id, user_id, screener_name, "tableName", query, "universeList", explainer, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		screener.OrgID, // Can be nil for superadmins
		screener.UserID, screener.ScreenerName,
		screener.TableName, cleanedQuery, screener.UniverseList, screener.Explainer, screener.IsActive,
	).Scan(&screener.ID, &screener.CreatedAt, &screener.UpdatedAt)

	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to create screener", errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *ScreenerRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Screener, error) {
	screener := &models.Screener{}

	query := `
		SELECT id, org_id, user_id, screener_name, "tableName", query, "universeList", explainer, is_active, created_at, updated_at
		FROM settings
		WHERE id = $1 AND is_active = true
	`

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&screener.ID, &screener.OrgID, &screener.UserID, &screener.ScreenerName,
		&screener.TableName, &screener.Query, &screener.UniverseList, &screener.Explainer,
		&screener.IsActive, &screener.CreatedAt, &screener.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get screener", errors.ErrInternalServer.Status)
	}

	return screener, nil
}

func (r *ScreenerRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.Screener, error) {
	var screeners []*models.Screener

	query := `
		SELECT id, org_id, user_id, screener_name, "tableName", query, "universeList", explainer, is_active, created_at, updated_at
		FROM settings
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to list screeners", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	for rows.Next() {
		screener := &models.Screener{}
		err := rows.Scan(
			&screener.ID, &screener.OrgID, &screener.UserID, &screener.ScreenerName,
			&screener.TableName, &screener.Query, &screener.UniverseList, &screener.Explainer,
			&screener.IsActive, &screener.CreatedAt, &screener.UpdatedAt,
		)
		if err != nil {
			return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan screener", errors.ErrInternalServer.Status)
		}
		screeners = append(screeners, screener)
	}

	return screeners, nil
}

func (r *ScreenerRepository) ExistsByName(ctx context.Context, userID uuid.UUID, screenerName string) (bool, error) {
	query := `
		SELECT COUNT(*) 
		FROM settings
		WHERE user_id = $1 AND screener_name = $2 AND is_active = true
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, userID, screenerName).Scan(&count)

	if err != nil {
		return false, errors.WrapError(err, "INTERNAL_ERROR", "Failed to check screener existence", errors.ErrInternalServer.Status)
	}

	return count > 0, nil
}

func (r *ScreenerRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	// Soft delete by setting is_active to false
	query := `
		UPDATE settings 
		SET is_active = false, updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND is_active = true
		RETURNING id
	`

	var deletedID uuid.UUID
	err := r.db.Pool.QueryRow(ctx, query, id, userID).Scan(&deletedID)

	if err == pgx.ErrNoRows {
		return errors.ErrNotFound
	}
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to delete screener", errors.ErrInternalServer.Status)
	}

	return nil
}

// GetByName gets a screener by name for a user (regardless of is_active status)
func (r *ScreenerRepository) GetByName(ctx context.Context, userID uuid.UUID, screenerName string) (*models.Screener, error) {
	screener := &models.Screener{}

	query := `
		SELECT id, org_id, user_id, screener_name, "tableName", query, "universeList", explainer, is_active, created_at, updated_at
		FROM settings
		WHERE user_id = $1 AND screener_name = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	err := r.db.Pool.QueryRow(ctx, query, userID, screenerName).Scan(
		&screener.ID, &screener.OrgID, &screener.UserID, &screener.ScreenerName,
		&screener.TableName, &screener.Query, &screener.UniverseList, &screener.Explainer,
		&screener.IsActive, &screener.CreatedAt, &screener.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get screener by name", errors.ErrInternalServer.Status)
	}

	return screener, nil
}

// ToggleActive toggles the is_active status of a screener
func (r *ScreenerRepository) ToggleActive(ctx context.Context, id, userID uuid.UUID) (*models.Screener, error) {
	screener := &models.Screener{}

	query := `
		UPDATE settings 
		SET is_active = NOT is_active, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING id, org_id, user_id, screener_name, "tableName", query, "universeList", explainer, is_active, created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query, id, userID).Scan(
		&screener.ID, &screener.OrgID, &screener.UserID, &screener.ScreenerName,
		&screener.TableName, &screener.Query, &screener.UniverseList, &screener.Explainer,
		&screener.IsActive, &screener.CreatedAt, &screener.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to toggle screener active status", errors.ErrInternalServer.Status)
	}

	return screener, nil
}

// Update updates an existing screener's data and sets is_active to true
func (r *ScreenerRepository) Update(ctx context.Context, screener *models.Screener) error {
	// Clean query - remove \n and format properly
	cleanedQuery := strings.ReplaceAll(screener.Query, "\\n", " ")
	cleanedQuery = strings.ReplaceAll(cleanedQuery, "\n", " ")
	cleanedQuery = strings.Join(strings.Fields(cleanedQuery), " ")
	cleanedQuery = strings.TrimSpace(cleanedQuery)

	query := `
		UPDATE settings 
		SET "tableName" = $1, query = $2, "universeList" = $3, explainer = $4, is_active = true, updated_at = NOW()
		WHERE id = $5 AND user_id = $6
		RETURNING updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		screener.TableName, cleanedQuery, screener.UniverseList, screener.Explainer,
		screener.ID, screener.UserID,
	).Scan(&screener.UpdatedAt)

	if err == pgx.ErrNoRows {
		return errors.ErrNotFound
	}
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to update screener", errors.ErrInternalServer.Status)
	}

	return nil
}
