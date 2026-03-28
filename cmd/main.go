package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		log.Fatal(err)
	}

	app := app.NewApp(ctxApp, cfg, logger)
	if err := app.Run(); err != nil {
		log.Fatalf("listen: %s\n", err)
	}

	<-ctxApp.Done()
	cancelApp()

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := app.Stop(ctxShutdown); err != nil {
		log.Fatal("Сервер принудительно остановлен:", err)
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
