# SPOF-анализ архитектуры flight-booking сервиса

## 0. Контекст и допущения

Дана архитектура flight-booking сервиса с нагрузкой около **10M MAU**.

Критические пользовательские сценарии:

- **логин / авторизация**;
- **поиск рейсов**;
- **расчёт цены**;
- **бронирование**;
- **оплата**;
- **email-уведомления**.

Из диаграммы видно, что часть компонентов уже имеет несколько pod'ов:

- `api` – 3 pod'а;
- `booking-service` – 3 pod'а;
- `search-service` – 3 pod'а;
- `pricing-service` – 3 pod'а.

Но несколько компонентов остаются единичными:

- `WAF (1 instance)`;
- `Load Balancer NGINX (1 VM)`;
- `auth-service (1 pod)`;
- `PostgreSQL primary, no replica`;
- `Redis 1 node`;
- `Elasticsearch 1 node`;
- `RabbitMQ 1 broker`;
- внешние зависимости: `Stripe`, `SendGrid`, `Amadeus GDS`;
- `S3 us-east-1 only`.

Для грубой оценки blast radius считаю:

- 10M MAU = 100% пользователей;
- если ломается входной слой до API – затронуто 100%;
- если ломается auth – затронуты все новые логины и большинство авторизованных операций;
- если ломается поиск – пользователь не может начать покупку;
- если ломается оплата – поиск работает, но деньги бизнес не получает;
- стоимость дана как **ROM-оценка порядка величины**, не точный cloud bill.

## 1. Методика анализа

Для каждого компонента задаём вопрос:

> Что произойдёт, если этот компонент будет недоступен 5 минут?

Для приоритизации используем упрощённую формулу:

```text
RiskScore = blast_radius_pct × (1 / MTBF_days) × MTTR_min
```

Где:

- `blast_radius_pct` – доля пользователей, затронутых отказом, от 0 до 1;
- `MTBF_days` – условная частота отказа компонента;
- `MTTR_min` – сколько минут потребуется на восстановление.

Приоритеты:

- **P0 / Critical** – отказ останавливает весь сервис или деньги;
- **P1 / High** – отказ ломает ключевой сценарий;
- **P2 / Medium** – отказ вызывает деградацию, но не полный outage.

---

# 2. Найденные SPOF

## SPOF #1: WAF – один instance

### SPOF Analysis

**Компонент:** WAF `(1 instance)`  
**Тип SPOF:** `no redundancy / single instance`

### Сценарий отказа

WAF VM падает из-за OOM, сбоя хоста, ошибки конфигурации или неудачного deploy правила. Так как весь трафик идёт по цепочке:

```text
Users → DNS → CDN → WAF → Load Balancer → API
```

при отказе WAF запросы не доходят до load balancer.

### Blast Radius

- **Затронутых пользователей:** 100% от 10M MAU = **10M пользователей**.
- **Недоступные функции:** логин, поиск, бронирование, оплата, уведомления через UI/API.
- **Тип недоступности:** полная недоступность публичного API.
- **Revenue impact/час:** высокий; условно **~100% онлайн-выручки за час**.

### Оценка риска

- **MTBF:** ~90 дней.
- **MTTR:** ~30 минут, если восстанавливать VM или откатывать конфиг вручную.
- **RiskScore:** `1.0 × (1 / 90) × 30 ≈ 0.33`.
- **Приоритет:** **P0 / Critical**.

### Митигация

Перевести WAF в managed/multi-AZ режим:

- использовать Cloudflare WAF / AWS WAF / managed WAF перед CDN/LB;
- держать минимум 2 экземпляра WAF в active-active или active-passive;
- конфигурацию WAF выкатывать через staged rollout и dry-run mode;
- добавить health checks и автоматическое исключение unhealthy instance.

### Стоимость митигации

