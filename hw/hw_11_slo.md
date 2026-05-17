# SLO-карточка: сервис поиска и бронирования авиабилетов

**Версия:** 1.0  
**Владелец:** команда платформы бронирования  
**Источник NFR:** [hw_2.md](hw_2.md) (занятие 2)  
**Связь:** мониторинг из [hw/10](hw/10/) (`http_requests_total`, `http_request_duration_seconds`)

---

## 1. Контекст и границы

Учебный сервис состоит из двух критичных HTTP-путей:

| Путь | NFR (ДЗ-2) | Пользовательский сценарий |
|------|------------|---------------------------|
| **Поиск** | Availability **99.9%**, p95 **< 1500 ms**, throughput **20 RPS** | Метапоиск по рейсам (fan-out к GDS/OTA) |
| **Бронирование** | Availability **99.95%**, p95 **< 3000 ms**, throughput **10 TPS** | Создание брони + платёж (strong consistency) |

**Исключаем из SLI** (не пользовательский трафик):

- `GET /healthz`, `GET /metrics`
- synthetic probes (`X-Synthetic-Probe: true`)
- `GET /admin/*`

**Окно измерения SLO:** rolling **30 дней** (календарный месяц – для отчётности error budget).

**База для расчёта:** 30 суток = **43 200 минут**; квартал (90 суток) = **129 600 минут**.

Формула error budget:

```text
Error Budget (мин) = (1 − SLO) × длительность_окна_в_минутах
```

---

## 2. SLI и SLO

| # | SLI | Формула (числитель / знаменатель) | SLO | EB мин/мес | EB мин/квартал | Обоснование |
|---|-----|-----------------------------------|-----|------------|----------------|-------------|
| 1 | **Availability – поиск** | HTTP 2xx на `/api/v1/search*` / все запросы поиска | **99.9%** | **43.2** | **129.6** | NFR зан. 2: поиск 99.9% (~8.7 ч/год); допустима деградация (кэш, CB) |
| 2 | **Availability – бронирование** | HTTP 2xx на `POST /bookings`, `GET /bookings*` / все booking-запросы | **99.95%** | **21.6** | **64.8** | NFR: бронирование 99.95% (~4.4 ч/год); финансы, RPO=0 |
| 3 | **Latency – поиск** | запросы с duration **< 1.5 s** / все запросы поиска | **99.0%** | **432.0** | **1296.0** | NFR p95 < 1500 ms; SLI «доля быстрых», не средний p95 |
| 4 | **Throughput – поиск (пик)** | 5-мин окна, где успешный RPS ≥ **16** при целевых 20 RPS / все 5-мин окна в peak-часах | **99.5%** | **216.0** | **648.0** | NFR throughput 20 RPS; запас ×0.8 на деградацию без 5xx |

### Почему именно эти SLI

| Выбрано | Почему | Что сознательно не берём |
|---------|--------|-------------------------|
| Availability ×2 | RED-метрика; разный NFR для поиска и брони | Один общий SLO скрывает падение оплаты при «зелёном» поиске |
| Latency (доля fast) | Отражает UX лучше, чем «средний p95» | SLI «p95 < X» как SLO – это percentil, не доля событий |
| Throughput (окна) | NFR на 20 RPS; проверяем устойчивость под пиком | CPU/RPS без ошибок – не равно хорошему UX |

**Альтернатива latency для бронирования** (можно добавить в v2): доля `POST /bookings` < 3 s при SLO **98.5%** (NFR p95 < 3000 ms + платёж).

**SLA vs SLO:** публичный SLA на поиск – **99.5%** (мягче на 0.4%), на бронирование – **99.9%** (буфер 0.05% к внутреннему 99.95%).

---

## 3. Расчёт error budget (детально)

### 3.1. На месяц (43 200 мин)

| SLI | SLO | (1 − SLO) | Error budget |
|-----|-----|-----------|--------------|
| Availability поиск | 99.9% | 0.001 | 0.001 × 43 200 = **43.2 мин** (~26 с/день) |
| Availability бронирование | 99.95% | 0.0005 | 0.0005 × 43 200 = **21.6 мин** (~43 с/день) |
| Latency поиск < 1.5 s | 99.0% | 0.01 | 0.01 × 43 200 = **432 мин** (~7.2 ч/мес) |
| Throughput peak | 99.5% | 0.005 | 0.005 × 43 200 = **216 мин** (~3.6 ч/мес) |

### 3.2. На квартал (129 600 мин)

| SLI | Error budget (квартал) | В часах |
|-----|------------------------|---------|
| Availability поиск | **129.6 мин** | **2.16 ч** |
| Availability бронирование | **64.8 мин** | **1.08 ч** |
| Latency поиск | **1296 мин** | **21.6 ч** |
| Throughput peak | **648 мин** | **10.8 ч** |

### 3.3. Справочник «девяток» (из занятия 11)

| SLO | Downtime / год | Downtime / месяц | Downtime / квартал |
|-----|----------------|------------------|-------------------|
| 99.9% | 8 ч 45 мин | 43.2 мин | 129.6 мин |
| 99.95% | 4 ч 22 мин | 21.6 мин | 64.8 min |
| 99.5% | 1 д 19 ч | 216 мин | 648 мин |
| 99.0% | 3 д 15 ч | 432 min | 1296 min |

---

## 4. PromQL для SLI

Метки: `service="travel-platform"`, маршруты по префиксу. Окно `[30d]` или `[5m]` для throughput.

### SLI 1 – Availability поиск

