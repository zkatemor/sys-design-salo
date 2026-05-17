# ДЗ-15. Кэширование search-эндпоинта

## Что дано

* `cmd/upstream` — «дорогой» поисковый бэкенд (`:9090`).
  Каждый `GET /search?q=…` спит 300 мс. Считает все вызовы в счётчике —
  это главный инструмент проверки эффективности кэша:
  ```
  curl http://localhost:9090/admin/stats   # calls=N
  curl -X POST http://localhost:9090/admin/reset
  ```

* `cmd/api` — учебный сервис (`:8080`), `GET /search?q=…` сейчас просто
  проксирует запрос в upstream, ничего не кэшируя.

* `internal/cache` — пустой скелет под кэш с TODO.

## Бизнес-контекст (чтобы вы могли обосновать TTL)

* Сервис — поиск по каталогу e-commerce.
* Каталог переиндексируется раз в 30 минут.
* По SLA: P99 латентности `/search` ≤ 50 мс.
* Без кэша P99 ≈ 300 мс (одна upstream-операция).
* Распределение запросов по `q` — Zipf: топ-100 запросов = ~80% трафика.

## Задача

### 1. Реализовать кэш

В `internal/cache/cache.go`:

* `Get(key) ([]byte, bool)` — лукап с учётом TTL;
* `Set(key, value, ttl)` — запись с TTL;
* `GetOrLoad(ctx, key, ttl, loader)` — read-through.

Допустимы любые реализации:
* `map + sync.RWMutex + time.Time` для каждого ключа;
* `golang.org/x/sync/singleflight` для thundering herd;
* (опционально) bounded LRU/SIEVE, чтобы кэш не рос бесконечно.

### 2. Стратегия и TTL

В файле `WRITEUP.md` (создать самим, ~15 строк) описать:

1. Какую стратегию выбрали (cache-aside / read-through / write-through)
   и почему — с учётом того, что записей в каталог сервис не делает.
2. Какой TTL выбрали и почему. Учтите:
   * каталог обновляется раз в 30 мин — значит, верхняя граница ~30 мин;
   * пользователю важна свежесть новых товаров — значит, не стоит ставить часы;
   * trade-off между hit-rate и stale-данными.
3. Как защититесь от thundering herd на холодном старте.

### 3. Встроить в search-клиент

`internal/search/client.go`, метод `Search` — обернуть `c.do()` в
`cache.GetOrLoad(ctx, q, ttl, loader)`. Сериализация — JSON.

### 4. Замерить улучшение

Запустить `scripts/bench.sh` дважды: до и после внедрения кэша.
Положить результаты в `WRITEUP.md`. Должно быть видно:

* upstream `calls` падает с ~N до 1 на TTL;
* P99 при попадании в кэш — порядка единиц мс.

## Запуск

```bash
go run ./cmd/upstream &
go run ./cmd/api &

curl 'http://localhost:8080/search?q=golang'

# нагрузка
./scripts/bench.sh
```

## Критерии приёмки

* `go test -race ./internal/cache/...` зелёный (все 5 кейсов).
* В `TestGetOrLoadSingleflight` loader вызван ровно 1 раз на 50 параллельных промахов.
* Bench показывает: при включённом кэше `upstream calls` сильно меньше
  числа клиентских запросов.
* `WRITEUP.md` отвечает на вопросы из раздела «Стратегия и TTL».