- **Инфра:** `+$50–200/мес` для managed WAF или второго instance.
- **Работы:** `1 инженер × 1 неделя` на настройку, тесты и runbook.
- **Availability после митигации:** отказ одного WAF instance больше не приводит к полной недоступности; blast radius снижается с 100% до ~0–5% на время failover.

---

## SPOF #2: Load Balancer – NGINX на одной VM

### SPOF Analysis

**Компонент:** Load Balancer `NGINX (1 VM)`  
**Тип SPOF:** `no redundancy / single VM / no failover`

### Сценарий отказа

VM с NGINX становится недоступна из-за сбоя хоста, сетевого разрыва, ошибки конфигурации или переполнения connection pool. API pod'ы живы, но внешний трафик до них не маршрутизируется.

### Blast Radius

- **Затронутых пользователей:** 100% от 10M MAU = **10M пользователей**.
- **Недоступные функции:** все пользовательские сценарии.
- **Тип недоступности:** полная недоступность сервиса.
- **Revenue impact/час:** высокий; условно **~100% онлайн-выручки за час**.

### Оценка риска

- **MTBF:** ~90 дней.
- **MTTR:** ~30 минут при ручном переключении или пересоздании VM.
- **RiskScore:** `1.0 × (1 / 90) × 30 ≈ 0.33`.
- **Приоритет:** **P0 / Critical**.

### Митигация

Вариант A – cloud-managed load balancer:

- AWS ALB / NLB, GCP Load Balancer или аналог;
- multi-AZ по умолчанию;
- health checks до API pod'ов;
- автоматическое исключение unhealthy backend'ов.

Вариант B – self-managed HA:

- 2 NGINX/HAProxy instance;
- active-passive через Keepalived/VRRP;
- автоматический failover.

### Стоимость митигации

- **Инфра:** `+$30–100/мес` для managed LB или `+$50–150/мес` за вторую VM.
- **Работы:** `1 инженер × 1 неделя`.
- **Availability после митигации:** blast radius снижается с 100% до ~0%; отказ одного LB становится прозрачным или даёт короткий failover.

---

## SPOF #3: auth-service – один pod

### SPOF Analysis

**Компонент:** `auth-service (1 pod)`  
**Тип SPOF:** `no redundancy / single pod`

### Сценарий отказа

Pod `auth-service` падает из-за OOM, panic, некорректного deploy или зависания при обращении к PostgreSQL. Все API pod'ы завязаны на один auth-service:

```text
api-pod-1 → auth-service
api-pod-2 → auth-service
api-pod-3 → auth-service
```

Если auth недоступен, сервис не может валидировать токены и авторизовать пользователей.

### Blast Radius

- **Затронутых пользователей:** ~80% от 10M MAU = **8M пользователей**.
- **Недоступные функции:** логин, бронирование, оплата, личный кабинет; поиск может частично работать только для anonymous-сценариев.
- **Тип недоступности:** частичная, но критичная.
- **Revenue impact/час:** высокий, потому что нельзя завершить покупку.

### Оценка риска

- **MTBF:** ~30 дней.
- **MTTR:** ~10 минут, если Kubernetes перезапускает pod, но нет второго pod для бесшовного обслуживания.
- **RiskScore:** `0.8 × (1 / 30) × 10 ≈ 0.27`.
- **Приоритет:** **P0 / Critical**.

### Митигация

- Запустить минимум **3 pod'а auth-service**.
- Разнести pod'ы по разным nodes / AZ через `podAntiAffinity`.
- Добавить readiness/liveness probes.
- В API добавить короткий local cache для проверки JWT/public keys, чтобы кратковременный отказ auth не ломал уже авторизованные запросы.
- Настроить HPA по latency/error rate.

### Стоимость митигации

- **Инфра:** `+$30–100/мес` на дополнительные pod'ы.
- **Работы:** `1 инженер × 0.5–1 неделя`.
- **Availability после митигации:** отказ одного pod'а auth не влияет на пользователей; blast radius снижается с ~80% до ~0–10% при проблемах с БД или общей зависимостью.

