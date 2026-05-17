# ДЗ-8. Репликация и консистентность

Проектирование схемы репликации для учебного сервиса бронирования (занятие 8).

| Файл | Описание |
|------|----------|
| [../hw_8_replication-adr.md](../hw_8_replication-adr.md) | ADR-008: топология, W/R/N, отказы, идемпотентность, PACELC |

## Содержание решения

1. Топология: **primary + 2 replica**, semi-sync (Patroni).
2. Таблица компонентов: Bookings DB, Redis cache, Analytics, Session store.
3. W/R/N для операций: create booking, my bookings, search flights.
4. Таблица поведения при 5 сценариях отказа.
5. Idempotency-Key для `POST /bookings`.
6. Consistency models и PACELC по операциям.

Код (DBPool, тесты) — опциональное расширение по чек-листу занятия; в этой сдаче — design-only.
