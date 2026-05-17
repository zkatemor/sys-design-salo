# ДЗ-14. Async: оформление заказа с оплатой

| Файл | Описание |
|------|----------|
| [../hw_14_async_booking_payment.md](../hw_14_async_booking_payment.md) | Saga, sequence diagrams, события, happy path, 3 ошибки |

## Содержание документа

- Orchestration Saga + Outbox + Kafka
- 6 событий (JSON-схемы)
- Happy path (mermaid sequence)
- Ошибки: payment failed, seat unavailable, payment timeout
- Идемпотентность, машина состояний

## Контекст

- NFR: [../hw_2.md](../hw_2.md)
- Репликация / RPO=0: [../hw_8_replication-adr.md](../hw_8_replication-adr.md)
- Ранний ADR async notify: [../hw_5_async-sync-adr.md](../hw_5_async-sync-adr.md)
