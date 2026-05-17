# ДЗ-10. Observability: logs + metrics + tracing

## Что дано

Сервис заказов на чистом `net/http`:

* `POST /orders` – создаёт заказ (имитация записи в БД, 50–200ms, 5% ошибок).
* `GET  /orders/{id}` – читает заказ.
* `GET  /healthz`.

Сейчас в нём только `log.Printf`, ни метрик, ни трейсов. Все места под
инструментацию помечены `TODO`.

## Задача

### 1. Structured logging

* Поднять `slog` с `slog.NewJSONHandler` и `slog.SetDefault`.
* Добавить attrs `trace_id` и `span_id`, которые автоматически тянутся из
  `ctx` (см. `trace.SpanFromContext(ctx).SpanContext()`).
* Заменить `log.Printf` в `internal/api/handler.go` на `slog.InfoContext`.

### 2. Метрики (Prometheus)

Минимум 3, желательно 4–5:

| Метрика                              | Тип       | Лейблы                      |
|--------------------------------------|-----------|-----------------------------|
| `http_requests_total`                | Counter   | `method`, `path`, `status`  |
| `http_request_duration_seconds`      | Histogram | `method`, `path`            |
| `http_active_connections`            | Gauge     | –                           |
| `orders_total`                       | Counter   | `status` (`ok` / `error`)   |

Ручка `/metrics` через `promhttp.HandlerFor(reg, …)`.

### 3. Трейсинг

* `OpenTelemetry` TracerProvider с экспортером:
  * `stdouttrace` (просто, без инфраструктуры) **или**
  * `otlptracehttp` в Jaeger (`docker compose up jaeger`, UI на
    [http://localhost:16686](http://localhost:16686)).
* W3C TraceContext propagator (`otel.SetTextMapPropagator(propagation.TraceContext{})`).
* `otelhttp.NewHandler(mux, "http")` – span на каждый входящий запрос.
* Дочерние span-ы внутри `store.Save` / `store.Get` / `orders.Create`
  с осмысленными атрибутами.

### 4. Корреляция

Сделать запрос и убедиться, что **один и тот же** `trace_id`:

1. Видно в JSON-логе сервиса.
2. Видно в Jaeger UI как trace.
3. (бонус) Виден как exemplar на гистограмме Prometheus.

Этот шаг описать в README/ответе и приложить скриншот / пример лога.

## Запуск

```bash
docker compose up -d              # jaeger + prometheus
go run ./cmd/api                  # сервис на :8080

# нагрузка
for i in (seq 1 50)
  curl -s -X POST localhost:8080/orders \
    -H content-type:application/json \
    -d '{"user":"alice","amount":100}' >/dev/null
end

# Метрики
curl localhost:8080/metrics | rg http_requests_total

# Jaeger UI
open http://localhost:16686
```

## Рекомендуемые зависимости

```
go get \
  go.opentelemetry.io/otel \
  go.opentelemetry.io/otel/sdk \
  go.opentelemetry.io/otel/exporters/stdout/stdouttrace \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \
  go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp \
  github.com/prometheus/client_golang/prometheus \
  github.com/prometheus/client_golang/prometheus/promhttp
```

## Критерии приёмки

* `go test -race ./...` зелёный.
* `/metrics` возвращает Prometheus-формат с описанными выше метриками.
* Каждое сообщение лога – валидный JSON и содержит `trace_id` (для
  запросов внутри хэндлеров).
* Один и тот же `trace_id` пробивается через лог и Jaeger.