---

## SPOF #4: PostgreSQL – primary без replica

### SPOF Analysis

**Компонент:** `PostgreSQL primary, no replica`  
**Тип SPOF:** `single primary / no replica / no automatic failover`

### Сценарий отказа

PostgreSQL primary падает из-за сбоя диска, OOM, ошибки миграции, повреждения данных или проблем с VM. Реплики нет, поэтому читать и писать данные невозможно.

На PostgreSQL завязаны:

- `auth-service`;
- `booking-service`.

### Blast Radius

- **Затронутых пользователей:** ~90–100% от 10M MAU = **9–10M пользователей**.
- **Недоступные функции:** логин, бронирование, история бронирований, часть пользовательского профиля.
- **Поиск:** может частично работать, если не требует авторизации и не пишет историю.
- **Оплата:** недоступна, потому что нельзя создать/подтвердить booking transaction.
- **Тип недоступности:** критичная частичная или почти полная.
- **Revenue impact/час:** очень высокий; новые бронирования и оплаты останавливаются.

### Оценка риска

- **MTBF:** ~60 дней.
- **MTTR:** ~60 минут при восстановлении из backup или ручном поднятии нового primary.
- **RiskScore:** `1.0 × (1 / 60) × 60 = 1.0`.
- **Приоритет:** **P0 / Critical**, самый важный SPOF.

### Митигация

- Primary + standby replica в другой AZ.
- Автоматический failover через Patroni / Cloud managed PostgreSQL / RDS Multi-AZ.
- PITR backup и регулярные restore drills.
- Readiness checks на уровне приложения.
- Для booking – idempotency keys, чтобы retry после failover не создавал дубли.

### Стоимость митигации

- **Инфра:** `+$150–500/мес` за standby/managed Multi-AZ, зависит от размера инстанса и storage.
- **Работы:** `1–2 инженера × 2 недели` на репликацию, failover-тесты, backup/restore runbook.
- **Availability после митигации:** RTO снижается с ~60 минут до 1–5 минут; blast radius снижается с ~100% до короткой деградации на время failover.

---

## SPOF #5: Redis – один node

### SPOF Analysis

**Компонент:** `Redis 1 node`  
**Тип SPOF:** `single cache node / no replica / no failover`

### Сценарий отказа

Redis падает или теряет память. `pricing-service` не может получить закэшированные цены. Сервис начинает чаще ходить в Amadeus GDS или возвращает ошибки/устаревшие цены.

### Blast Radius

- **Затронутых пользователей:** ~50% от 10M MAU = **5M пользователей**.
- **Недоступные функции:** быстрый расчёт цены, часть поиска, отображение актуальных цен.
- **Тип недоступности:** деградация, но возможен каскад: весь pricing traffic пойдёт во внешний GDS.
- **Revenue impact/час:** средний/высокий: конверсия падает, часть бронирований не завершается.

### Оценка риска

- **MTBF:** ~45 дней.
- **MTTR:** ~20 минут.
- **RiskScore:** `0.5 × (1 / 45) × 20 ≈ 0.22`.
- **Приоритет:** **P1 / High**.

### Митигация

- Redis Sentinel: 1 primary + 2 replicas.
- Или managed Redis/ElastiCache with Multi-AZ.
- Cache warming для популярных направлений.
- TTL с jitter, чтобы избежать одновременного истечения ключей.
- Fallback: отдавать last-known price с пометкой, что цена будет проверена на этапе бронирования.

### Стоимость митигации

- **Инфра:** `+$100–300/мес` за 2 дополнительные Redis-ноды или managed Redis.
- **Работы:** `1 инженер × 1 неделя`.
- **Availability после митигации:** отказ одной Redis-ноды не ломает pricing; blast radius снижается с ~50% до ~5–10% на время failover/cache miss.

---

