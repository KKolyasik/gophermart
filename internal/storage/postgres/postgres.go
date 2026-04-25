package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Kkolyasik/gophermart/internal/domainerr"
	"github.com/Kkolyasik/gophermart/internal/model"
	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Storage реализует доступ к данным через PostgreSQL.
type Storage struct {
	pool *pgxpool.Pool

	logger *slog.Logger
}

// NewStorage создает подключение к PostgreSQL и возвращает storage-объект.
func NewStorage(ctx context.Context, connURL string, logger *slog.Logger) (*Storage, error) {
	pool, err := pgxpool.New(ctx, connURL)
	if err != nil {
		return nil, fmt.Errorf("connect to db: %w", err)
	}

	storage := &Storage{
		pool:   pool,
		logger: logger,
	}
	if err := storage.initSchema(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return storage, nil
}

func (s *Storage) initSchema(ctx context.Context) error {
	const schema = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    login      VARCHAR(256) NOT NULL UNIQUE,
    password   VARCHAR(256) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS balances (
    user_id   UUID PRIMARY KEY REFERENCES users(id),
    current   NUMERIC(15,2) NOT NULL DEFAULT 0,
    withdrawn NUMERIC(15,2) NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS orders (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id),
    number      VARCHAR(64) NOT NULL UNIQUE,
    status      VARCHAR(16) NOT NULL DEFAULT 'NEW',
    accrual     NUMERIC(15,2),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);

CREATE TABLE IF NOT EXISTS withdrawals (
    id           BIGSERIAL PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id),
    order_number VARCHAR(64) NOT NULL,
    sum          NUMERIC(15,2) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_withdrawals_user_id ON withdrawals(user_id);
`
	_, err := s.pool.Exec(ctx, schema)
	return err
}

// Register создает пользователя и инициализирует его баланс.
func (s *Storage) Register(ctx context.Context, login, hash string) (uuid.UUID, error) {
	const op = "storage.postgres.Register"

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not begin tx", "err", err)
		return uuid.Nil, err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			s.logger.With(slog.String("op", op)).Error("rollback error", "err", err)
		}
	}()

	query, args, err := sq.Insert("users").
		Columns("login", "password").
		Values(login, hash).
		Suffix("RETURNING id").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return uuid.Nil, err
	}

	var uid uuid.UUID
	err = tx.QueryRow(ctx, query, args...).Scan(&uid)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return uuid.Nil, domainerr.ErrAlreadyExists
		}
		s.logger.With(slog.String("op", op)).Error("can not create user", "err", err)
		return uuid.Nil, err
	}

	query, args, err = sq.Insert("balances").
		Columns("user_id", "current", "withdrawn").
		Values(uid, 0, 0).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return uuid.Nil, err
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create balance", "err", err)
		return uuid.Nil, err
	}

	return uid, tx.Commit(ctx)
}

// Login возвращает uid и хеш пароля пользователя по логину.
func (s *Storage) Login(ctx context.Context, login string) (uuid.UUID, string, error) {
	const op = "storage.postgres.Login"

	query, args, err := sq.Select("id", "password").
		From("users").
		Where(sq.Eq{"login": login}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return uuid.Nil, "", err
	}

	row := s.pool.QueryRow(ctx, query, args...)

	var (
		uid      uuid.UUID
		password string
	)
	err = row.Scan(&uid, &password)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", domainerr.ErrInvalidInput
		}
		s.logger.With(slog.String("op", op)).Error("can not scan uid or password", "err", err)
		return uuid.Nil, "", err
	}

	return uid, password, nil
}

// GetOrderByNumber возвращает заказ по его номеру.
func (s *Storage) GetOrderByNumber(ctx context.Context, number string) (model.Order, error) {
	const op = "storage.postgres.GetOrderByNumber"

	query, args, err := sq.
		Select("number", "user_id", "status", "accrual", "uploaded_at").
		From("orders").Where(sq.Eq{"number": number}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return model.Order{}, err
	}

	var order model.Order
	row := s.pool.QueryRow(ctx, query, args...)
	err = row.Scan(&order.Number, &order.UserID, &order.Status, &order.Accrual, &order.UploadedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Order{}, domainerr.ErrNoDataFound
		}
		s.logger.With(slog.String("op", op)).Error("can not scan order", "err", err)
		return model.Order{}, err
	}

	return order, nil
}

// CreateOrder добавляет новый заказ пользователя.
func (s *Storage) CreateOrder(ctx context.Context, uid uuid.UUID, number string) error {
	const op = "storage.postgres.CreateOrder"

	query, args, err := sq.Insert("orders").
		Columns("user_id", "number").
		Values(uid, number).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return err
	}

	_, err = s.pool.Exec(ctx, query, args...)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can create order in DB", "err", err)
		return err
	}
	return nil
}

// GetUnprocessedOrders возвращает заказы, требующие обновления статуса.
func (s *Storage) GetUnprocessedOrders(ctx context.Context) ([]model.Order, error) {
	const op = "storage.postgres.GetUnprocessed"

	query, args, err := sq.
		Select("user_id", "number", "status", "accrual", "uploaded_at").
		From("orders").Where(sq.NotEq{"status": []model.Status{model.Processed, model.Invalid}}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return nil, err
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not execute sql query", "err", err)
		return nil, err
	}

	var orders []model.Order
	for rows.Next() {
		var order model.Order
		err := rows.Scan(&order.UserID, &order.Number, &order.Status, &order.Accrual, &order.UploadedAt)

		if err != nil {
			s.logger.With(slog.String("op", op)).Error("can not scan order", "err", err)
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// CompleteOrder обновляет статус заказа и начисляет баланс при наличии accrual.
func (s *Storage) CompleteOrder(ctx context.Context, uid uuid.UUID, number string, status model.Status, accrual *float64) error {
	const op = "storage.postgres.CompleteOrder"
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not begin tx", "err", err)
		return err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			s.logger.With(slog.String("op", op)).Error("rollback error", "err", err)
		}
	}()

	query, args, err := sq.Update("orders").
		SetMap(map[string]any{"status": status, "accrual": accrual}).
		Where(sq.Eq{"number": number}).
		PlaceholderFormat(sq.Dollar).
		ToSql()

	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return err
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can update order in DB", "err", err)
		return err
	}

	if accrual != nil {
		query, args, err = sq.Update("balances").
			Set("current", sq.Expr("current + ?", *accrual)).
			Where(sq.Eq{"user_id": uid}).
			PlaceholderFormat(sq.Dollar).
			ToSql()

		if err != nil {
			s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
			return err
		}

		_, err = tx.Exec(ctx, query, args...)
		if err != nil {
			s.logger.With(slog.String("op", op)).Error("can update balance in DB", "err", err)
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetUserOrders возвращает все заказы пользователя.
func (s *Storage) GetUserOrders(ctx context.Context, uid uuid.UUID) ([]model.Order, error) {
	const op = "storage.postgres.GetUserOrders"

	query, args, err := sq.
		Select("user_id", "number", "status", "accrual", "uploaded_at").
		From("orders").
		Where(sq.Eq{"user_id": uid}).
		OrderBy("uploaded_at ASC").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return nil, err
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not execute sql query", "err", err)
		return nil, err
	}

	var orders []model.Order
	for rows.Next() {
		var order model.Order
		err := rows.Scan(&order.UserID, &order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			s.logger.With(slog.String("op", op)).Error("can not scan order", "err", err)
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// GetBalance возвращает текущий баланс пользователя.
func (s *Storage) GetBalance(ctx context.Context, uid uuid.UUID) (model.Balance, error) {
	const op = "storage.postgres.GetBalance"

	query, args, err := sq.Select("user_id", "current", "withdrawn").
		From("balances").
		Where(sq.Eq{"user_id": uid}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return model.Balance{}, err
	}

	var balance model.Balance
	err = s.pool.QueryRow(ctx, query, args...).Scan(&balance.UserId, &balance.Current, &balance.Withdrawn)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not get balance", "err", err)
		return model.Balance{}, err
	}
	return balance, nil
}

// CreateWithdraw создает списание и обновляет баланс в одной транзакции.
func (s *Storage) CreateWithdraw(ctx context.Context, uid uuid.UUID, sum float64, number string) error {
	const op = "storage.postgres.CreateWithdraw"
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not begin tx", "err", err)
		return err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			s.logger.With(slog.String("op", op)).Error("rollback error", "err", err)
		}
	}()

	var current float64
	query, args, err := sq.Select("current").
		From("balances").Where(sq.Eq{"user_id": uid}).
		Suffix("FOR UPDATE").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return err
	}
	err = tx.QueryRow(ctx, query, args...).Scan(&current)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not get balance", "err", err)
		return err
	}

	if current < sum {
		return domainerr.ErrInsufficientFunds
	}

	query, args, err = sq.Update("balances").SetMap(map[string]interface{}{
		"current":   sq.Expr("current - ?", sum),
		"withdrawn": sq.Expr("withdrawn + ?", sum),
	}).Where(sq.Eq{"user_id": uid}).PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return err
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can update balance in DB", "err", err)
		return err
	}

	query, args, err = sq.Insert("withdrawals").
		Columns("user_id", "order_number", "sum").
		Values(uid, number, sum).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return err
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can update withdrawals in DB", "err", err)
		return err
	}

	return tx.Commit(ctx)
}

// GetWithdrawals возвращает историю списаний пользователя.
func (s *Storage) GetWithdrawals(ctx context.Context, uid uuid.UUID) ([]model.Withdraw, error) {
	const op = "storage.postgres.GetWithdrawals"

	query, args, err := sq.Select("user_id", "order_number", "sum", "processed_at").
		From("withdrawals").
		Where(sq.Eq{"user_id": uid}).
		OrderBy("processed_at ASC").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not create sql query", "err", err)
		return nil, err
	}

	var withdrawals []model.Withdraw
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		s.logger.With(slog.String("op", op)).Error("can not query withdrawals", "err", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var withdraw model.Withdraw
		err := rows.Scan(&withdraw.UserId, &withdraw.Order, &withdraw.Sum, &withdraw.ProcessedAt)
		if err != nil {
			s.logger.With(slog.String("op", op)).Error("can not scan withdraw", "err", err)
			return nil, err
		}

		withdrawals = append(withdrawals, withdraw)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return withdrawals, nil
}
