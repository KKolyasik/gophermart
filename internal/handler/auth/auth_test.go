package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/handler/auth/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type AuthHandlerSuite struct {
	suite.Suite

	service *mocks.MockAuthService
	handler *AuthHandler
}

func (s *AuthHandlerSuite) SetupTest() {
	s.service = mocks.NewMockAuthService(s.T())
	s.handler = NewAuthHandler(slog.New(slog.DiscardHandler), s.service)
}

func (s *AuthHandlerSuite) SetupSubTest() {
	s.service = mocks.NewMockAuthService(s.T())
	s.handler = NewAuthHandler(slog.New(slog.DiscardHandler), s.service)
}

func TestAuthHandler(t *testing.T) {
	suite.Run(t, new(AuthHandlerSuite))
}

func (s *AuthHandlerSuite) TestRegister() {
	tests := []struct {
		name       string
		body       string
		setupMock  func()
		wantStatus int
	}{
		{
			name: "valid registration",
			body: `{"login":"user","password":"pass123"}`,
			setupMock: func() {
				s.service.EXPECT().Register(mock.Anything, "user", "pass123").
					Return("token-abc", nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "duplicate user",
			body: `{"login":"user","password":"pass123"}`,
			setupMock: func() {
				s.service.EXPECT().Register(mock.Anything, "user", "pass123").
					Return("", domainerr.ErrAlreadyExists)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "invalid body",
			body: `{"login":"","password":"pass123"}`,
			setupMock: func() {
				s.service.EXPECT().Register(mock.Anything, "", "pass123").
					Return("", domainerr.ErrInvalidInput)
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			s.handler.Register(w, req)

			s.Require().Equal(tt.wantStatus, w.Result().StatusCode)

			cookies := w.Result().Cookies()

			if tt.wantStatus == http.StatusOK {
				s.Require().NotEmpty(cookies)

				var authCookie *http.Cookie
				for _, c := range cookies {
					if c.Name == "Authorization" {
						authCookie = c
						break
					}
				}

				s.Require().NotNil(authCookie)
				s.Equal("token-abc", authCookie.Value)
				s.True(authCookie.HttpOnly)
				s.True(authCookie.Secure)
				s.Equal("/", authCookie.Path)
				s.Equal(3600, authCookie.MaxAge)
			} else {
				s.Empty(cookies)
			}
		})
	}
}

func (s *AuthHandlerSuite) TestAuth() {
	tests := []struct {
		name       string
		body       string
		setupMock  func()
		wantStatus int
	}{
		{
			name: "valid login",
			body: `{"login":"user","password":"pass123"}`,
			setupMock: func() {
				s.service.EXPECT().Login(mock.Anything, "user", "pass123").
					Return("token-abc", nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid body",
			body: `{"login":"user","password":"pass123"}`,
			setupMock: func() {
				s.service.EXPECT().Login(mock.Anything, "user", "pass123").
					Return("", domainerr.ErrInvalidInput)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "any error",
			body: `{"login":"user","password":"pass123"}`,
			setupMock: func() {
				s.service.EXPECT().Login(mock.Anything, "user", "pass123").
					Return("", errors.New("some error"))
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			s.handler.Login(w, req)

			s.Require().Equal(tt.wantStatus, w.Result().StatusCode)

			cookies := w.Result().Cookies()

			if tt.wantStatus == http.StatusOK {
				s.Require().NotEmpty(cookies)

				var authCookie *http.Cookie
				for _, c := range cookies {
					if c.Name == "Authorization" {
						authCookie = c
						break
					}
				}

				s.Require().NotNil(authCookie)
				s.Equal("token-abc", authCookie.Value)
				s.True(authCookie.HttpOnly)
				s.True(authCookie.Secure)
				s.Equal("/", authCookie.Path)
				s.Equal(3600, authCookie.MaxAge)
			} else {
				s.Empty(cookies)
			}
		})
	}
}
