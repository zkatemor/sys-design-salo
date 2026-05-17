# ДЗ-12. Алерты и runbooks для сервиса поиска и бронирования

**Версия:** 1.0  
**Сервис:** `travel-platform` (поиск + бронирование)  
**Источники:** [hw_11_slo.md](hw_11_slo.md), [hw_2.md](hw_2.md), [hw/10](hw/10/)

---

## 1. Стратегия алертинга

| Принцип | Реализация |
|---------|-----------|
| **Symptom-based** | Алертируем на UX-метрики (ошибки, latency, saturation), не на «CPU > 80%» без контекста |
| **Привязка к SLO** | Пороги выведены из NFR и SLO занятия 11 |
| **Runbook обязателен** | Каждый алерт содержит `runbook_url` – без инструкции алерт не проходит review |
| **for ≥ 2m** | Critical-алерты не срабатывают мгновенно (защита от flapping) |

**Маршрутизация (Alertmanager):**

| Severity | Канал | Когда |
|----------|-------|-------|
| `critical` | PagerDuty / on-call | Error rate, latency p99, критическая saturation |
| `warning` | Slack `#travel-alerts` | Тренд, предупреждение до исчерпания ресурса |

---

## 2. Сводная таблица алертов

| # | Alert name | Тип (задание) | SLI / SLO | Severity | `for` | Порог |
|---|------------|---------------|-----------|----------|-------|-------|
| 1 | `SearchHighErrorRate` | **Error rate** | Availability поиск **99.9%** | critical | 3m | > **1%** 5xx за 5m |
| 2 | `SearchLatencyP99High` | **Latency p99** | Latency поиск **p95 < 1.5s** (NFR) | critical | 5m | p99 > **2.0s** за 10m |
| 3 | `SearchSaturationHigh` | **Saturation** | Throughput **20 RPS**, устойчивость под пиком | warning | 10m | CPU > **85%** limit или pool DB > **80%** |

**Обоснование порогов:**

- **1% error rate** – при SLO 99.9% допустимо ~0.1% в среднем за месяц; 1% за 5 мин – сигнал деградации, требующий реакции до burn rate.
- **p99 > 2.0s** – NFR p95 < 1500 ms; p99 строже p95; 2s даёт запас и ловит деградацию до массовых жалоб.
- **85% CPU / 80% DB pool** – USE-методика: действовать до исчерпания (100%), пока ещё есть запас на scale/restart.

---

## 3. Prometheus alert rules

Файл: [`hw/12/alerts/travel-platform-alerts.yaml`](hw/12/alerts/travel-platform-alerts.yaml)

```yaml
groups:
  - name: travel_search_slo_alerts
    rules:
      - alert: SearchHighErrorRate
        expr: |
          (
            sum(rate(http_requests_total{
              service="travel-platform",
              route=~"/api/v1/search.*",
              status=~"5.."
            }[5m]))
            /
            sum(rate(http_requests_total{
              service="travel-platform",
              route=~"/api/v1/search.*"
            }[5m]))
          ) > 0.01
          and
          sum(rate(http_requests_total{
            service="travel-platform",
            route=~"/api/v1/search.*"
          }[5m])) > 0.1
        for: 3m
        labels:
          severity: critical
          service: travel-platform
          component: search
          sli: availability
          team: platform
        annotations:
          summary: "Поиск: error rate > 1% (5xx)"
          description: "Доля 5xx на /api/v1/search – {{ $value | humanizePercentage }}. SLO availability 99.9% под угрозой."
          runbook_url: "https://wiki.example.com/runbooks/search-high-error-rate"
          dashboard_url: "https://grafana.example.com/d/travel-search-slo"

      - alert: SearchLatencyP99High
        expr: |
          histogram_quantile(0.99,
            sum by (le) (
              rate(http_request_duration_seconds_bucket{
                service="travel-platform",
                route=~"/api/v1/search.*"
              }[5m])
            )
          ) > 2.0
          and
          sum(rate(http_request_duration_seconds_count{
            service="travel-platform",
            route=~"/api/v1/search.*"
          }[5m])) > 0.1
        for: 5m
        labels:
          severity: critical
          service: travel-platform
          component: search
          sli: latency
          team: platform
        annotations:
          summary: "Поиск: p99 latency > 2s"
          description: "p99 = {{ $value | humanizeDuration }}. NFR p95 < 1.5s нарушен на хвосте распределения."
          runbook_url: "https://wiki.example.com/runbooks/search-latency-p99-high"
          dashboard_url: "https://grafana.example.com/d/travel-search-latency"

      - alert: SearchSaturationHigh
        expr: |
          (
            sum(rate(container_cpu_usage_seconds_total{pod=~"search-api-.*"}[5m]))
            /
            sum(kube_pod_container_resource_limits{
              resource="cpu",
              pod=~"search-api-.*"
            })
          ) > 0.85
          or
          (
            avg(pg_stat_activity_count{service="travel-platform", role="search"})
            /
            avg(pg_settings_max_connections{service="travel-platform"})
          ) > 0.80
        for: 10m
        labels:
          severity: warning
          service: travel-platform
          component: search
          sli: saturation
          team: platform
        annotations:
          summary: "Поиск: высокая saturation (CPU или DB pool)"
          description: "CPU {{ $value | humanizePercentage }} от limit или connection pool > 80%. Риск 503 при пике 20 RPS."
          runbook_url: "https://wiki.example.com/runbooks/search-saturation-high"
          dashboard_url: "https://grafana.example.com/d/travel-search-infra"
```