```promql
# Good (числитель):
sum(increase(http_requests_total{
  service="travel-platform",
  route=~"/api/v1/search.*",
  status=~"2.."
}[30d]))

# Total (знаменатель):
sum(increase(http_requests_total{
  service="travel-platform",
  route=~"/api/v1/search.*"
}[30d]))

# SLI:
# good / total
```

### SLI 2 – Availability бронирование

```promql
# Good:
sum(increase(http_requests_total{
  service="travel-platform",
  route=~"/api/v1/bookings.*|/api/v1/bookings",
  method=~"GET|POST|PUT",
  status=~"2.."
}[30d]))

# Total:
sum(increase(http_requests_total{
  service="travel-platform",
  route=~"/api/v1/bookings.*|/api/v1/bookings",
  method=~"GET|POST|PUT"
}[30d]))
```

### SLI 3 – Latency поиск (доля запросов < 1.5 s)

```promql
# Good (гистограмма, le=1.5 для 1500ms):
sum(increase(http_request_duration_seconds_bucket{
  service="travel-platform",
  route=~"/api/v1/search.*",
  le="1.5"
}[30d]))

# Total:
sum(increase(http_request_duration_seconds_count{
  service="travel-platform",
  route=~"/api/v1/search.*"
}[30d]))
```

### SLI 4 – Throughput поиск (peak windows)

```promql
# Good: 5-минутные окна, где успешный RPS >= 16 (80% от 20 RPS)
count_over_time(
  (
    sum(rate(http_requests_total{
      service="travel-platform",
      route=~"/api/v1/search.*",
      status=~"2.."
    }[5m])) >= 16
  )[30d:5m]
)

# Total: число 5-мин окон в peak-часах (например 20% суток = 3 ч/день × 30)
# count_over_time((sum(rate(...[5m])) >= 0)[peak_hours:5m])
# Упрощённо для учебного ДЗ – все 5m окна за 30d:
count_over_time((vector(1))[30d:5m])
```

> Для production peak-часы задаются recording rule (`hour()`) или отдельным label `traffic_profile="peak"`.

---

## 5. Error Budget Policy

Остаток бюджета считаем по **самому строгому** из четырёх SLI (обычно availability бронирования) или по каждому SLI отдельно.

| Зона | Остаток EB (месяц) | Условие (пример: availability брони) | Действия |
|------|-------------------|--------------------------------------|----------|
| **Зелёная** | **> 50%** (> 10.8 мин из 21.6) | SLO на траектории | Обычный ритм: фичи, рефакторинг, chaos в staging |
| **Жёлтая** | **10–50%** (2.2–10.8 мин) | Ускоренный burn | Заморозка рискованных деплоев; приоритет reliability; postmortem по инцидентам; burn-rate алерты warning |
| **Красная** | **< 10%** (< 2.2 мин) | SLA под угрозой | **Stop the line**: только hotfix и reliability; game day; пересмотр SLO на квартальном review |
| **Исчерпан** | **≤ 0%** | Нарушение SLO | Incident review обязателен; RCA; план восстановления EB; эскалация на product |

**Правила учёта потребления:**

- Плановые работы с уведомлением B2B – аннотация в Grafana, **не** вычитаются из billable EB (по согласованию).
- Инциденты P1/P2 – полностью в billable.
- Деплои с rolling restart – в billable, если нарушили SLI.

**Burn rate (для алертов, занятие 12):**

- Fast burn: 14.4× месячного бюджета за 1 ч → page (для 99.9% availability).
- Slow burn: 6× за 6 ч → ticket.

---

## 6. Связь SLO ↔ NFR ↔ архитектура

| SLO | NFR (ДЗ-2) | Как достигаем (ДЗ 7–10) |
|-----|------------|-------------------------|
| Availability поиск 99.9% | Табл. 3.1 | CB + cache fallback, degraded mode |
| Availability брони 99.95% | Табл. 3.1 | PG semi-sync, Patroni, idempotency |
| Latency поиск 99% < 1.5 s | p95 < 1500 ms | Redis cache, timeout на GDS |
| Throughput 20 RPS | §1.3, §2.1 | HPA stateless search, rate limit на upstream |

---

## 7. Альтернативы (отвергнуты)

| Вариант | Почему отвергнут |
|---------|------------------|
| SLO 99.99% на всё | Стоимость ×10; NFR не требует; EB ≈ 4.3 мин/мес – блокирует деплои |
| SLI по CPU < 80% | Не отражает UX (antipattern из конспекта) |
| Один composite SLO 99.9% | Не видно, что упало: поиск или оплата |
| SLO 100% | Error budget = 0, нереалистично |

---

## 8. План внедрения (кратко)

1. Recording rules в Prometheus для четырёх SLI (30d rolling).
2. Grafana dashboard «SLO Overview» + еженедельный EB report в Slack.
3. MWMBR алерты (fast/slow burn) – занятие 12.
4. Quarterly review SLO: сравнить факт за 90 дней с целями, скорректировать на ±0.05%.

---

## Итог

Для учебного сервиса на основе NFR из занятия 2 заданы **4 SLI**: availability поиска (**99.9%**), availability бронирования (**99.95%**), latency поиска (**99%** запросов < 1.5 s), throughput поиска в peak (**99.5%** окон).

**Error budget на месяц:** 43.2 + 21.6 + 432 + 216 мин (по SLI независимо).  
**Error budget на квартал:** 129.6 + 64.8 + 1296 + 648 мин.

Критичный внутренний контракт – **бронирование 99.95%** (21.6 мин/мес); публичный SLA мягче – **99.9%**.
