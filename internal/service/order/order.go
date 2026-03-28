package order

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Kkolyasik/gophermart/internal/config"
	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/model"
	"github.com/google/uuid"
)

// OrderValidator проверяет корректность номера заказа.
type OrderValidator interface {
	Valid(number string) bool
}

// OrderStorage описывает доступ к данным заказов.
type OrderStorage interface {
	GetOrderByNumber(ctx context.Context, number string) (model.Order, error)
	CreateOrder(ctx context.Context, uid uuid.UUID, number string) error
	GetUserOrders(ctx context.Context, uid uuid.UUID) ([]model.Order, error)
	GetUnprocessedOrders(ctx context.Context) ([]model.Order, error)
	CompleteOrder(ctx context.Context, uid uuid.UUID, number string, status model.Status, accrual *float64) error
}

// OrderService реализует бизнес-логику обработки заказов.
type OrderService struct {
	storage   OrderStorage
	validator OrderValidator

	cfg    *config.Config
	logger *slog.Logger
}

// NewOrderService создает order-сервис и запускает воркер начислений.
func NewOrderService(
	ctx context.Context,
	logger *slog.Logger,
	cfg *config.Config,
	storage OrderStorage,
	accrualProvider AccrualProvider,
	validator OrderValidator,
) *OrderService {
	accrualWorker := NewAccrualWorker(logger, cfg, 10*time.Second, storage, accrualProvider)
	go accrualWorker.Run(ctx)

	return &OrderService{
		storage:   storage,
		validator: validator,
		cfg:       cfg,
		logger:    logger,
	}
}

// CreateOrder добавляет заказ пользователя в систему.
func (s *OrderService) CreateOrder(ctx context.Context, uid uuid.UUID, number string) error {
	const op = "service.order.CreateOrder"
	if !s.validator.Valid(number) {
		s.logger.With(slog.String("op", op)).Error("got invalid number")
		return domainerr.ErrInvalidInput
	}

	order, err := s.storage.GetOrderByNumber(ctx, number)
	if err != nil && !errors.Is(err, domainerr.ErrNoDataFound) {
		s.logger.With(slog.String("op", op)).Error("can not get order from DB", "err", err)
		return err
	}

	if err == nil {
		if order.UserID == uid {
			s.logger.With(slog.String("op", op)).Info("order already created by user")
			return domainerr.ErrAlreadyExists
		}
		s.logger.With(slog.String("op", op)).Info("order already created by another user")
		return domainerr.ErrConflict
	}

	return s.storage.CreateOrder(ctx, uid, number)
}

// GetOrders возвращает список заказов пользователя.
func (s *OrderService) GetOrders(ctx context.Context, uid uuid.UUID) ([]model.Order, error) {
	const op = "service.order.GetOrders"
	orders, err := s.storage.GetUserOrders(ctx, uid)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not get user's orders from DB", "err", err)
		return nil, err
	}

	return orders, nil
}
