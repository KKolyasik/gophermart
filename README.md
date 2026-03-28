# Gophermart

Небольшой HTTP-сервис лояльности: регистрация/логин, загрузка заказов, баланс и вывод средств.

## Что умеет

- Регистрация и авторизация пользователя (JWT в cookie `Authorization`)
- Загрузка заказов
- Просмотр баланса
- Списание баллов
- История списаний

## Требования

- Go `1.25+`
- PostgreSQL `14+`
- Запущенный accrual-сервис (адрес в `ACCRUAL_SYSTEM_ADDRESS`)

## Конфигурация

Через env:

- `DATABASE_URI` — строка подключения к PostgreSQL
- `ACCRUAL_SYSTEM_ADDRESS` — адрес accrual-сервиса
- `SECRET_KEY` — ключ подписи JWT (если не задан, сгенерируется)
- `RUN_ADDRESS` — поддерживается в config, но сейчас сервер в коде

## Быстрый запуск

1. Поднять БД и применить миграцию:

```bash
psql "$DATABASE_URI" -f migrations/000001_initial_schema.up.sql
```

2. Выставить переменные окружения:

```bash
export DATABASE_URI='postgres://user:password@localhost:5432/gophermart?sslmode=disable'
export ACCRUAL_SYSTEM_ADDRESS='http://localhost:8081'
export SECRET_KEY='super-secret-key'
```

3. Запустить сервис:

```bash
go run ./cmd
```

Сервис будет доступен на `http://localhost:8080`.

## Основные эндпоинты

- `POST /api/user/register`
- `POST /api/user/login`
- `POST /api/user/orders`
- `GET /api/user/orders`
- `GET /api/user/balance`
- `POST /api/user/balance/withdraw`
- `GET /api/user/withdrawals`

## Тесты

```bash
go test ./...
```