## SPOF #6: Elasticsearch – один node

### SPOF Analysis

**Компонент:** `Elasticsearch 1 node`  
**Тип SPOF:** `single data/search node`

### Сценарий отказа

Elasticsearch node падает, индекс повреждается или диск заполняется. `search-service` не может выполнять поиск по рейсам.

### Blast Radius

- **Затронутых пользователей:** ~70% от 10M MAU = **7M пользователей**.
- **Недоступные функции:** поиск рейсов, фильтры, сортировка, быстрый discovery.
- **Бронирование:** уже начатые бронирования могут завершиться, если не нужен новый поиск.
- **Тип недоступности:** критичная частичная недоступность.
- **Revenue impact/час:** высокий, потому что поиск – начало funnel'а покупки.

### Оценка риска

- **MTBF:** ~45 дней.
- **MTTR:** ~60 минут, если нужно пересоздавать индекс или восстанавливать snapshot.
- **RiskScore:** `0.7 × (1 / 45) × 60 ≈ 0.93`.
- **Приоритет:** **P0 / Critical**.

### Митигация

- Elasticsearch cluster минимум из 3 nodes.
- Репликация shard'ов: `number_of_replicas >= 1`.
- Разнести nodes по AZ.
- Snapshot lifecycle policy.
- Fallback: показывать популярные направления/последний успешный индекс в read-only mode.

### Стоимость митигации

- **Инфра:** `+$200–600/мес` за 2 дополнительные ES-ноды, storage и snapshots.
- **Работы:** `1–2 инженера × 1–2 недели`.
- **Availability после митигации:** отказ одной ноды не останавливает поиск; blast radius снижается с ~70% до ~0–10%.

---

## SPOF #7: RabbitMQ – один broker

### SPOF Analysis

**Компонент:** `RabbitMQ 1 broker`  
**Тип SPOF:** `single broker / no queue replication`

### Сценарий отказа

RabbitMQ broker падает или теряет диск. `booking-service` не может публиковать события бронирования: подтверждения, последующая обработка, уведомления, интеграции.

### Blast Radius

- **Затронутых пользователей:** ~30% от 10M MAU = **3M пользователей**.
- **Недоступные функции:** асинхронная обработка booking-событий, email-уведомления, возможно подтверждение бронирования.
- **Тип недоступности:** частичная; при синхронной зависимости booking может полностью ломаться.
- **Revenue impact/час:** средний/высокий: часть успешных оплат может не получить подтверждение, растёт нагрузка на support.

### Оценка риска

- **MTBF:** ~60 дней.
- **MTTR:** ~30 минут.
- **RiskScore:** `0.3 × (1 / 60) × 30 = 0.15`.
- **Приоритет:** **P1 / High**.

### Митигация

- RabbitMQ quorum queues.
- Минимум 3 broker nodes в разных AZ.
- Publisher confirms.
- Dead letter queues.
- Outbox pattern в PostgreSQL для booking-событий, чтобы не терять события при временной недоступности RabbitMQ.

### Стоимость митигации

- **Инфра:** `+$100–300/мес` за 2 дополнительные broker-ноды.
- **Работы:** `1–2 инженера × 1–2 недели`, особенно если внедрять outbox pattern.
- **Availability после митигации:** отказ одного broker не приводит к потере очереди; blast radius снижается с ~30% до ~0–5%.

---

## SPOF #8: Amadeus GDS – единственный поставщик inventory

### SPOF Analysis

**Компонент:** `Amadeus GDS`  
**Тип SPOF:** `external vendor / no fallback / no multi-vendor`

### Сценарий отказа

Amadeus возвращает 5xx, деградирует latency или начинает отдавать неполные данные. На него завязаны:

- `search-service`;
- `pricing-service`.

Если Amadeus недоступен, поиск и расчёт цен деградируют или полностью ломаются.

### Blast Radius

