package auth

import (
	"context"
	"errors"
	"time"

	"github.com/Kkolyasik/gophermart/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrInvalidToken возвращается при невалидном токене.
var ErrInvalidToken = errors.New("invalid token")

type ctxKey string

// userIDKey — ключ uid пользователя в context.
const userIDKey ctxKey = "uid"

// UserClaims хранит uid пользователя в JWT.
type UserClaims struct {
	jwt.RegisteredClaims
	UID uuid.UUID `json:"uid"`
}

// GetUserID достает uid пользователя из context.
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	uid, ok :=  ctx.Value(userIDKey).(uuid.UUID)
	return uid, ok
}

func WithUserID(ctx context.Context, uid uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, uid)
}

// GenerateToken создает JWT-токен для пользователя.
func GenerateToken(uid uuid.UUID, cfg *config.Config) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour))},
		UID:              uid,
	})

	return token.SignedString([]byte(cfg.SecretKey))
}

// ParseToken валидирует JWT-токен и возвращает uid пользователя.
func ParseToken(tokenString string, cfg *config.Config) (uuid.UUID, error) {
	var claims UserClaims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) { return []byte(cfg.SecretKey), nil })

	if err != nil {
		return uuid.Nil, err
	}

	if !token.Valid {
		return uuid.Nil, ErrInvalidToken
	}

	return claims.UID, nil
}
