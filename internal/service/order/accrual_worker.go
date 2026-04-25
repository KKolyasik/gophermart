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
			if retryAfter, ok := w.processOrders(ctx); ok {
				wait(ctx, retryAfter)
			}
		}
	}
}

func (w *AccrualWorker) processOrders(ctx context.Context) (time.Duration, bool) {
	const op = "service.order.processOrders"

	orders, err := w.storage.GetUnprocessedOrders(ctx)
	w.logger.With(slog.String("op", op)).Debug("records were received from the database", "amount", len(orders))
	if err != nil {
		return 0, false
	}

	batchCtx, cancelBatch := context.WithCancel(ctx)
	defer cancelBatch()

	retryAfterCh := make(chan time.Duration, 1)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

loop:
	for _, order := range orders {
		select {
		case <-batchCtx.Done():
			break loop
		case sem <- struct{}{}:
			wg.Go(func() {
				defer func() { <-sem }()
				if err := w.processOrder(batchCtx, order); err != nil {
					var retryErr *domainerr.RetryAfterError
					switch {
					case errors.As(err, &retryErr):
						w.logger.With(slog.String("op", op)).Info("too many requests", "retry_after", retryErr.RetryAfter)
						select {
						case retryAfterCh <- time.Second * time.Duration(retryErr.RetryAfter):
							cancelBatch()
						default:
						}
					case errors.Is(err, context.Canceled):
					case errors.Is(err, domainerr.ErrNoDataFound):
					case errors.Is(err, domainerr.ErrExternalServiceNotAvailable):
						w.logger.With(slog.String("op", op)).Warn("accrual service unavailable")
					default:
						w.logger.With(slog.String("op", op)).Error("unexpected error", "error", err)
					}
				}
			})
		}
	}
	wg.Wait()

	select {
	case duration := <-retryAfterCh:
		return duration, true
	default:
	}

	w.logger.With(slog.String("op", op)).Debug("processing completed")
	return 0, false
}

func (w *AccrualWorker) processOrder(ctx context.Context, order model.Order) error {
	const op = "service.order.processOrder"

	accrual, err := w.client.GetOrderAccrual(ctx, order.Number)
	if err != nil {
		return err
	}

	if err := w.storage.CompleteOrder(ctx, order.UserID, order.Number, accrual.Status, accrual.Accrual); err != nil {
		w.logger.With(slog.String("op", op)).Error("error when requesting the database", "err", err)
		return err
	}
	return nil
}

func wait(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