### Inhibit rules (фрагмент Alertmanager)

```yaml
inhibit_rules:
  - source_matchers:
      - alertname = "SearchHighErrorRate"
    target_matchers:
      - alertname = "SearchLatencyP99High"
    equal: ['service', 'component']
  - source_matchers:
      - alertname = "SearchSaturationHigh"
    target_matchers:
      - alertname = "SearchLatencyP99High"
    equal: ['service', 'component']
```

При массовых 5xx saturation часто является первопричиной – подавляем вторичный latency-алерт.

---

## 4. Runbook #1: `SearchHighErrorRate`

**Владелец:** team-platform | **Версия:** 1.0 | **Severity:** critical  
**Связанный SLO:** Availability поиск **99.9%** ([hw_11_slo.md](hw_11_slo.md))

### Симптом

- Пользователи видят ошибки при поиске рейсов («что-то пошло не так»).
- Растёт доля HTTP **5xx** на `GET /api/v1/search*`.
- Bounce rate на странице поиска растёт; B2B-партнёры могут получать 502/503.

### Возможные причины

| Причина | Вероятность | Как подтвердить |
|---------|-------------|----------------|
| Недоступен upstream (GDS/OTA API) | Высокая | CB open, timeout в логах |
| Деградация PostgreSQL (реплика/read) | Средняя | `pg_stat_replication`, ошибки в логах |
| OOM / crash pod search-api | Средняя | `kubectl describe pod`, restart count |
| Недавний деплой с багом | Средняя | Correlate с `kube_deployment_status_observed_generation` |
| Перегрузка (saturation) | Средняя | CPU/memory, RPS vs limit |

### Шаги диагностики

**Шаг 1 – масштаб (2 мин):**

```promql
# Error rate %
sum(rate(http_requests_total{service="travel-platform",route=~"/api/v1/search.*",status=~"5.."}[5m]))
/ sum(rate(http_requests_total{service="travel-platform",route=~"/api/v1/search.*"}[5m])) * 100

# По маршрутам
topk(5, sum by (route, status) (rate(http_requests_total{service="travel-platform",route=~"/api/v1/search.*",status=~"5.."}[5m])))
```

**Шаг 2 – логи и traces (3 мин):**

```bash
kubectl logs -l app=search-api --since=10m | jq 'select(.level=="ERROR" or .status>=500)'

# В Jaeger: service=search-api, error=true, последние 15 мин
```

**Шаг 3 – зависимости (3 мин):**

```bash
# Circuit breaker / upstream
curl -s http://search-api:8080/debug/circuit | jq .

# Health upstream
kubectl exec deploy/search-api -- curl -sf http://fare-provider/healthz
kubectl exec deploy/search-api -- curl -sf http://schedule-cache/healthz
```

**Шаг 4 – инфраструктура (2 мин):**

```bash
kubectl get pods -l app=search-api
kubectl top pods -l app=search-api
```

