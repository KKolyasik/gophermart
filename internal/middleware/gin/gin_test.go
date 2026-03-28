package ginMiddlewre

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kkolyasik/gophermart/internal/auth"
	"github.com/Kkolyasik/gophermart/internal/config"
	"github.com/Kkolyasik/gophermart/internal/middleware/gin/mocks"
	"github.com/gin-gonic/gin"
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

type MiddlwareSuite struct {
	suite.Suite

	compressor *mocks.MockCompressor
	middleware *Middlware
}

func (s *MiddlwareSuite) SetupTest() {
	cfg := NewMockConfig()
	s.compressor = mocks.NewMockCompressor(s.T())
	s.middleware = NewMiddlware(cfg, slog.New(slog.DiscardHandler), s.compressor)
}

func (s *MiddlwareSuite) SetupSubTest() {
	cfg := NewMockConfig()
	s.compressor = mocks.NewMockCompressor(s.T())
	s.middleware = NewMiddlware(cfg, slog.New(slog.DiscardHandler), s.compressor)
}

func TestMiddlware(t *testing.T) {
	suite.Run(t, new(MiddlwareSuite))
}

func (s *MiddlwareSuite) TestAuthMiddleware() {
	uid := uuid.New()
	token, err := auth.GenerateToken(uid, s.middleware.cfg)
	if err != nil {
		s.Require().NoError(err)
	}
	tests := []struct {
		name       string
		cookie     *http.Cookie
		wantStatus int
	}{
		{
			name:       "success",
			cookie:     &http.Cookie{Name: "Authorization", Value: token},
			wantStatus: http.StatusOK,
		},
		{
			name:       "no cookie",
			cookie:     nil,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token",
			cookie:     &http.Cookie{Name: "Authorization", Value: "invalid"},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			gin.SetMode(gin.TestMode)

			router := gin.New()
			router.Use(s.middleware.AuthMiddleware)
			router.GET("/protected", func(ctx *gin.Context) {
				contextUID := ctx.Request.Context().Value(auth.UserIDKey)
				s.Require().NotNil(contextUID)
				s.Require().Equal(uid, contextUID)
				ctx.Status(http.StatusOK)
			})

			r := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.cookie != nil {
				r.AddCookie(tt.cookie)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			s.Require().Equal(tt.wantStatus, w.Result().StatusCode)
		})
	}
}

func (s *MiddlwareSuite) TestCompressionMiddleware() {
	encoding := "gzip"

	tests := []struct {
		name            string
		setupMock       func()
		contentEncoding string
		acceptEncoding  string
		requestBody     string
		responseBody    string
		wantStatus      int
		wantEncoded     bool
	}{
		{
			name: "decode request and encode response",
			setupMock: func() {
				s.compressor.EXPECT().Encoding().Return(encoding).Once()
				s.compressor.EXPECT().NewReader(mock.Anything).RunAndReturn(
					func(r io.Reader) (io.ReadCloser, error) {
						return io.NopCloser(r), nil
					},
				).Once()
				s.compressor.EXPECT().NewWriter(mock.Anything).RunAndReturn(
					func(w io.Writer) (io.WriteCloser, error) {
						return nopWriteCloser{w}, nil
					},
				).Once()
			},
			contentEncoding: encoding,
			acceptEncoding:  encoding,
			requestBody:     "Hello from request",
			responseBody:    "Hello from response",
			wantStatus:      http.StatusOK,
			wantEncoded:     true,
		},
		{
			name: "no encoding headers",
			setupMock: func() {
				s.compressor.EXPECT().Encoding().Return(encoding).Once()
			},
			contentEncoding: "",
			acceptEncoding:  "",
			requestBody:     "Hello from request",
			responseBody:    "Hello from response",
			wantStatus:      http.StatusOK,
			wantEncoded:     false,
		},
		{
			name: "only decode request",
			setupMock: func() {
				s.compressor.EXPECT().Encoding().Return(encoding).Once()
				s.compressor.EXPECT().NewReader(mock.Anything).RunAndReturn(
					func(r io.Reader) (io.ReadCloser, error) {
						return io.NopCloser(r), nil
					},
				).Once()
			},
			contentEncoding: encoding,
			acceptEncoding:  "",
			requestBody:     "Hello from request",
			responseBody:    "Hello from response",
			wantStatus:      http.StatusOK,
			wantEncoded:     false,
		},
		{
			name: "only encode response",
			setupMock: func() {
				s.compressor.EXPECT().Encoding().Return(encoding).Once()
				s.compressor.EXPECT().NewWriter(mock.Anything).RunAndReturn(
					func(w io.Writer) (io.WriteCloser, error) {
						return nopWriteCloser{w}, nil
					},
				).Once()
			},
			contentEncoding: "",
			acceptEncoding:  encoding,
			requestBody:     "Hello from request",
			responseBody:    "Hello from response",
			wantStatus:      http.StatusOK,
			wantEncoded:     true,
		},
		{
			name: "reader creation error",
			setupMock: func() {
				s.compressor.EXPECT().Encoding().Return(encoding).Once()
				s.compressor.EXPECT().NewReader(mock.Anything).Return(nil, errors.New("error")).Once()
			},
			contentEncoding: encoding,
			acceptEncoding:  encoding,
			requestBody:     "Hello from request",
			responseBody:    "Hello from response",
			wantStatus:      http.StatusBadRequest,
			wantEncoded:     false,
		},
		{
			name: "writer creation error",
			setupMock: func() {
				s.compressor.EXPECT().Encoding().Return(encoding).Once()
				s.compressor.EXPECT().NewWriter(mock.Anything).Return(nil, errors.New("error")).Once()
			},
			contentEncoding: "",
			acceptEncoding:  encoding,
			requestBody:     "Hello from request",
			responseBody:    "Hello from response",
			wantStatus:      http.StatusBadRequest,
			wantEncoded:     false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			gin.SetMode(gin.TestMode)

			router := gin.New()
			router.Use(s.middleware.CompressionMiddleware)
			router.POST("/test", func(ctx *gin.Context) {
				_, err := io.ReadAll(ctx.Request.Body)
				s.Require().NoError(err)
				_, err = ctx.Writer.Write([]byte(tt.responseBody))
				s.Require().NoError(err)
				ctx.Status(http.StatusOK)
			})

			r := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.requestBody))
			if tt.contentEncoding != "" {
				r.Header.Set("Content-Encoding", tt.contentEncoding)
			}
			if tt.acceptEncoding != "" {
				r.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			s.Require().Equal(tt.wantStatus, w.Result().StatusCode)

			if tt.contentEncoding != "" && tt.wantStatus == http.StatusOK {
				s.Require().Equal(tt.responseBody, w.Body.String())
			}

			if tt.wantEncoded {
				s.Require().Equal(encoding, w.Result().Header.Get("Content-Encoding"))
				s.Require().Equal("Accept-Encoding", w.Result().Header.Get("Vary"))
			} else {
				s.Require().Equal("", w.Result().Header.Get("Content-Encoding"))
				s.Require().Equal("", w.Result().Header.Get("Vary"))
			}
		})
	}
}

type nopWriteCloser struct {
	io.Writer
}

func (w nopWriteCloser) Close() error {
	return nil
}
