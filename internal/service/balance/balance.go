package balance

import (
	"context"
	"log/slog"

	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/model"
	"github.com/google/uuid"
)

// OrderValidator проверяет номер заказа.
type OrderValidator interface {
	Valid(number string) bool
}

// BalanceStorage описывает доступ к данным баланса.
type BalanceStorage interface {
	GetBalance(ctx context.Context, uid uuid.UUID) (model.Balance, error)
	CreateWithdraw(ctx context.Context, uid uuid.UUID, sum float64, number string) error
	GetWithdrawals(ctx context.Context, uid uuid.UUID) ([]model.Withdraw, error)
}

// BalanceService реализует бизнес-логику операций по балансу.
type BalanceService struct {
	storage   BalanceStorage
	validator OrderValidator

	logger *slog.Logger
}

// NewBalanceService создает balance-сервис.
func NewBalanceService(logger *slog.Logger, storage BalanceStorage, validator OrderValidator) *BalanceService {
	return &BalanceService{
		storage:   storage,
		validator: validator,
		logger:    logger,
	}
}

// GetBalance возвращает баланс пользователя.
func (s *BalanceService) GetBalance(ctx context.Context, uid uuid.UUID) (model.Balance, error) {
	return s.storage.GetBalance(ctx, uid)
}

// Withdraw создает операцию списания баланса пользователя.
func (s *BalanceService) Withdraw(ctx context.Context, uid uuid.UUID, sum float64, number string) error {
	const op = "service.balance.Withdraw"

	if !s.validator.Valid(number) {
		s.logger.With(slog.String("op", op)).Info("got invalid number")
		return domainerr.ErrInvalidInput
	}
	err := s.storage.CreateWithdraw(ctx, uid, sum, number)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("error when debiting points")
		return err
	}
	return nil
}

// GetWithdrawals возвращает историю списаний пользователя.
func (s *BalanceService) GetWithdrawals(ctx context.Context, uid uuid.UUID) ([]model.Withdraw, error) {
	return s.storage.GetWithdrawals(ctx, uid)
}
