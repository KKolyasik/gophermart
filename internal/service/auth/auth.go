package auth

import (
	"context"
	"log/slog"

	"github.com/Kkolyasik/gophermart/internal/auth"
	"github.com/Kkolyasik/gophermart/internal/config"
	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthStorage описывает доступ к данным пользователей.
type AuthStorage interface {
	Register(ctx context.Context, login, hash string) (uuid.UUID, error)
	Login(ctx context.Context, login string) (uuid.UUID, string, error)
}

// AuthService реализует бизнес-логику регистрации и авторизации.
type AuthService struct {
	storage AuthStorage

	cfg    *config.Config
	logger *slog.Logger
}

// NewAuthService создает auth-сервис.
func NewAuthService(logger *slog.Logger, cfg *config.Config, storage AuthStorage) *AuthService {
	return &AuthService{
		storage: storage,
		cfg:     cfg,
		logger:  logger,
	}
}

// Register регистрирует пользователя и возвращает токен.
func (s *AuthService) Register(ctx context.Context, login, password string) (string, error) {
	const op = "service.auth.Register"

	hash, err := hashPassword(password)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create hash", "err", err)
		return "", err
	}

	uid, err := s.storage.Register(ctx, login, hash)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not register user", "err", err)
		return "", err
	}

	token, err := auth.GenerateToken(uid, s.cfg)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not generate token", "err", err)
		return "", err
	}

	s.logger.With(slog.String("op", op)).Info("user registered succesfully")
	return token, nil
}

// Login проверяет учетные данные и возвращает токен.
func (s *AuthService) Login(ctx context.Context, login, password string) (string, error) {
	const op = "service.auth.Register"

	uid, hash, err := s.storage.Login(ctx, login)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not login user", "err", err)
		return "", err
	}

	if !checkPassword(password, hash) {
		s.logger.With(slog.String("op", op)).Error("invalid password")
		return "", domainerr.ErrInvalidInput
	}

	token, err := auth.GenerateToken(uid, s.cfg)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not generate token", "err", err)
		return "", err
	}

	s.logger.With(slog.String("op", op)).Info("user logined succesfully")
	return token, nil
}

func hashPassword(password string) (string, error) {
	hashPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashPassword), err
}

func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