- **Затронутых пользователей:** ~70% от 10M MAU = **7M пользователей**.
- **Недоступные функции:** поиск доступных рейсов, актуальные цены, проверка availability перед бронированием.
- **Тип недоступности:** частичная или почти полная для поиска новых билетов.
- **Revenue impact/час:** высокий: пользователь не может выбрать и купить билет.

### Оценка риска

- **MTBF:** ~30 дней для внешней зависимости/деградаций.
- **MTTR:** ~60 минут, потому что команда не контролирует провайдера.
- **RiskScore:** `0.7 × (1 / 30) × 60 = 1.4`.
- **Приоритет:** **P0 / Critical**, самый опасный внешний SPOF.

### Митигация

- Подключить второго GDS/поставщика inventory.
- Реализовать provider abstraction layer.
- Circuit breaker на вызовы Amadeus.
- Timeout budget: не ждать GDS бесконечно.
- Cache last-known inventory/prices для degraded mode.
- Partial results: показывать рейсы от доступных источников.

### Стоимость митигации

- **Инфра:** `+$100–300/мес` на cache/дополнительные очереди, но основная стоимость – интеграция и контракт.
- **Vendor:** зависит от договора; условно `+$1k–10k/мес`.
- **Работы:** `2–3 инженера × 4–8 недель`.
- **Availability после митигации:** отказ Amadeus больше не равен отказу поиска; blast radius снижается с ~70% до ~20–30%, если часть inventory доступна через другого провайдера или cache.

---

## SPOF #9: Stripe – единственный провайдер оплаты

### SPOF Analysis

**Компонент:** `Stripe payments`  
**Тип SPOF:** `external vendor / payment provider lock-in`

### Сценарий отказа

Stripe API недоступен, возвращает ошибки, деградирует latency или блокирует часть операций из-за antifraud/rate limit. Пользователь может найти рейс и создать booking, но не может оплатить.

### Blast Radius

- **Затронутых пользователей:** ~25% от 10M MAU = **2.5M пользователей** – все, кто дошёл до оплаты.
- **Недоступные функции:** оплата, финальное подтверждение бронирования.
- **Тип недоступности:** частичная, но напрямую бьёт по revenue.
- **Revenue impact/час:** очень высокий именно для transactional path.

### Оценка риска

- **MTBF:** ~90 дней.
- **MTTR:** ~60 минут, так как восстановление зависит от Stripe.
- **RiskScore:** `0.25 × (1 / 90) × 60 ≈ 0.17`.
- **Приоритет:** **P1 / High**.

### Митигация

- Подключить второго payment provider.
- Payment orchestration layer: маршрутизация платежа по провайдерам.
- Idempotency keys на оплату.
- Retry с backoff и circuit breaker.
- Degraded mode: удержать booking на 15–30 минут и предложить повторить оплату позже.

### Стоимость митигации

- **Инфра:** `+$50–150/мес`.
- **Vendor/fees:** зависит от второго провайдера.
- **Работы:** `2 инженера × 3–6 недель`, включая юридические/финансовые проверки.
- **Availability после митигации:** отказ Stripe не блокирует 100% оплат; blast radius снижается с ~25% до ~5–10%.

---

## SPOF #10: S3 только в us-east-1

### SPOF Analysis

**Компонент:** `S3 us-east-1 only`  
**Тип SPOF:** `single region / no cross-region replication`

### Сценарий отказа

Регион `us-east-1` деградирует, S3 bucket становится недоступен или возникает ошибка IAM/bucket policy. `booking-service` не может читать/писать артефакты бронирования: билеты, ваучеры, документы, возможно вложения для email.

### Blast Radius

- **Затронутых пользователей:** ~20–30% от 10M MAU = **2–3M пользователей**.
- **Недоступные функции:** выдача билетов/документов, часть booking flow, повторная отправка подтверждений.
- **Тип недоступности:** частичная, но чувствительная после оплаты.
- **Revenue impact/час:** средний/высокий; растёт нагрузка на support и риск chargeback.

