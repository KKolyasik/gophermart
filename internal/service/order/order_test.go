package order

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/Kkolyasik/gophermart/internal/config"
	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/model"
	"github.com/Kkolyasik/gophermart/internal/service/order/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var ErrStorage = errors.New("storage error")

func NewMockConfig() *config.Config {
	return &config.Config{
		RunAddress:           "test",
		DatabaseURI:          "test",
		AccrualSystemAddress: "test",
		SecretKey:            "test",
	}
}

type OrderServiceSuite struct {
	suite.Suite

	provider  *mocks.MockAccrualProvider
	validator *mocks.MockOrderValidator
	storage   *mocks.MockOrderStorage
	service   *OrderService
}

func (s *OrderServiceSuite) SetupTest() {
	cfg := NewMockConfig()
	s.provider = mocks.NewMockAccrualProvider(s.T())
	s.validator = mocks.NewMockOrderValidator(s.T())
	s.storage = mocks.NewMockOrderStorage(s.T())
	s.service = NewOrderService(context.Background(), slog.New(slog.DiscardHandler), cfg, s.storage, s.provider, s.validator)
}

func (s *OrderServiceSuite) SetupSubTest() {
	cfg := NewMockConfig()
	s.provider = mocks.NewMockAccrualProvider(s.T())
	s.validator = mocks.NewMockOrderValidator(s.T())
	s.storage = mocks.NewMockOrderStorage(s.T())
	s.service = NewOrderService(context.Background(), slog.New(slog.DiscardHandler), cfg, s.storage, s.provider, s.validator)
}

func TestOrderService(t *testing.T) {
	suite.Run(t, new(OrderServiceSuite))
}

func (s *OrderServiceSuite) TestCreateOrder() {
	uid := uuid.New()
	tests := []struct {
		name        string
		setupMock   func()
		number      string
		expectedErr error
	}{
		{
			name: "create order",
			setupMock: func() {
				s.validator.EXPECT().Valid("123123").Return(true)
				s.storage.EXPECT().GetOrderByNumber(mock.Anything, "123123").
					Return(model.Order{}, domainerr.ErrNoDataFound)
				s.storage.EXPECT().CreateOrder(mock.Anything, uid, "123123").
					Return(nil)
			},
			number: "123123",
		},
		{
			name: "user already created",
			setupMock: func() {
				s.validator.EXPECT().Valid("123123").Return(true)
				s.storage.EXPECT().GetOrderByNumber(mock.Anything, "123123").
					Return(model.Order{UserID: uid}, nil)
				s.storage.AssertNotCalled(s.T(), "CreateOrder")
			},
			number:      "123123",
			expectedErr: domainerr.ErrAlreadyExists,
		},
		{
			name: "another user already created",
			setupMock: func() {
				s.validator.EXPECT().Valid("123123").Return(true)
				s.storage.EXPECT().GetOrderByNumber(mock.Anything, "123123").
					Return(model.Order{UserID: uuid.New()}, nil)
			},
			number:      "123123",
			expectedErr: domainerr.ErrConflict,
		},
		{
			name: "invalid number",
			setupMock: func() {
				s.validator.EXPECT().Valid("123123").Return(false)
			},
			number:      "123123",
			expectedErr: domainerr.ErrInvalidInput,
		},
		{
			name: "storage get error",
			setupMock: func() {
				s.validator.EXPECT().Valid("123123").Return(true)
				s.storage.EXPECT().GetOrderByNumber(mock.Anything, "123123").
					Return(model.Order{}, ErrStorage)
			},
			number:      "123123",
			expectedErr: ErrStorage,
		},
		{
			name: "storage create error",
			setupMock: func() {
				s.validator.EXPECT().Valid("123123").Return(true)
				s.storage.EXPECT().GetOrderByNumber(mock.Anything, "123123").
					Return(model.Order{}, domainerr.ErrNoDataFound)
				s.storage.EXPECT().CreateOrder(mock.Anything, uid, "123123").
					Return(ErrStorage)
			},
			number:      "123123",
			expectedErr: ErrStorage,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			err := s.service.CreateOrder(context.Background(), uid, tt.number)
			if tt.expectedErr != nil {
				s.Require().Error(err)
				s.ErrorIs(err, tt.expectedErr)
				return
			}
			s.Require().NoError(err)
		})
	}
}

func (s *OrderServiceSuite) TestGetOrders() {
	uid := uuid.New()
	uploadedAt := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		setupMock func()
		orders    []model.Order
		wantErr   bool
	}{
		{
			name: "success",
			setupMock: func() {
				s.storage.EXPECT().GetUserOrders(mock.Anything, uid).
					Return([]model.Order{
						{Number: "123123", UserID: uid, Status: model.Invalid, Accrual: nil, UploadedAt: uploadedAt},
						{Number: "321321", UserID: uid, Status: model.Processed, Accrual: float64ToPtr(500), UploadedAt: uploadedAt},
					}, nil)
			},
			orders: []model.Order{
				{Number: "123123", UserID: uid, Status: model.Invalid, Accrual: nil, UploadedAt: uploadedAt},
				{Number: "321321", UserID: uid, Status: model.Processed, Accrual: float64ToPtr(500), UploadedAt: uploadedAt},
			},
			wantErr: false,
		},
		{
			name: "storage error",
			setupMock: func() {
				s.storage.EXPECT().GetUserOrders(mock.Anything, uid).
					Return(nil, ErrStorage)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			orders, err := s.service.GetOrders(context.Background(), uid)
			if tt.wantErr {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)
			s.Equal(tt.orders, orders)
		})
	}
}

func float64ToPtr(n float64) *float64 {
	return &n
}
