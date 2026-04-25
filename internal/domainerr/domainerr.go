package domainerr

import (
	"errors"
	"fmt"
)

var (
	ErrAlreadyExists               = errors.New("already exist")
	ErrInvalidInput                = errors.New("invalid input")
	ErrNoDataFound                 = errors.New("no data found")
	ErrOrderAcceptedByUser         = errors.New("order accepted by user")
	ErrConflict                    = errors.New("order accepted by another user")
	ErrInsufficientFunds           = errors.New("insufficient funds")
	ErrExternalServiceNotAvailable = errors.New("external service not available")
)

// RetryAfterError описывает ограничение внешнего сервиса по частоте запросов.
type RetryAfterError struct {
	RetryAfter int
}

// NewRetryAfterError создает ошибку с временем повторной попытки в секундах.
func NewRetryAfterError(seconds int) *RetryAfterError {
	return &RetryAfterError{RetryAfter: seconds}
}

// Error возвращает текст ошибки для логирования и ответа клиенту.
func (e *RetryAfterError) Error() string {
	return fmt.Sprintf("retry after %d seconds", e.RetryAfter)
}
