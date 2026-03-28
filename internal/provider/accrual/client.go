package accrual

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Kkolyasik/gophermart/internal/config"
	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/model"
)

const calcucationSystemPath = "/api/orders/"

// Provider инкапсулирует HTTP-клиент системы начислений.
type Provider struct {
	client *http.Client
	cfg    *config.Config
	logger *slog.Logger
}

// NewProvider создает клиент для обращения к системе начислений.
func NewProvider(cfg *config.Config, logger *slog.Logger) *Provider {
	return &Provider{
		client: &http.Client{},
		cfg:    cfg,
		logger: logger,
	}
}

// GetOrderAccrual запрашивает данные начислений по номеру заказа.
func (p *Provider) GetOrderAccrual(ctx context.Context, orderNumber string) (model.AccrualResponse, error) {
	const op = "provider.accrual.GetOrderAccrual"

	url, err := url.JoinPath(p.cfg.AccrualSystemAddress, calcucationSystemPath, orderNumber)
	if err != nil {
		return model.AccrualResponse{}, err
	}

	response, err := p.client.Get(url)
	if err != nil {
		return model.AccrualResponse{}, err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			p.logger.With(slog.String("op", op)).Error("body close error", "err", err)
		}
	}()

	switch response.StatusCode {
	case http.StatusOK:
		accrualResponse, err := readBody(response.Body)
		if err != nil {
			return model.AccrualResponse{}, err
		}

		return accrualResponse, nil
	case http.StatusNoContent:
		return model.AccrualResponse{}, domainerr.ErrNoDataFound
	case http.StatusTooManyRequests:
		retryAfter, err := strconv.Atoi(response.Header.Get("Retry-After"))
		if err != nil {
			return model.AccrualResponse{}, domainerr.NewRetryAfterError(retryAfter)
		}
		return model.AccrualResponse{}, err
	default:
		return model.AccrualResponse{}, domainerr.ErrExternalServiceNotAvailable
	}
}

func readBody(data io.Reader) (model.AccrualResponse, error) {
	var accrualResponse model.AccrualResponse
	decoder := json.NewDecoder(data)
	if err := decoder.Decode(&accrualResponse); err != nil {
		return model.AccrualResponse{}, err
	}

	return accrualResponse, nil
}
