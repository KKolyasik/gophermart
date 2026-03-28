package app

import (
	"compress/gzip"
	"context"
	"log/slog"
	"net/http"

	"github.com/Kkolyasik/gophermart/internal/config"
	gzipEncoding "github.com/Kkolyasik/gophermart/internal/encoding/gzip"
	authHandler "github.com/Kkolyasik/gophermart/internal/handler/auth"
	balanceHandler "github.com/Kkolyasik/gophermart/internal/handler/balance"
	orderHandler "github.com/Kkolyasik/gophermart/internal/handler/order"
	"github.com/Kkolyasik/gophermart/internal/luhn"
	ginMiddlewre "github.com/Kkolyasik/gophermart/internal/middleware/gin"
	"github.com/Kkolyasik/gophermart/internal/provider/accrual"
	authService "github.com/Kkolyasik/gophermart/internal/service/auth"
	balanceService "github.com/Kkolyasik/gophermart/internal/service/balance"
	orderService "github.com/Kkolyasik/gophermart/internal/service/order"
	"github.com/Kkolyasik/gophermart/internal/storage/postgres"
	"github.com/gin-gonic/gin"
)

// App инкапсулирует HTTP-сервер и его зависимости.
type App struct {
	server *http.Server

	logger *slog.Logger
}

// NewApp собирает зависимости и настраивает HTTP-роутер приложения.
func NewApp(ctx context.Context, cfg *config.Config, logger *slog.Logger) *App {
	storage := postgres.NewStorage(ctx, cfg.DatabaseURI, logger)
	validator := luhn.NewLuhnValidator()
	accrualProvider := accrual.NewProvider(cfg, logger)

	authSvc := authService.NewAuthService(logger, cfg, storage)
	authH := authHandler.NewAuthHandler(logger, authSvc)

	balanceSvc := balanceService.NewBalanceService(logger, storage, validator)
	balanceH := balanceHandler.NewBalanceHandler(logger, balanceSvc)

	orderSvc := orderService.NewOrderService(ctx, logger, cfg, storage, accrualProvider, validator)
	orderH := orderHandler.NewOrderHandler(logger, orderSvc)

	encoder := gzipEncoding.NewGZIPCompressor(gzip.BestSpeed)

	middleware := ginMiddlewre.NewMiddlware(cfg, logger, encoder)

	router := gin.New()
	router.Use(middleware.CompressionMiddleware)

	public := router.Group("/")

	public.POST("/api/user/register", gin.WrapF(authH.Register))
	public.POST("/api/user/login", gin.WrapF(authH.Login))

	authorized := router.Group("/")

	authorized.Use(middleware.AuthMiddleware)
	authorized.GET("/api/user/balance", gin.WrapF(balanceH.GetBalance))
	authorized.POST("/api/user/balance/withdraw", gin.WrapF(balanceH.CreateWithdraw))
	authorized.GET("/api/user/withdrawals", gin.WrapF(balanceH.GetWithdrawals))

	authorized.POST("/api/user/orders", gin.WrapF(orderH.CreateOrder))
	authorized.GET("/api/user/orders", gin.WrapF(orderH.CalculatePoints))

	server := &http.Server{
		Addr:    cfg.RunAddress,
		Handler: router,
	}
	return &App{
		server: server,
		logger: logger,
	}
}

// Run запускает HTTP-сервер в отдельной горутине.
func (a *App) Run() error {

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	return nil
}

// Stop корректно останавливает HTTP-сервер.
func (a *App) Stop(ctx context.Context) error {
	if err := a.server.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
