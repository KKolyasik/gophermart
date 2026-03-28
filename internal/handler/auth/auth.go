package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/model"
)

// AuthService описывает операции авторизации для handler-слоя.
type AuthService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
}

// AuthHandler обрабатывает HTTP-запросы регистрации и логина.
type AuthHandler struct {
	service AuthService

	logger *slog.Logger
}

// NewAuthHandler создает auth-handler.
func NewAuthHandler(logger *slog.Logger, service AuthService) *AuthHandler {
	return &AuthHandler{
		service: service,
		logger:  logger,
	}
}

// Register регистрирует пользователя и выставляет cookie авторизации.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	const op = "handler.auth.Register"

	defer func() {
		if err := r.Body.Close(); err != nil {
			h.logger.With(slog.String("op", op)).Error("body close error", "err", err)
		}
	}()
	enc := json.NewDecoder(r.Body)

	var userCredentials model.Credentials
	if err := enc.Decode(&userCredentials); err != nil {
		h.logger.With(slog.String("op", op)).Error("can not decode body", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	token, err := h.service.Register(r.Context(), userCredentials.Login, userCredentials.Password)
	if err != nil {
		if errors.Is(err, domainerr.ErrAlreadyExists) {
			h.logger.With(slog.String("op", op)).Error("user already exist", "err", err)
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		h.logger.With(slog.String("op", op)).Error("registration error", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "Authorization",
		Value:    token,
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   true,
	})

	w.WriteHeader(http.StatusOK)
}

// Login аутентифицирует пользователя и выставляет cookie авторизации.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	const op = "handler.auth.Login"

	defer func() {
		if err := r.Body.Close(); err != nil {
			h.logger.With(slog.String("op", op)).Error("body close error", "err", err)
		}
	}()
	enc := json.NewDecoder(r.Body)

	var userCredentials model.Credentials
	if err := enc.Decode(&userCredentials); err != nil {
		h.logger.With(slog.String("op", op)).Error("can not decode body", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	token, err := h.service.Login(r.Context(), userCredentials.Login, userCredentials.Password)
	if err != nil {
		if errors.Is(err, domainerr.ErrInvalidInput) {
			h.logger.With(slog.String("op", op)).Error("incorrect login/password", "err", err)
			http.Error(w, "incorrect login/password", http.StatusUnauthorized)
			return
		}
		h.logger.With(slog.String("op", op)).Error("login error", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "Authorization",
		Value:    token,
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   true,
	})

	w.WriteHeader(http.StatusOK)
}
