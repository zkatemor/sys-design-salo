# ДЗ-17. Контрактные тесты

## Что дано

Сервис на чистом `net/http` с 6 эндпоинтами:

* `GET  /healthz`
* `GET  /users`
* `POST /users` — `{name, email}` → 201 + User
* `GET  /users/{id}` — 200 / 404
* `POST /orders` — `{user_id, items}` → 201 + Order
* `GET  /orders/{id}` — 200 / 404

В `api/openapi.yaml` описана только заготовка с `/healthz`. В
`internal/contract/contract_test.go` — 3 заскипанных теста-каркаса.
В `.github/workflows/ci.yml` — базовый pipeline с TODO под контрактный шаг.
В `CONTRACT.md` — шаблон процесса.

## Задача

### 1. Контрактные тесты

Покрыть **минимум 3 эндпоинта** контрактными тестами. Допустимые подходы
(один на выбор, обоснование в `CONTRACT.md`):

* OpenAPI + `kin-openapi` или `libopenapi`: спека `api/openapi.yaml` —
  источник истины, тест валидирует ответ против неё.
* JSON Schema файлы + `xeipuuv/gojsonschema`.
* Pact (`pact-foundation/pact-go`).
* Хардкод через структуры + `json.Decoder.DisallowUnknownFields`.

Тест должен падать (с **внятным** сообщением) при:
* удалении поля,
* переименовании поля,
* смене типа,
* смене статус-кода,
* смене Content-Type.

### 2. CI

Доработать `.github/workflows/ci.yml`:

* отдельный шаг `contract` с именем, видным в логах;
* (опционально) линт спеки через `spectral-action`;
* шаг должен падать на breaking-изменении контракта.

Проверить, что pipeline зелёный на `main` и красный, если намеренно
сломать контракт (сделайте такой PR в порядке демонстрации, после
ревью отмените).

### 3. Процесс

Заполнить `CONTRACT.md`:

* классификация изменений (breaking / non-breaking);
* владелец контракта и список потребителей;
* пошаговый процесс изменения;
* версионирование;
* что делать, если контрактный тест упал.

## Запуск

```bash
go test -race ./...                       # все тесты
go test -race ./internal/contract/...     # только контрактные
go run ./cmd/api                          # сервис на :8080
```

## Критерии приёмки

* Покрыто ≥3 эндпоинта.
* `go test -race ./internal/contract/...` зелёный, ни одного `t.Skip`.
* Тесты падают с понятным сообщением, если убрать поле из response
  (проверьте — намеренно сломайте `internal/api/handler.go` и убедитесь).
* CI шаг `contract` отдельный и виден в Actions UI.
* `CONTRACT.md` заполнен — все TODO заменены на реальный текст.
