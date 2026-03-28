package balance

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Kkolyasik/gophermart/internal/auth"
	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/model"
	"github.com/google/uuid"
)

// BalanceService описывает операции с балансом пользователя.
type BalanceService interface {
	GetBalance(ctx context.Context, uid uuid.UUID) (model.Balance, error)
	Withdraw(ctx context.Context, uid uuid.UUID, sum float64, number string) error
	GetWithdrawals(ctx context.Context, uid uuid.UUID) ([]model.Withdraw, error)
}

// BalanceHandler обрабатывает HTTP-запросы по балансу и списаниям.
type BalanceHandler struct {
	service BalanceService

	logger *slog.Logger
}

// NewBalanceHandler создает balance-handler.
func NewBalanceHandler(logger *slog.Logger, service BalanceService) *BalanceHandler {
	return &BalanceHandler{
		service: service,
		logger:  logger,
	}
}

// GetBalance возвращает текущий баланс и сумму списаний пользователя.
func (h *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	const op = "http.balance.GetBalance"

	uid := auth.GetUserID(r.Context())
	balance, err := h.service.GetBalance(r.Context(), uid)
	if err != nil {
		h.logger.With(slog.String("op", op)).Error("couldn't get balance", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(balance); err != nil {
		h.logger.With(slog.String("op", op)).Error("data could not be serialized", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreateWithdraw создает запрос на списание средств.
func (h *BalanceHandler) CreateWithdraw(w http.ResponseWriter, r *http.Request) {
	const op = "handler.balance.Withdraw"

	if r.Header.Get("Content-Type") != "application/json" {
		h.logger.With(slog.String("op", op)).Info("invalid content type in request")
		http.Error(w, "invalid content type", http.StatusBadRequest)
		return
	}

	defer func() {
		if err := r.Body.Close(); err != nil {
			h.logger.With(slog.String("op", op)).Error("body close error", "err", err)
		}
	}()
	dec := json.NewDecoder(r.Body)

	var withdraw model.WithdrawRequest
	if err := dec.Decode(&withdraw); err != nil {
		h.logger.With(slog.String("op", op)).Error("can not encode answer", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	uid := auth.GetUserID(r.Context())
	err := h.service.Withdraw(r.Context(), uid, withdraw.Sum, withdraw.Order)
	if err != nil {
		switch {
		case errors.Is(err, domainerr.ErrInsufficientFunds):
			h.logger.With(slog.String("op", op)).Info("user does not have enough funds", "err", err)
			http.Error(w, "not have enough funds", http.StatusPaymentRequired)
		case errors.Is(err, domainerr.ErrInvalidInput):
			h.logger.With(slog.String("op", op)).Info("got invalid input", "err", err)
			http.Error(w, "invalid input", http.StatusUnprocessableEntity)
		default:
			h.logger.With(slog.String("op", op)).Error("funds could not be debited", "err", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetWithdrawals возвращает историю списаний пользователя.
func (h *BalanceHandler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	const op = "http.balance.Withdraw"

	uid := auth.GetUserID(r.Context())
	withdrawals, err := h.service.GetWithdrawals(r.Context(), uid)
	if err != nil {
		h.logger.With(slog.String("op", op)).Error("couldn't get withdrawals", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(withdrawals) == 0 {
		h.logger.With(slog.String("op", op)).Info("user hasn't got withdrawals")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(withdrawals); err != nil {
		h.logger.With(slog.String("op", op)).Error("can not encode answer", "err", err)
		http.Error(w, "invalid request body", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
