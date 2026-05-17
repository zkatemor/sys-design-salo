# ДЗ-9. План chaos-эксперимента

Тестирование отказоустойчивости для сервиса бронирования (занятие 9), на базе архитектуры из [ДЗ-8](../hw_8_replication-adr.md).

| Файл | Описание |
|------|----------|
| [../hw_9_chaos-experiment-plan.md](../hw_9_chaos-experiment-plan.md) | План эксперимента: primary kill при пиковой нагрузке |

## Содержание плана

- Гипотеза (RTO < 30 с, RPO = 0, идемпотентность)
- Steady state (метрики и пороги)
- Инъекция: Litmus `pod-delete` на PostgreSQL primary
- Метрики наблюдения (Prometheus, логи, canary)
- Критерии успеха / провала
- Rollback plan (авто + ручной)
- Runbook по времени
