package model

import (
	"time"

	"github.com/google/uuid"
)

// Status описывает текущее состояние заказа.
type Status string

const (
	New        Status = "NEW"
	Processing Status = "PROCESSING"
	Registered Status = "REGISTERED"
	Invalid    Status = "INVALID"
	Processed  Status = "PROCESSED"
)

// Credentials хранит логин и пароль пользователя.
type Credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Order описывает заказ пользователя и его состояние.
type Order struct {
	Number     string    `json:"number"`
	UserID     uuid.UUID `json:"-"`
	Status     Status    `json:"status"`
	Accrual    *float64  `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// AccrualResponse содержит ответ от системы начислений.
type AccrualResponse struct {
	Order   string   `json:"order"`
	Status  Status   `json:"status"`
	Accrual *float64 `json:"accrual"`
}

// Balance хранит текущий баланс и сумму списаний пользователя.
type Balance struct {
	UserId    uuid.UUID `json:"-"`
	Current   float64   `json:"current"`
	Withdrawn float64   `json:"withdrawn"`
}

// WithdrawRequest описывает запрос на списание баланса.
type WithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

// Withdraw описывает операцию списания пользователя.
type Withdraw struct {
	UserId      uuid.UUID `json:"-"`
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}