### Митигация

| Сценарий | Действие |
|----------|----------|
| Upstream down | Включить **degraded mode**: `FARE_CACHE_MODE=true`, CB уже открыт |
| OOM / crash | `kubectl rollout restart deployment/search-api` или **rollback** |
| Баг после деплоя | `kubectl rollout undo deployment/search-api` |
| DB перегружена | Снизить read с реплик с lag, scale read pool, fail over при необходимости |
| Пик нагрузки | HPA scale +2 replicas (см. runbook saturation) |

### Эскалация

| Условие | Кому |
|---------|------|
| Error rate > **5%** более **10 мин** | Team lead + eng manager |
| Не помогло за **15 мин** | @platform-oncall, DBA on-call |
| Проблема в payment/booking path | Переключить фокус на booking runbook |

### После инцидента

- Тикет + timeline; обновить error budget report.
- PIR при error rate > 5% или downtime > 5 min.
- Проверить, нужен ли отдельный алерт на upstream.

---

## 5. Runbook #2: `SearchLatencyP99High`

**Владелец:** team-platform | **Версия:** 1.0 | **Severity:** critical  
**Связанный SLO:** NFR p95 < **1500 ms**, SLI latency **99% < 1.5s** ([hw_11_slo.md](hw_11_slo.md))

### Симптом

- Поиск «тормозит»: пользователи ждут > 2 с.
- p99 latency превышает порог; p95 может быть ещё в норме – страдает хвост.
- B2B SLA «ответ < 5 с» пока не нарушен, но риск растёт.

### Возможные причины

| Причина | Вероятность | Как подтвердить |
|---------|-------------|----------------|
| Медленный GDS/OTA (fan-out) | Высокая | Traces: долгие spans `upstream.*` |
| Cache miss storm | Средняя | Redis hit rate ↓, latency ↑ |
| CPU saturation | Средняя | `SearchSaturationHigh` firing |
| Cold start после scale | Низкая | Spike после HPA |
| Сеть между AZ | Низкая | Node latency, packet loss |

### Шаги диагностики

**Шаг 1 – распределение latency:**

```promql
histogram_quantile(0.50, sum by (le)(rate(http_request_duration_seconds_bucket{route=~"/api/v1/search.*"}[5m])))
histogram_quantile(0.95, sum by (le)(rate(http_request_duration_seconds_bucket{route=~"/api/v1/search.*"}[5m])))
histogram_quantile(0.99, sum by (le)(rate(http_request_duration_seconds_bucket{route=~"/api/v1/search.*"}[5m])))
```

**Шаг 2 – traces (Jaeger):**

- Фильтр: `service=search-api`, sort by duration.
- Найти span с max duration – upstream vs DB vs cache.

**Шаг 3 – upstream и cache:**

```promql
# Latency по dependency (если есть метрики)
histogram_quantile(0.99, sum by (le)(rate(upstream_request_duration_seconds_bucket{service="travel-platform"}[5m])))

# Redis
redis_keyspace_hits_total / (redis_keyspace_hits_total + redis_keyspace_misses_total)
```

```bash
kubectl exec deploy/search-api -- redis-cli INFO stats | grep keyspace
```

**Шаг 4 – ресурсы:**

```bash
kubectl top pods -l app=search-api
```

### Митигация

| Сценарий | Действие |
|----------|----------|
| Медленный upstream | Увеличить timeout budget только для slow path; включить cache; отключить slow provider через feature flag |
| Cache miss | Прогреть cache топ-маршрутов; увеличить TTL на 5 мин (в пределах NFR) |
| CPU saturation | Scale HPA +2; см. runbook saturation |
| После деплоя | Rollback canary |

### Эскалация

| Условие | Кому |
|---------|------|
| p99 > **5s** более **5 мин** | Escalate critical – риск B2B SLA |
| Совместно с `SearchHighErrorRate` | Одна war-room, root cause общая |

### После инцидента

- Зафиксировать, какой upstream дал хвост latency.
- Рассмотреть отдельный SLO burn alert на latency (занятие 11).

---

## 6. Runbook #3: `SearchSaturationHigh`

