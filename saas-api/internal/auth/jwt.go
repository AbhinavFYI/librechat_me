package auth

import (
	"errors"
	"fmt"
	"time"

	"saas-api/config"
	"saas-api/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID       uuid.UUID  `json:"user_id"`
	Email        string     `json:"email"`
	OrgID        *uuid.UUID `json:"org_id,omitempty"`
	IsSuperAdmin bool       `json:"is_super_admin"`
	jwt.RegisteredClaims
}

type TokenService struct {
	config *config.Config
}

func NewTokenService(cfg *config.Config) *TokenService {
	return &TokenService{config: cfg}
}

func (ts *TokenService) GenerateAccessToken(user *models.User) (string, error) {
	expirationTime := time.Now().Add(time.Duration(ts.config.JWT.AccessTokenTTL) * time.Minute)

	claims := &Claims{
		UserID:       user.ID,
		Email:        user.Email,
		OrgID:        user.OrgID,
		IsSuperAdmin: user.IsSuperAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "saas-api",
			Subject:   user.ID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(ts.config.JWT.SecretKey))
}

func (ts *TokenService) GenerateRefreshToken(userID uuid.UUID) (string, error) {
	expirationTime := time.Now().Add(time.Duration(ts.config.JWT.RefreshTokenTTL) * 24 * time.Hour)

	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "saas-api",
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(ts.config.JWT.SecretKey))
}

func (ts *TokenService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(ts.config.JWT.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func (ts *TokenService) ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("authorization header is required")
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		return "", errors.New("authorization header must start with Bearer")
	}

	return authHeader[len(bearerPrefix):], nil
}
