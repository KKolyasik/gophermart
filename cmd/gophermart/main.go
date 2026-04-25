package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Kkolyasik/gophermart/internal/app"
	"github.com/Kkolyasik/gophermart/internal/config"
)

func main() {

	logger := initializeLogger()
	logger.Info("Запуск приложения")

	ctxApp, cancelApp := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelApp()

	cfg, err := config.NewConfig()
	if err != nil {
		exitWithError(logger, "Не удалось загрузить конфигурацию", err)
	}

	app, err := app.NewApp(ctxApp, cfg, logger)
	if err != nil {
		exitWithError(logger, "Не удалось инициализировать приложение", err)
	}
	serverErrCh := app.Run()

	select {
	case <-ctxApp.Done():
		cancelApp()
	case err, ok := <-serverErrCh:
		if ok && err != nil {
			exitWithError(logger, "Сервер остановился с ошибкой", err)
		}
		return
	}

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()

	if err := app.Stop(ctxShutdown); err != nil {
		exitWithError(logger, "Сервер принудительно остановлен", err)
	}
	logger.Info("Приложение остановилось")
}

// Инициализация логгера
func initializeLogger() *slog.Logger {

	logger := slog.New(slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug},
	))

	return logger
}

func exitWithError(logger *slog.Logger, msg string, err error) {
	logger.Error(msg, "err", err)
	os.Exit(1)
}
