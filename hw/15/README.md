# ДЗ-15. Кэширование search endpoint

| Файл | Описание |
|------|----------|
| [TASK.md](TASK.md) | Условие |
| [WRITEUP.md](WRITEUP.md) | Стратегия, TTL, thundering herd, bench |
| [internal/cache/cache.go](internal/cache/cache.go) | TTL + singleflight |
| [internal/search/client.go](internal/search/client.go) | Read-through в Search |

## Запуск

```bash
go run ./cmd/upstream &   # :9090, 300ms latency
go run ./cmd/api &        # :8080

curl 'http://localhost:8080/search?q=golang'
./scripts/bench.sh
curl http://localhost:9090/admin/stats   # calls=1 после bench
```

## Тесты

```bash
go test -race ./internal/cache/...
```

## Решение

- **Стратегия:** cache-aside (read-through `GetOrLoad`)
- **TTL:** 10 минут (`DefaultCacheTTL`)
- **Thundering herd:** `golang.org/x/sync/singleflight`
