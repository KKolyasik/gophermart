package order

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Kkolyasik/gophermart/internal/auth"
	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/model"
	"github.com/google/uuid"
)

const conentType = "text/plain"

// OrderService описывает операции с заказами для handler-слоя.
type OrderService interface {
	CreateOrder(ctx context.Context, uid uuid.UUID, number string) error
	GetOrders(ctx context.Context, uid uuid.UUID) ([]model.Order, error)
}

// OrderHandler обрабатывает HTTP-запросы по заказам пользователя.
type OrderHandler struct {
	service OrderService

	logger *slog.Logger
}

// NewOrderHandler создает order-handler.
func NewOrderHandler(logger *slog.Logger, service OrderService) *OrderHandler {
	return &OrderHandler{
		service: service,
		logger:  logger,
	}
}

// CreateOrder принимает номер заказа и отправляет его в обработку.
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	const op = "handler.order.CreateOrder"

	if r.Header.Get("Content-Type") != conentType {
		h.logger.With(slog.String("op", op)).Info("invalid content type in request")
		http.Error(w, "invalid content type", http.StatusBadRequest)
		return
	}

	body := http.MaxBytesReader(w, r.Body, 8<<10)
	data, err := io.ReadAll(body)
	if err != nil {
		h.logger.With(slog.String("op", op)).Error("can not read body", "err", err)
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}

	number := strings.TrimSpace(string(data))
	if number == "" {
		h.logger.With(slog.String("op", op)).Info("got empty number")
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	uid := auth.GetUserID(r.Context())
	err = h.service.CreateOrder(r.Context(), uid, number)
	if err != nil {
		switch {
		case errors.Is(err, domainerr.ErrInvalidInput):
			h.logger.With(slog.String("op", op)).Info("got invalid input", "err", err)
			http.Error(w, "invalid input", http.StatusUnprocessableEntity)
		case errors.Is(err, domainerr.ErrAlreadyExists):
			h.logger.With(slog.String("op", op)).Info("user already has order with this number", "err", err)
			w.WriteHeader(http.StatusOK)
		case errors.Is(err, domainerr.ErrConflict):
			h.logger.With(slog.String("op", op)).Info("another user already has order with this number", "err", err)
			w.WriteHeader(http.StatusConflict)
		default:
			h.logger.With(slog.String("op", op)).Error("can not get create or get order from DB", "err", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// CalculatePoints возвращает список заказов пользователя.
func (h *OrderHandler) CalculatePoints(w http.ResponseWriter, r *http.Request) {
	const op = "handler.order.CalculatePoints"

	uid := auth.GetUserID(r.Context())

	orders, err := h.service.GetOrders(r.Context(), uid)
	if err != nil {
		h.logger.With(slog.String("op", op)).Error("can not get user's orders from DB", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		h.logger.With(slog.String("op", op)).Info("user hasn't got orders")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(orders); err != nil {
		h.logger.With(slog.String("op", op)).Error("can not encode answer", "err", err)
		http.Error(w, "internal server errorx", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
