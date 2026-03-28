package order

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
	"github.com/Kkolyasik/gophermart/internal/handler/order/mocks"
	"github.com/Kkolyasik/gophermart/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type OrderHanlerSuite struct {
	suite.Suite

	service *mocks.MockOrderService
	handler *OrderHandler
}

func (s *OrderHanlerSuite) SetupTest() {
	s.service = mocks.NewMockOrderService(s.T())
	s.handler = NewOrderHandler(slog.New(slog.DiscardHandler), s.service)
}

func (s *OrderHanlerSuite) SetupSubTest() {
	s.service = mocks.NewMockOrderService(s.T())
	s.handler = NewOrderHandler(slog.New(slog.DiscardHandler), s.service)
}

func TestOrderHandler(t *testing.T) {
	suite.Run(t, new(OrderHanlerSuite))
}

func (s *OrderHanlerSuite) TestCreateOrder() {
	uid := uuid.New()
	tests := []struct {
		name       string
		setupMock  func()
		body       string
		wantStatus int
	}{
		{
			name: "success",
			setupMock: func() {
				s.service.EXPECT().CreateOrder(mock.Anything, uid, "100").
					Return(nil)
			},
			body:       "100",
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "empty number",
			setupMock:  func() { s.service.AssertNotCalled(s.T(), "CreateOrder") },
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid input",
			setupMock: func() {
				s.service.EXPECT().CreateOrder(mock.Anything, uid, "100").
					Return(domainerr.ErrInvalidInput)
			},
			body:       "100",
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "invalid input",
			setupMock: func() {
				s.service.EXPECT().CreateOrder(mock.Anything, uid, "100").
					Return(domainerr.ErrAlreadyExists)
			},
			body:       "100",
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid input",
			setupMock: func() {
				s.service.EXPECT().CreateOrder(mock.Anything, uid, "100").
					Return(domainerr.ErrConflict)
			},
			body:       "100",
			wantStatus: http.StatusConflict,
		},
		{
			name: "invalid input",
			setupMock: func() {
				s.service.EXPECT().CreateOrder(mock.Anything, uid, "100").
					Return(errors.New("error"))
			},
			body:       "100",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			ctx := context.WithValue(context.Background(), auth.UserIDKey, uid)
			r := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(tt.body)).WithContext(ctx)
			r.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()

			s.handler.CreateOrder(w, r)

			s.Require().Equal(tt.wantStatus, w.Result().StatusCode)
		})
	}
}

func (s *OrderHanlerSuite) TestCalculatePoints() {
	type body struct {
		Number     string       `json:"number"`
		Status     model.Status `json:"status"`
		Accrual    *float64     `json:"accrual,omitempty"`
		UploadedAt time.Time    `json:"uploaded_at"`
	}

	uid := uuid.New()
	uploadedAt := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		setupMock  func()
		body       []body
		wantStatus int
	}{
		{
			name: "success",
			setupMock: func() {
				s.service.EXPECT().GetOrders(mock.Anything, uid).
					Return([]model.Order{
						{Number: "100", UserID: uid, Status: model.New, Accrual: nil, UploadedAt: uploadedAt},
						{Number: "100", UserID: uid, Status: model.Processing, Accrual: nil, UploadedAt: uploadedAt},
						{Number: "100", UserID: uid, Status: model.Invalid, Accrual: nil, UploadedAt: uploadedAt},
						{Number: "100", UserID: uid, Status: model.Processed, Accrual: float64ToPtr(500), UploadedAt: uploadedAt},
					}, nil)
			},
			body: []body{
				{Number: "100", Status: model.New, Accrual: nil, UploadedAt: uploadedAt},
				{Number: "100", Status: model.Processing, Accrual: nil, UploadedAt: uploadedAt},
				{Number: "100", Status: model.Invalid, Accrual: nil, UploadedAt: uploadedAt},
				{Number: "100", Status: model.Processed, Accrual: float64ToPtr(500), UploadedAt: uploadedAt},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "error",
			setupMock: func() {
				s.service.EXPECT().GetOrders(mock.Anything, uid).
					Return(nil, errors.New("error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "empty orders",
			setupMock: func() {
				s.service.EXPECT().GetOrders(mock.Anything, uid).
					Return(nil, nil)
			},
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			ctx := context.WithValue(context.Background(), auth.UserIDKey, uid)
			r := httptest.NewRequest(http.MethodPost, "/orders", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			s.handler.CalculatePoints(w, r)

			s.Require().Equal(tt.wantStatus, w.Result().StatusCode)
			if tt.wantStatus == http.StatusOK {
				wantBody, err := json.Marshal(tt.body)
				s.Require().NoError(err)
				s.JSONEq(string(wantBody), w.Body.String())
			}
		})
	}
}

func float64ToPtr(n float64) *float64 {
	return &n
}