### Оценка риска

- **MTBF:** ~180 дней.
- **MTTR:** ~120 минут при региональной деградации.
- **RiskScore:** `0.3 × (1 / 180) × 120 = 0.20`.
- **Приоритет:** **P1 / High**.

### Митигация

- Cross-region replication в другой регион.
- Версионирование bucket'ов.
- Read fallback: если `us-east-1` недоступен, читать из replica bucket.
- Backup critical documents metadata в PostgreSQL или отдельном object storage provider.
- Регулярные disaster recovery drills.

### Стоимость митигации

- **Инфра:** `+$50–300/мес` за хранение копий и cross-region traffic, зависит от объёма документов.
- **Работы:** `1 инженер × 1–2 недели`.
- **Availability после митигации:** региональный отказ S3 не блокирует выдачу уже созданных документов; blast radius снижается с ~30% до ~5%.

---

# 3. Сводная таблица SPOF

| # | SPOF | Тип | Blast radius | Основной эффект | RiskScore | Приоритет | Митигация | ROM-стоимость |
|---|------|-----|--------------|-----------------|-----------|-----------|----------|---------------|
| 1 | WAF 1 instance | single instance | 100% | весь API недоступен | 0.33 | P0 | managed/multi-AZ WAF | $50–200/мес + 1 нед |
| 2 | NGINX LB 1 VM | single VM | 100% | весь сервис недоступен | 0.33 | P0 | managed LB или HA pair | $30–150/мес + 1 нед |
| 3 | auth-service 1 pod | single pod | ~80% | логин/booking/payment недоступны | 0.27 | P0 | 3 pod'а + anti-affinity | $30–100/мес + 1 нед |
| 4 | PostgreSQL no replica | single DB primary | ~100% | нет логина и бронирования | 1.00 | P0 | standby + auto failover | $150–500/мес + 2 нед |
| 5 | Redis 1 node | single cache | ~50% | деградация pricing | 0.22 | P1 | Redis Sentinel/Cluster | $100–300/мес + 1 нед |
| 6 | Elasticsearch 1 node | single search node | ~70% | поиск недоступен | 0.93 | P0 | ES cluster 3 nodes | $200–600/мес + 1–2 нед |
| 7 | RabbitMQ 1 broker | single broker | ~30% | теряются/стопорятся booking events | 0.15 | P1 | quorum queues + 3 brokers | $100–300/мес + 1–2 нед |
| 8 | Amadeus GDS | external vendor | ~70% | поиск и цены деградируют | 1.40 | P0 | второй GDS + cache fallback | $1k–10k/мес + 4–8 нед |
| 9 | Stripe | external vendor | ~25% | оплата недоступна | 0.17 | P1 | второй PSP + orchestration | vendor fees + 3–6 нед |
| 10 | S3 us-east-1 only | single region | ~30% | документы/билеты недоступны | 0.20 | P1 | cross-region replication | $50–300/мес + 1–2 нед |

---

# 4. Availability: грубая оценка до и после митигаций

## 4.1 Текущая ситуация

Для критического booking path цепочка выглядит так:

```text
DNS → CDN → WAF → LB → API → auth → booking → PostgreSQL → RabbitMQ → Stripe → SendGrid/S3
```

Даже если каждый компонент имеет условные 99.9%, длинная последовательная цепочка снижает системную availability.

Грубая оценка для критического пути:

| Компонент | Условная availability |
|----------|-----------------------|
| DNS Route 53 | 99.99% |
| CDN Cloudflare | 99.99% |
| WAF single instance | 99.5% |
| NGINX LB single VM | 99.5% |
| API 3 pods | 99.99% |
| auth-service 1 pod | 99.5% |
| booking-service 3 pods | 99.99% |
| PostgreSQL primary only | 99.5% |
| RabbitMQ 1 broker | 99.8% |
| Stripe | 99.9% |

