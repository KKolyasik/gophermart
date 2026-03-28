package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/Kkolyasik/gophermart/internal/auth"
	"github.com/Kkolyasik/gophermart/internal/config"
	"github.com/Kkolyasik/gophermart/internal/service/auth/mocks"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func NewMockConfig() *config.Config {
	return &config.Config{
		RunAddress:           "test",
		DatabaseURI:          "test",
		AccrualSystemAddress: "test",
		SecretKey:            "test",
	}
}

type AuthServiceSuite struct {
	suite.Suite

	storage *mocks.MockAuthStorage
	service *AuthService
}

func (s *AuthServiceSuite) SetupTest() {
	cfg := NewMockConfig()
	s.storage = mocks.NewMockAuthStorage(s.T())
	s.service = NewAuthService(slog.New(slog.DiscardHandler), cfg, s.storage)
}

func (s *AuthServiceSuite) SetupSubTest() {
	cfg := NewMockConfig()
	s.storage = mocks.NewMockAuthStorage(s.T())
	s.service = NewAuthService(slog.New(slog.DiscardHandler), cfg, s.storage)
}

func TestAuthService(t *testing.T) {
	suite.Run(t, new(AuthServiceSuite))
}

func (s *AuthServiceSuite) TestRegister() {
	uid := uuid.New()
	tests := []struct {
		name      string
		setupMock func()
		login     string
		password  string
		wantErr   bool
	}{
		{
			name: "success",
			setupMock: func() {
				s.storage.EXPECT().Register(mock.Anything, "user", mock.MatchedBy(func(h string) bool {
					return h != "pass123" && checkPassword("pass123", h)
				})).
					Return(uid, nil)
			},
			login:    "user",
			password: "pass123",
			wantErr:  false,
		},
		{
			name: "storage error",
			setupMock: func() {
				s.storage.EXPECT().Register(mock.Anything, "user", mock.MatchedBy(func(h string) bool {
					return h != "pass123" && checkPassword("pass123", h)
				})).Return(uuid.Nil, errors.New("error"))
			},
			login:    "user",
			password: "pass123",
			wantErr:  true,
		},
		{
			name:      "hash error",
			setupMock: func() {},
			login:     "user",
			password:  strings.Repeat("pass123", 72),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			token, err := s.service.Register(context.Background(), tt.login, tt.password)
			if tt.wantErr {
				s.Require().Error(err)
				return
			}

			s.Require().NoError(err)

			var claims auth.UserClaims
			parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
				return []byte(s.service.cfg.SecretKey), nil
			})

			s.Require().NoError(err)
			s.True(parsed.Valid)
			s.Equal(uid, claims.UID)
		})
	}
}

func (s *AuthServiceSuite) TestLogin() {
	uid := uuid.New()
	tests := []struct {
		name      string
		setupMock func()
		login     string
		password  string
		wantErr   bool
	}{
		{
			name: "success",
			setupMock: func() {
				hash, err := hashPassword("pass123")
				s.Require().NoError(err)
				s.storage.EXPECT().Login(mock.Anything, "user").
					Return(uid, hash, nil)
			},
			login:    "user",
			password: "pass123",
			wantErr:  false,
		},
		{
			name: "storage error",
			setupMock: func() {
				s.storage.EXPECT().Login(mock.Anything, "user").
					Return(uuid.Nil, "", errors.New("error"))
			},
			login:    "user",
			password: "pass123",
			wantErr:  true,
		},
		{
			name: "check password error",
			setupMock: func() {
				hash, err := hashPassword("pass321")
				s.Require().NoError(err)
				s.storage.EXPECT().Login(mock.Anything, "user").
					Return(uid, hash, nil)
			},
			login:    "user",
			password: "pass123",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			token, err := s.service.Login(context.Background(), tt.login, tt.password)
			if tt.wantErr {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)

			var claims auth.UserClaims
			parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
				return []byte(s.service.cfg.SecretKey), nil
			})

			s.Require().NoError(err)
			s.True(parsed.Valid)
			s.Equal(uid, claims.UID)
		})
	}
}
