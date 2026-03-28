package balance

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/Kkolyasik/gophermart/internal/model"
	"github.com/Kkolyasik/gophermart/internal/service/balance/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type BalanceServiceSuite struct {
	suite.Suite

	validator *mocks.MockOrderValidator
	storage   *mocks.MockBalanceStorage
	service   *BalanceService
}

func (s *BalanceServiceSuite) SetupTest() {
	s.validator = mocks.NewMockOrderValidator(s.T())
	s.storage = mocks.NewMockBalanceStorage(s.T())
	s.service = NewBalanceService(slog.New(slog.DiscardHandler), s.storage, s.validator)
}

func (s *BalanceServiceSuite) SetupSubTest() {
	s.validator = mocks.NewMockOrderValidator(s.T())
	s.storage = mocks.NewMockBalanceStorage(s.T())
	s.service = NewBalanceService(slog.New(slog.DiscardHandler), s.storage, s.validator)
}

func TestBalanceService(t *testing.T) {
	suite.Run(t, new(BalanceServiceSuite))
}

func (s *BalanceServiceSuite) TestWithdraw() {
	uid := uuid.New()
	tests := []struct {
		name      string
		setupMock func()
		sum       float64
		number    string
		wantErr   bool
	}{
		{
			name: "success",
			setupMock: func() {
				s.validator.EXPECT().Valid("123123").Return(true)
				s.storage.EXPECT().CreateWithdraw(mock.Anything, uid, 100.0, "123123").
					Return(nil)
			},
			sum:     100,
			number:  "123123",
			wantErr: false,
		},
		{
			name: "invalid number",
			setupMock: func() {
				s.validator.EXPECT().Valid("123123").Return(false)
			},
			sum:     100,
			number:  "123123",
			wantErr: true,
		},
		{
			name: "storage error",
			setupMock: func() {
				s.validator.EXPECT().Valid("123123").Return(true)
				s.storage.EXPECT().CreateWithdraw(mock.Anything, uid, 100.0, "123123").
					Return(errors.New("error"))
			},
			sum:     100,
			number:  "123123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			err := s.service.Withdraw(context.Background(), uid, tt.sum, tt.number)
			if tt.wantErr {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)
		})
	}
}

func (s *BalanceServiceSuite) TestGetBalance() {
	uid := uuid.New()
	tests := []struct {
		name      string
		setupMock func()
		current   float64
		withdrawn float64
		wantErr   bool
	}{
		{
			name: "success",
			setupMock: func() {
				s.storage.EXPECT().GetBalance(mock.Anything, uid).
					Return(model.Balance{UserId: uid, Current: 1000, Withdrawn: 500}, nil)
			},
			current:   1000,
			withdrawn: 500,
			wantErr:   false,
		},
		{
			name: "storage error",
			setupMock: func() {
				s.storage.EXPECT().GetBalance(mock.Anything, uid).
					Return(model.Balance{}, errors.New("error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			balance, err := s.service.GetBalance(context.Background(), uid)
			if tt.wantErr {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)
			s.Equal(uid, balance.UserId)
			s.Equal(tt.current, balance.Current)
			s.Equal(tt.withdrawn, balance.Withdrawn)
		})
	}
}

func (s *BalanceServiceSuite) TestGetWithdrawals() {
	uid := uuid.New()
	processedAt := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		setupMock   func()
		withdrawals []model.Withdraw
		wantErr     bool
	}{
		{
			name: "success",
			setupMock: func() {
				s.storage.EXPECT().GetWithdrawals(mock.Anything, uid).
					Return([]model.Withdraw{
						{UserId: uid, Order: "123123", Sum: 100, ProcessedAt: processedAt},
						{UserId: uid, Order: "321321", Sum: 200, ProcessedAt: processedAt},
					}, nil)
			},
			withdrawals: []model.Withdraw{
				{UserId: uid, Order: "123123", Sum: 100, ProcessedAt: processedAt},
				{UserId: uid, Order: "321321", Sum: 200, ProcessedAt: processedAt},
			},
			wantErr: false,
		},
		{
			name: "storage error",
			setupMock: func() {
				s.storage.EXPECT().GetWithdrawals(mock.Anything, uid).
					Return(nil, errors.New("error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMock()

			withdrawals, err := s.service.GetWithdrawals(context.Background(), uid)
			if tt.wantErr {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)
			s.Equal(tt.withdrawals, withdrawals)
		})
	}
}
