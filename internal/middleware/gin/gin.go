package ginMiddlewre

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Kkolyasik/gophermart/internal/auth"
	"github.com/Kkolyasik/gophermart/internal/config"
	"github.com/gin-gonic/gin"
)

// Compressor описывает адаптер сжатия для middleware.
type Compressor interface {
	Encoding() string
	NewWriter(w io.Writer) (io.WriteCloser, error)
	NewReader(r io.Reader) (io.ReadCloser, error)
}

type compressWriter struct {
	gin.ResponseWriter
	writer io.WriteCloser
}

func (c *compressWriter) Write(p []byte) (int, error) {
	return c.writer.Write(p)
}

// Middlware объединяет middleware для авторизации и сжатия.
type Middlware struct {
	compressor Compressor

	cfg    *config.Config
	logger *slog.Logger
}

// NewMiddlware создает набор gin-мидлварей приложения.
func NewMiddlware(cfg *config.Config, logger *slog.Logger, compressor Compressor) *Middlware {
	return &Middlware{
		compressor: compressor,
		cfg:        cfg,
		logger:     logger,
	}
}

// AuthMiddleware проверяет JWT и добавляет uid пользователя в context запроса.
func (m *Middlware) AuthMiddleware(ctx *gin.Context) {
	const op = "middleware.gin.AuthMiddleware"

	token, err := ctx.Request.Cookie("Authorization")
	if err != nil {
		m.logger.With(slog.String("op", op)).Error("can not extract token", "err", err)
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	uid, err := auth.ParseToken(token.Value, m.cfg)
	if err != nil {
		m.logger.With(slog.String("op", op)).Error("can not parse token", "err", err)
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	m.logger.With(slog.String("op", op)).Info("uid successfully added to context")
	ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), auth.UserIDKey, uid))
	ctx.Next()
}

// CompressionMiddleware декодирует входящее и кодирует исходящее тело в gzip.
func (m *Middlware) CompressionMiddleware(ctx *gin.Context) {
	const op = "middleware.gin.CompressionMiddleware"
	encoding := m.compressor.Encoding()

	contentEncoding := ctx.GetHeader("Content-Encoding")
	isEncoded := strings.Contains(contentEncoding, encoding)

	if isEncoded {
		m.logger.With(slog.String("op", op)).Info("content decoding")
		decoder, err := m.compressor.NewReader(ctx.Request.Body)
		if err != nil {
			m.logger.With(slog.String("op", op)).Error("decoder creation error", "err", err)
			ctx.AbortWithStatus(http.StatusBadRequest)
			return
		}
		ctx.Request.Body = decoder
	}

	acceptEncoding := ctx.GetHeader("Accept-Encoding")
	isAcceptEncoding := strings.Contains(acceptEncoding, encoding)

	if isAcceptEncoding {
		m.logger.With(slog.String("op", op)).Info("content encoding")
		encoder, err := m.compressor.NewWriter(ctx.Writer)
		if err != nil {
			m.logger.With(slog.String("op", op)).Error("encoder creation error", "err", err)
			ctx.AbortWithStatus(http.StatusBadRequest)
			return
		}
		ctx.Header("Content-Encoding", encoding)
		ctx.Header("Vary", "Accept-Encoding")

		ctx.Writer = &compressWriter{
			ResponseWriter: ctx.Writer,
			writer:         encoder,
		}
		defer func() {
			if err := encoder.Close(); err != nil {
				m.logger.With(slog.String("op", op)).Error("encoder close error", "err", err)
			}
		}()
	}
	ctx.Next()
}
