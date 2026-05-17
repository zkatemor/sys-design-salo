# ДЗ-16. План выката ranking v2

| Файл | Описание |
|------|----------|
| [../hw_16_ranking_rollout_plan.md](../hw_16_ranking_rollout_plan.md) | Полный план выката |

## Содержание

- Стратегия: **Canary** + **Unleash** feature flag
- Rollout: **0% → 1% → 5% → 25% → 50% → 100%**
- Метрики и gates на каждом этапе
- Rollback: killswitch + Argo abort
- Коммуникационный план (T-7d … rollback)

## Связанные ДЗ

- [hw_11_slo.md](../hw_11_slo.md) – пороги SLO
- [hw_13_platform_checklist.md](../hw_13_platform_checklist.md) – Unleash
- [hw_12_alerts_runbooks.md](../hw_12_alerts_runbooks.md) – алерты отката
- [hw/15](../hw/15/) – прогрев кэша после деплоя
