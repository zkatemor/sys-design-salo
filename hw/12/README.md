# ДЗ-12. Алерты и runbooks

| Файл | Описание |
|------|----------|
| [../hw_12_alerts_runbooks.md](../hw_12_alerts_runbooks.md) | Полный документ: алерты + 3 runbook |
| [../hw_11_slo.md](../hw_11_slo.md) | SLO и пороги |
| [alerts/travel-platform-alerts.yaml](alerts/travel-platform-alerts.yaml) | Prometheus rules |
| [alerts/travel-platform-alerts_test.yaml](alerts/travel-platform-alerts_test.yaml) | promtool test |
| [alertmanager.yml](alertmanager.yml) | Маршрутизация + inhibit |

## Три алерта (по заданию)

1. **SearchHighErrorRate** – error rate > 1% (SLO availability 99.9%)
2. **SearchLatencyP99High** – p99 > 2s (NFR p95 < 1.5s)
3. **SearchSaturationHigh** – CPU > 85% или DB pool > 80%

## Проверка promtool

```bash
cd sys-design-salo/hw/12
promtool test rules alerts/travel-platform-alerts_test.yaml
```
