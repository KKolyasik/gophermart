package balance

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Kkolyasik/gophermart/internal/auth"
	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/handler/balance/mocks"
	"github.com/Kkolyasik/gophermart/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type BalanceHandlerSuite struct {
	suite.Suite

	service *mocks.MockBalanceService
	handler *BalanceHandler
}

func (s *BalanceHandlerSuite) SetupTest() {
	s.service = mocks.NewMockBalanceService(s.T())
	s.handler = NewBalanceHandler(slog.New(slog.DiscardHandler), s.service)
}

func (s *BalanceHandlerSuite) SetupSubTest() {
	s.service = mocks.NewMockBalanceService(s.T())
	s.handler = NewBalanceHandler(slog.New(slog.DiscardHandler), s.service)
}

func TestBalanceHandler(t *testing.T) {
	suite.Run(t, new(BalanceHandlerSuite))
}

func (s *BalanceHandlerSuite) TestGetBalance() {
	uid := uuid.New()
	tests := []struct {
		name       string
		setupMock  func()
		wantStatus int
		body       string
	}{
		{
			name: "success",
			setupMock: func() {
				s.service.EXPECT().GetBalance(mock.Anything, uid).Return(
					model.Balance{UserId: uid, Current: 1000, Withdrawn: 500}, nil,
				)
			},
			wantStatus: http.StatusOK,
			body:       `{"current":1000,"withdrawn":500}`,
		},
		{
			name: "service error",
			setupMock: func() {
				s.service.EXPECT().GetBalance(mock.Anything, uid).Return(
					model.Balance{}, errors.New("service error"),
				)
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			ctx := context.WithValue(context.Background(), auth.UserIDKey, uid)
			r := httptest.NewRequest(http.MethodGet, "/balance", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			s.handler.GetBalance(w, r)

			s.Require().Equal(tt.wantStatus, w.Result().StatusCode)
			if tt.wantStatus == http.StatusOK {
				body := w.Body.String()
				s.JSONEq(tt.body, body)
			}
		})
	}
}

func (s *BalanceHandlerSuite) TestCreateWithdraw() {
	uid := uuid.New()
	tests := []struct {
		name       string
		setupMock  func()
		wantStatus int
		body       string
	}{
		{
			name: "success",
			setupMock: func() {
				s.service.EXPECT().Withdraw(mock.Anything, uid, 100.0, "100").
					Return(nil)
			},
			wantStatus: http.StatusOK,
			body:       `{"order":"100","sum":100}`,
		},
		{
			name: "not enough funds",
			setupMock: func() {
				s.service.EXPECT().Withdraw(mock.Anything, uid, 100.0, "100").
					Return(domainerr.ErrInsufficientFunds)
			},
			wantStatus: http.StatusPaymentRequired,
			body:       `{"order":"100","sum":100}`,
		},
		{
			name: "invalid input",
			setupMock: func() {
				s.service.EXPECT().Withdraw(mock.Anything, uid, 100.0, "100").
					Return(domainerr.ErrInvalidInput)
			},
			wantStatus: http.StatusUnprocessableEntity,
			body:       `{"order":"100","sum":100}`,
		},
		{
			name: "error",
			setupMock: func() {
				s.service.EXPECT().Withdraw(mock.Anything, uid, 100.0, "100").
					Return(errors.New("error"))
			},
			wantStatus: http.StatusInternalServerError,
			body:       `{"order":"100","sum":100}`,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			ctx := context.WithValue(context.Background(), auth.UserIDKey, uid)
			r := httptest.NewRequest(http.MethodPost, "/balance/withdraw", strings.NewReader(tt.body)).WithContext(ctx)
			w := httptest.NewRecorder()

			r.Header.Set("Content-Type", "application/json")

			s.handler.CreateWithdraw(w, r)

			s.Require().Equal(tt.wantStatus, w.Result().StatusCode)
		})
	}
}

func (s *BalanceHandlerSuite) TestGetWithdrawals() {
	uid := uuid.New()
	processedAt := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	want := []model.Withdraw{
		{UserId: uid, Order: "100", Sum: 100, ProcessedAt: processedAt},
		{UserId: uid, Order: "200", Sum: 200, ProcessedAt: processedAt},
	}
	tests := []struct {
		name       string
		setupMock  func()
		wantStatus int
	}{
		{
			name: "success",
			setupMock: func() {
				s.service.EXPECT().GetWithdrawals(mock.Anything, uid).
					Return([]model.Withdraw{
						{UserId: uid, Order: "100", Sum: 100, ProcessedAt: processedAt},
						{UserId: uid, Order: "200", Sum: 200, ProcessedAt: processedAt},
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "no withdrawals",
			setupMock: func() {
				s.service.EXPECT().GetWithdrawals(mock.Anything, uid).
					Return(nil, nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "error",
			setupMock: func() {
				s.service.EXPECT().GetWithdrawals(mock.Anything, uid).
					Return(nil, errors.New("err"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			ctx := context.WithValue(context.Background(), auth.UserIDKey, uid)
			r := httptest.NewRequest(http.MethodGet, "/withdrawals", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			s.handler.GetWithdrawals(w, r)

			s.Require().Equal(tt.wantStatus, w.Result().StatusCode)
			if tt.wantStatus == http.StatusOK {
				wantBody, err := json.Marshal(want)
				s.Require().NoError(err)
				s.JSONEq(string(wantBody), w.Body.String())
			}
		})
	}
}
