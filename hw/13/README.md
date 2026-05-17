# ДЗ-13. Платформа vs своё – чек-лист критериев

| Файл | Описание |
|------|----------|
| [../hw_13_platform_checklist.md](../hw_13_platform_checklist.md) | Чек-лист + обоснование для 3 сценариев |

## Три сценария

| Сценарий | Выбор | Инструмент |
|----------|-------|------------|
| **А** Rate limiting | Платформа | Kong API Gateway + Redis |
| **Б** Feature flags | Платформа | Unleash (self-hosted) |
| **В** Distributed tracing | Платформа | OpenTelemetry + Grafana Tempo |

## Контекст

- NFR и нагрузка: [../hw_2.md](../hw_2.md)
- Tracing уже в [../hw/10](../hw/10/)
- SLO и rollout: [../hw_11_slo.md](../hw_11_slo.md)