```text
A_system ≈ 0.9999 × 0.9999 × 0.995 × 0.995 × 0.9999 × 0.995 × 0.9999 × 0.995 × 0.998 × 0.999
A_system ≈ 98.38%
```

**Вывод:** текущая архитектура не достигает NFR 99.9% для критического booking path. Главные причины – одиночные stateful-компоненты и одиночный edge path.

## 4.2 После митигации P0

Если исправить P0:

- WAF → managed/multi-AZ;
- LB → managed/multi-AZ;
- auth-service → 3 pod'а;
- PostgreSQL → standby + auto failover;
- Elasticsearch → 3 nodes;
- Amadeus → circuit breaker + cache fallback + второй поставщик хотя бы для части inventory.

Тогда критический booking path примерно становится:

| Компонент | Availability после митигации |
|----------|-------------------------------|
| DNS Route 53 | 99.99% |
| CDN Cloudflare | 99.99% |
| WAF HA | 99.99% |
| Managed LB | 99.99% |
| API 3 pods | 99.99% |
| auth-service 3 pods | 99.99% |
| booking-service 3 pods | 99.99% |
| PostgreSQL HA | 99.95–99.99% |
| RabbitMQ пока single | 99.8% |
| Stripe пока single vendor | 99.9% |

```text
A_system ≈ 99.5–99.7%
```

Это всё ещё может быть ниже 99.9%, потому что остаются внешние и асинхронные зависимости. Поэтому следующим шагом нужны P1-митигации: RabbitMQ cluster, payment fallback, S3 cross-region replication.

---

# 5. Итоговый план работ

| Приоритет | SPOF | Почему первым | Митигация | Стоимость | Срок | Снижает риск на |
|----------|------|---------------|-----------|-----------|------|-----------------|
| P0 | PostgreSQL primary only | ломает логин, booking и оплату | standby replica + auto failover | $150–500/мес | 2 недели | ~100% outage → failover 1–5 мин |
| P0 | Load Balancer 1 VM | единая точка входа | managed LB / HA pair | $30–150/мес | 1 неделя | 100% outage → ~0% |
| P0 | WAF 1 instance | стоит перед всем API | managed/multi-AZ WAF | $50–200/мес | 1 неделя | 100% outage → ~0–5% |
| P0 | auth-service 1 pod | блокирует все авторизованные сценарии | 3 pod'а + anti-affinity | $30–100/мес | 0.5–1 нед | 80% impact → ~0–10% |
| P0 | Elasticsearch 1 node | ломает начало purchase funnel | ES cluster 3 nodes | $200–600/мес | 1–2 нед | 70% impact → ~0–10% |
| P0 | Amadeus GDS | внешний поставщик ломает поиск и цены | CB + cache + второй provider | $1k–10k/мес | 4–8 нед | 70% impact → ~20–30% |
| P1 | RabbitMQ 1 broker | риск потери booking events | quorum queues + 3 brokers + outbox | $100–300/мес | 1–2 нед | 30% impact → ~0–5% |
| P1 | Redis 1 node | каскад на Amadeus/pricing | Redis Sentinel/managed Redis | $100–300/мес | 1 нед | 50% impact → ~5–10% |
| P1 | Stripe only | прямая потеря оплат | второй PSP + orchestration | vendor fees | 3–6 нед | 25% impact → ~5–10% |
| P1 | S3 us-east-1 only | региональный риск после оплаты | CRR + fallback bucket | $50–300/мес | 1–2 нед | 30% impact → ~5% |

---

# 6. ADR-006: SPOF Mitigation Strategy

## Статус

Proposed

## Контекст

Текущая архитектура flight-booking сервиса содержит несколько SPOF:

