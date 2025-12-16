package repositories

import (
	"context"
	"encoding/json"

	"saas-api/internal/database"
	"saas-api/internal/models"
	"saas-api/pkg/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RefreshTokenRepository struct {
	db *database.DB
}

func NewRefreshTokenRepository(db *database.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (
			id, user_id, token_hash, device_info, ip_address, user_agent, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, last_used_at
	`

	deviceInfoJSON, _ := json.Marshal(token.DeviceInfo)

	err := r.db.Pool.QueryRow(ctx, query,
		token.ID, token.UserID, token.TokenHash,
		deviceInfoJSON, token.IPAddress, token.UserAgent, token.ExpiresAt,
	).Scan(&token.CreatedAt, &token.LastUsedAt)

	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to create refresh token", errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *RefreshTokenRepository) GetByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	token := &models.RefreshToken{}
	var deviceInfoJSON []byte

	// Query directly - refresh_tokens table doesn't have RLS enabled
	query := `
		SELECT id, user_id, token_hash, device_info, 
			ip_address::TEXT, user_agent,
			expires_at, created_at, last_used_at, revoked_at, revoked_by, revoked_reason
		FROM refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL
	`

	var ipAddressStr *string
	var userAgentStr *string

	err := r.db.Pool.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID, &token.UserID, &token.TokenHash, &deviceInfoJSON,
		&ipAddressStr, &userAgentStr, &token.ExpiresAt,
		&token.CreatedAt, &token.LastUsedAt, &token.RevokedAt,
		&token.RevokedBy, &token.RevokedReason,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get refresh token", errors.ErrInternalServer.Status)
	}

	if len(deviceInfoJSON) > 0 {
		json.Unmarshal(deviceInfoJSON, &token.DeviceInfo)
	} else {
		token.DeviceInfo = make(map[string]interface{})
	}

	// Set IP address and user agent
	token.IPAddress = ipAddressStr
	token.UserAgent = userAgentStr

	return token, nil
}

func (r *RefreshTokenRepository) UpdateLastUsed(ctx context.Context, tokenID uuid.UUID) error {
	query := `UPDATE refresh_tokens SET last_used_at = NOW() WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, tokenID)
	return err
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, tokenID uuid.UUID, revokedBy uuid.UUID, reason string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW(), revoked_by = $1, revoked_reason = $2
		WHERE id = $3 AND revoked_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, revokedBy, reason, tokenID)
	return err
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, revokedBy uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW(), revoked_by = $1, revoked_reason = 'User logged out'
		WHERE user_id = $2 AND revoked_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, revokedBy, userID)
	return err
}

func (r *RefreshTokenRepository) CleanupExpired(ctx context.Context) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW() AND revoked_at IS NULL`
	_, err := r.db.Pool.Exec(ctx, query)
	return err
}
