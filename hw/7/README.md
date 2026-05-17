# ДЗ-7. Circuit Breaker

Учебный сервис на Go: API-прокси (`:8080`) с circuit breaker на вызовы flaky upstream (`:9090`).

| Файл | Описание |
|------|----------|
| [TASK.md](TASK.md) | Условие задания |
| [hw_7_circuit-breaker-adr.md](hw_7_circuit-breaker-adr.md) | ADR: обоснование решения и параметров |

## Требования

- Go 1.23+ (`go version`)
- Опционально: [mise](https://mise.jdx.dev/) – версия Go из `mise.toml`

```bash
cd sys-design-salo/hw/7
mise install   # если используете mise
```

## Тесты

Из корня задания:

```bash
cd sys-design-salo/hw/7

# все пакеты + детектор гонок (критерий приёмки)
go test -race ./...

# только circuit breaker
go test -race ./internal/breaker/...

# один тест по имени
go test -race ./internal/breaker/... -run TestOpenRejectsCalls

# подробный вывод
go test -race -v ./internal/breaker/...
```

Ожидаемый результат: `ok` для `internal/breaker`, остальные пакеты без тестовых файлов.

## Ручной запуск

Два терминала (или фон):

```bash
# терминал 1 – flaky upstream
go run ./cmd/upstream

# терминал 2 – API с breaker
go run ./cmd/api
```

Проверка сценария «upstream down → breaker open → recovery»:

```bash
# перевести upstream в режим 500
curl -X POST localhost:9090/admin/down

# несколько запросов: сначала 502, затем 503 без задержки
for i in $(seq 1 20); do
  curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/proxy
done

# восстановить upstream и подождать OpenTimeout (2s)
curl -X POST localhost:9090/admin/healthy
sleep 3
curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/proxy
# ожидаем: 200
```

Режимы upstream:

```bash
curl -X POST localhost:9090/admin/healthy   # 200, быстрый ответ
curl -X POST localhost:9090/admin/down      # 500
curl -X POST localhost:9090/admin/slow      # ~5s, затем 200
curl localhost:9090/admin/mode              # текущий режим
```

## Структура

```text
cmd/
  api/          – HTTP :8080, GET /proxy
  upstream/     – HTTP :9090, GET /data + admin/*
internal/
  breaker/      – circuit breaker (Closed / Open / HalfOpen)
  upstream/     – HTTP-клиент с breaker
  api/          – handler, ErrOpen → 503
```