1. WAF – один instance.
2. Load Balancer – одна VM.
3. auth-service – один pod.
4. PostgreSQL – primary без replica.
5. Redis – один node.
6. Elasticsearch – один node.
7. RabbitMQ – один broker.
8. Amadeus GDS – единственный внешний поставщик inventory.
9. Stripe – единственный платёжный провайдер.
10. S3 используется только в регионе `us-east-1`.

Самые опасные риски:

- PostgreSQL primary без replica: отказ блокирует логин, бронирование и оплату.
- Amadeus GDS: внешний vendor SPOF для поиска и цен.
- Edge path: WAF и LB являются последовательными single points of failure.
- Elasticsearch: отказ ломает поиск, то есть начало воронки покупки.

Текущая грубая availability booking path оценивается примерно в **98.38%**, что ниже целевого уровня **99.9%**.

## Решение

В первую очередь митигируем P0 SPOF:

1. PostgreSQL перевести в HA-топологию: primary + standby replica + auto failover.
2. Load Balancer заменить на managed/multi-AZ LB или HA pair.
3. WAF перевести в managed/multi-AZ режим.
4. auth-service масштабировать до 3 pod'ов с anti-affinity.
5. Elasticsearch перевести в кластер из 3 nodes.
6. Для Amadeus добавить circuit breaker, cache fallback и начать интеграцию второго поставщика.

## Последствия

Положительные:

- отказ одного edge-компонента больше не приводит к полному outage;
- отказ PostgreSQL primary превращается из ручного восстановления в контролируемый failover;
- поиск становится устойчивее к отказу одной ES-ноды;
- отказ Amadeus перестаёт полностью ломать поиск, появляется degraded mode.

Отрицательные:

- растёт monthly infra cost примерно на `$500–1,500/мес` без учёта второго GDS;
- появляется дополнительная операционная сложность: failover tests, runbooks, мониторинг репликации;
- интеграция второго GDS дорогая и долгая;
- HA PostgreSQL требует дисциплины миграций, backup/restore drills и контроля split-brain.

## Отложенные решения

Не чиним сразу всё:

- **Stripe multi-provider** откладываем на P1: дорого, требует юридической и финансовой интеграции. До этого делаем idempotency keys, retry с backoff и возможность удержать booking на 15–30 минут.
- **S3 cross-region** откладываем на P1: важно для post-payment документов, но не главный источник полного outage.
- **RabbitMQ cluster** делаем после PostgreSQL HA, потому что без БД booking всё равно не работает.
- **Redis cluster** делаем после edge/PostgreSQL/search, потому что Redis failure чаще даёт деградацию, а не полный outage.

---

# 7. Что сознательно НЕ чиним прямо сейчас

1. **Полный multi-region active-active для всего сервиса.**
   - Слишком дорого и сложно для первого этапа.
   - Сначала достаточно multi-AZ и устранения одиночных stateful-компонентов.

2. **Полная замена всех внешних vendors.**
   - Multi-vendor нужен для Amadeus и Stripe, но это разные по стоимости задачи.
   - Для Amadeus начинаем с cache fallback и circuit breaker.
   - Для Stripe сначала делаем idempotency и удержание booking.

3. **Абсолютная доступность 99.99% для всех сценариев.**
   - Для учебного этапа целимся в 99.9% для основных сценариев.
   - Более высокий SLO потребует отдельного TCO/ADR.

---

# 8. Короткий вывод

В архитектуре найдено больше 5 SPOF. Самые критичные:

1. **PostgreSQL без replica** – ломает booking и auth.
2. **Amadeus как единственный GDS** – внешний SPOF для поиска и цен.
3. **WAF + NGINX LB как одиночный edge path** – отказ до API ломает весь сервис.
4. **auth-service в одном pod'е** – ломает авторизацию и покупку.
5. **Elasticsearch в одном node** – ломает поиск.

Первый этап митигации должен закрыть P0-риски: БД, edge path, auth, search и внешний GDS. После этого можно переходить к P1: Redis, RabbitMQ, Stripe и S3 cross-region.
