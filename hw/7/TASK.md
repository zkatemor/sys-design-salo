# ДЗ-7. Circuit Breaker

## Что дано

* `cmd/upstream` — флаки-бэкенд (`:9090`). Режимы переключаются админ-ручками:
  ```
  curl -X POST localhost:9090/admin/healthy
  curl -X POST localhost:9090/admin/down
  curl -X POST localhost:9090/admin/slow
  ```
* `cmd/api` — основной сервис (`:8080`). `GET /proxy` ходит в upstream через
  `internal/upstream.Client`.

## Задача

1. Реализовать circuit breaker в `internal/breaker` со стейтами
   `Closed / Open / HalfOpen` и переходами между ними.

2. Параметры конфига:
   * `FailureThreshold` — сколько подряд ошибок открывает breaker;
   * `OpenTimeout` — сколько ждать в Open до перехода в HalfOpen;
   * `HalfOpenMaxProbes` — лимит одновременных проб в HalfOpen;
   * `SuccessThreshold` — сколько подряд успехов в HalfOpen закрывают breaker.

3. Встроить breaker в `internal/upstream.Client.Fetch` (там стоит TODO).
   В `internal/api/handler.go` для `breaker.ErrOpen` отдавать 503.

4. Снять `t.Skip` в `internal/breaker/breaker_test.go` и добиться, чтобы все
   7 тестов проходили (можно добавить свои).

## Ручная проверка

```bash
go run ./cmd/upstream &
go run ./cmd/api &

curl -X POST localhost:9090/admin/down
for i in (seq 1 20); curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/proxy; end
# после N ошибок /proxy должен отдавать 503 мгновенно

curl -X POST localhost:9090/admin/healthy
sleep 2
curl localhost:8080/proxy
```

## Критерии приёмки

* `go test -race ./...` зелёный.
* В Open запросы отклоняются мгновенно, без обращения к upstream.
* Время в тестах берётся из `Config.Now`, не из `time.Now`.