**Владелец:** team-platform | **Версия:** 1.0 | **Severity:** warning  
**Связанный SLO:** Throughput peak **99.5%**, проект **20 RPS** ([hw_11_slo.md](hw_11_slo.md))

### Симптом

- CPU search-api близок к limit – растёт latency, возможны throttling.
- Connection pool к PostgreSQL > 80% – новые запросы ждут соединения.
- При пике (marketing) возможен cascade в error rate и latency.

### Возможные причины

| Причина | Вероятность | Как подтвердить |
|---------|-------------|----------------|
| Пик трафика (RPS > 20) | Высокая | `sum(rate(http_requests_total{...}[1m]))` |
| Утечка горутин / memory | Средняя | `go_goroutines`, memory trend |
| Мало replicas | Средняя | HPA на min replicas |
| Тяжёлые запросы (fan-out) | Средняя | High latency + high CPU correlate |
| DB pool слишком мал | Средняя | `pg_stat_activity` vs max |

### Шаги диагностики

**Шаг 1 – CPU и RPS:**

```promql
sum(rate(http_requests_total{service="travel-platform",route=~"/api/v1/search.*"}[5m]))

sum(rate(container_cpu_usage_seconds_total{pod=~"search-api-.*"}[5m]))
/ sum(kube_pod_container_resource_limits{resource="cpu",pod=~"search-api-.*"})
```

**Шаг 2 – DB pool:**

```sql
SELECT count(*) FROM pg_stat_activity WHERE datname = 'travel';
SHOW max_connections;
```

**Шаг 3 – горутины и GC (Go):**

```promql
go_goroutines{service="travel-platform"}
rate(go_gc_duration_seconds_count{service="travel-platform"}[5m])
```

**Шаг 4 – HPA:**

```bash
kubectl get hpa search-api
kubectl describe hpa search-api
```

### Митигация

| Сценарий | Действие |
|----------|----------|
| Пик RPS | HPA: min replicas 4 → 6; rate limit на B2B; CDN для static |
| CPU limit низкий | Временно поднять limit (не requests) + scale out |
| DB pool exhausted | Увеличить `max_connections` pool в app (осторожно с PG max); pgbouncer |
| Memory leak | Rolling restart; rollback деплоя |

### Эскалация

| Условие | Кому |
|---------|------|
| CPU > **95%** более **5 min** | Critical – page on-call |
| Совпадает с error rate | Объединить инциденты |

### После инцидента

- Capacity review: хватает ли 20 RPS с запасом ×3.
- Load test в staging на 30 RPS.

---

## 7. Alertmanager routing (кратко)

Файл: [`hw/12/alertmanager.yml`](hw/12/alertmanager.yml)

```yaml
route:
  receiver: slack-warning
  group_by: [alertname, service, component]
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  routes:
    - matchers:
        - severity = critical
      receiver: pagerduty-critical
      continue: true
    - matchers:
        - severity = critical
      receiver: slack-critical

receivers:
  - name: pagerduty-critical
  - name: slack-critical
  - name: slack-warning
```

---

## 8. Promtool test (фрагмент)

Файл: [`hw/12/alerts/travel-platform-alerts_test.yaml`](hw/12/alerts/travel-platform-alerts_test.yaml)

Тестируем `SearchHighErrorRate`: при норме – нет алерта; при 5% 5xx – firing.

---

## 9. Связь с занятиями 11–10

| Занятие | Связь |
|---------|-------|
| **11 SLO** | Пороги error rate и latency выведены из SLO 99.9% / 99% fast |
| **10 Observability** | Метрики `http_requests_total`, `http_request_duration_seconds`; traces по `trace_id` в runbook |
| **7 Circuit breaker** | Диагностика upstream в runbook error rate |

---

## Итог

Для учебного сервиса определены **3 symptom-based алерта** по требованию ДЗ:

1. **Error rate** – доля 5xx на поиске > 1%.  
2. **Latency p99** – p99 > 2 s (строже NFR p95 < 1.5 s).  
3. **Saturation** – CPU > 85% или DB connection pool > 80%.

Для каждого – **runbook** с симптомом, причинами, PromQL/kubectl-диагностикой, митигацией и эскалацией. Дополнительно: YAML правил, Alertmanager routing, promtool test.
