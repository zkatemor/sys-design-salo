# ДЗ-17. Контрактные тесты API

| Файл | Описание |
|------|----------|
| [TASK.md](TASK.md) | Условие |
| [CONTRACT.md](CONTRACT.md) | Процесс изменения контракта |
| [api/openapi.yaml](api/openapi.yaml) | OpenAPI — источник истины |
| [internal/contract/](internal/contract/) | Контрактные тесты (≥5 эндпоинтов) |

## Подход

**OpenAPI 3 + kin-openapi** — ответы валидируются против схемы в `api/openapi.yaml`.

## Покрытые эндпоинты

- `GET /healthz`
- `POST /users`, `GET /users/{id}`
- `POST /orders`, `GET /orders/{id}`

## Запуск

```bash
cd sys-design-salo/hw/17

go test -race ./...
go test -race -run '^TestContract_' ./internal/contract/...

go run ./cmd/api   # :8080
```

## CI

Workflow в корне репозитория **sys-design-salo**: [`.github/workflows/hw17-contract.yml`](../../.github/workflows/hw17-contract.yml) (шаг **`contract`**).

После push: **Actions → hw17-contract**. Если runs пусто — **Run workflow** (кнопка справа, `workflow_dispatch`).
