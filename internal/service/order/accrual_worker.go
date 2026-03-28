package order

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Kkolyasik/gophermart/internal/config"
	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/model"
	"github.com/google/uuid"
)

// AccrualProvider описывает клиент системы начислений.
type AccrualProvider interface {
	GetOrderAccrual(ctx context.Context, orderNumber string) (model.AccrualResponse, error)
}

// AccrualStorage описывает операции хранилища для воркера начислений.
type AccrualStorage interface {
	GetUnprocessedOrders(ctx context.Context) ([]model.Order, error)
	CompleteOrder(ctx context.Context, uid uuid.UUID, number string, status model.Status, accrual *float64) error
}

// AccrualWorker периодически запрашивает статусы заказов и обновляет БД.
type AccrualWorker struct {
	client   AccrualProvider
	storage  AccrualStorage
	interval time.Duration
	cfg      *config.Config
	logger   *slog.Logger
}

// NewAccrualWorker создает воркер синхронизации заказов с accrual-сервисом.
func NewAccrualWorker(
	logger *slog.Logger,
	cfg *config.Config,
	interval time.Duration,
	storage AccrualStorage,
	client AccrualProvider,
) *AccrualWorker {
	return &AccrualWorker{
		client:   client,
		storage:  storage,
		interval: interval,
		cfg:      cfg,
		logger:   logger,
	}
}

// Run запускает фоновую обработку до отмены контекста.
func (w *AccrualWorker) Run(ctx context.Context) {
	const op = "service.order.Run"

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.logger.With(slog.String("op", op)).Debug("start of order processing")
			w.processOrders(ctx)
		}
	}
}

func (w *AccrualWorker) processOrders(ctx context.Context) {
	const op = "service.order.processOrders"

	orders, err := w.storage.GetUnprocessedOrders(ctx)
	w.logger.With(slog.String("op", op)).Debug("records were received from the database", "amount", len(orders))
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for _, order := range orders {
		select {
		case <-ctx.Done():
			return
		case sem <- struct{}{}:
			wg.Go(func() {
				defer func() { <-sem }()
				w.processOrder(ctx, order)
			})
		}
	}
	wg.Wait()
	w.logger.With(slog.String("op", op)).Debug("processing completed")
}

func (w *AccrualWorker) processOrder(ctx context.Context, order model.Order) {
	const op = "service.order.processOrder"

	accrual, err := w.client.GetOrderAccrual(ctx, order.Number)
	if err != nil {
		w.logger.With(slog.String("op", op)).Error("error when sending a request to the points calculation service")
		var retryErr *domainerr.RetryAfterError
		switch {
		case errors.As(err, &retryErr):
			w.logger.With(slog.String("op", op)).Info("too many requests", "retry_after", retryErr.RetryAfter)
			time.Sleep(time.Duration(retryErr.RetryAfter) * time.Second)
		case errors.Is(err, domainerr.ErrNoDataFound):
		case errors.Is(err, domainerr.ErrExternalServiceNotAvailable):
			w.logger.With(slog.String("op", op)).Warn("accrual service unavailable")
		default:
			w.logger.With(slog.String("op", op)).Error("unexpected error", "error", err)
		}
		return
	}

	if err := w.storage.CompleteOrder(ctx, order.UserID, order.Number, accrual.Status, accrual.Accrual); err != nil {
		w.logger.With(slog.String("op", op)).Error("error when requesting the database", "err", err)
	}
}
