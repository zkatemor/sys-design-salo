# ДЗ-10. Observability: logs + metrics + tracing

Учебный сервис заказов с тремя столпами observability: structured logs, Prometheus metrics, OpenTelemetry traces.

| Файл | Описание |
|------|----------|
| [TASK.md](TASK.md) | Условие задания |

## Реализовано

### Structured logging

- В каждой записи из request context: `trace_id`, `span_id` (через `traceLogHandler` и middleware)
- `log.Printf` заменён на `slog.InfoContext` / `slog.ErrorContext` в handlers

### Метрики (Prometheus)

| Метрика | Тип | Лейблы |
|---------|-----|--------|
| `http_requests_total` | Counter | `method`, `path`, `status` |
| `http_request_duration_seconds` | Histogram | `method`, `path` (+ exemplar `trace_id`) |
| `http_active_connections` | Gauge | – |
| `orders_total` | Counter | `status` (`ok` / `error`) |

Эндпоинт: `GET /metrics`

### Трейсинг

- OpenTelemetry `TracerProvider` → OTLP HTTP (`localhost:4318`, Jaeger из docker-compose)
- Альтернатива: `OTEL_TRACES_EXPORTER=stdout` (без Jaeger)
- W3C `TraceContext` propagator
- `otelhttp.NewHandler` – span на каждый HTTP-запрос
- Дочерние spans: `orders.Create`, `orders.Get`, `store.Save`, `store.Get`

## Запуск

```bash
cd sys-design-salo/hw/10

# Jaeger + Prometheus
docker compose up -d

# Сервис
go run ./cmd/api

# Нагрузка
for i in $(seq 1 50); do
  curl -s -X POST localhost:8080/orders \
    -H 'Content-Type: application/json' \
    -d '{"user":"alice","amount":100}' >/dev/null
done

# Метрики
curl -s localhost:8080/metrics | rg 'http_requests_total|orders_total'

# Jaeger UI
open http://localhost:16686
```

## Проверка корреляции

Один запрос – один `trace_id` во всех сигналах.

### 1. Сгенерировать запрос

```bash
curl -v -X POST localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -d '{"user":"alice","amount":100}'
```

В ответе от `otelhttp` будет заголовок `Traceparent: 00-<trace_id>-<span_id>-01`.

### 2. Логи (JSON)

В stdout сервиса найти строку `request completed` с тем же `trace_id`:

```json
{
  "time": "...",
  "level": "INFO",
  "msg": "request completed",
  "method": "POST",
  "path": "POST /orders",
  "status": 201,
  "duration_seconds": 0.12,
  "trace_id": "abc123...",
  "span_id": "def456..."
}
```

### 3. Jaeger UI

1. Открыть http://localhost:16686
2. Service: `obssvc`
3. Find Traces → выбрать trace по `trace_id` из лога
4. Дерево spans: `POST /orders` → `orders.Create` → `store.Save`

### 4. Prometheus (бонус: exemplar)

В Prometheus UI (http://localhost:9090) на гистограмме `http_request_duration_seconds` при включённых exemplars можно перейти к trace по `trace_id` (если Prometheus ≥ 2.40 и scrape с exemplars).

### Пример: совпадение trace_id

| Источник | trace_id (пример) |
|----------|-------------------|
| HTTP header `Traceparent` | `7f3a1b2c...` (вторая часть после `00-`) |
| JSON log `trace_id` | `7f3a1b2c...` |
| Jaeger UI | `7f3a1b2c...` |

## Тесты

```bash
go test -race ./...
```

- `/metrics` содержит все метрики
- после запроса в логе есть валидный JSON с `trace_id`
- ответ содержит заголовок `Traceparent`

## Переменные окружения

| Переменная | По умолчанию | Назначение |
|------------|--------------|------------|
| `API_ADDR` | `:8080` | Адрес HTTP |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4318` | Jaeger OTLP HTTP |
| `OTEL_TRACES_EXPORTER` | `otlp` | `stdout` – вывод spans в консоль (для тестов) |
