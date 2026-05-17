# ДЗ-11. SLO / SLI / SLA

Определение SLO для учебного сервиса поиска и бронирования на основе NFR из занятия 2.

| Файл | Описание |
|------|----------|
| [../hw_11_slo.md](../hw_11_slo.md) | SLO-карточка, PromQL, error budget, EBP |
| [../hw_2.md](../hw_2.md) | Исходные NFR |
| [../hw/10/README.md](../hw/10/README.md) | Метрики для PromQL |

## Содержание решения

- 4 SLI: availability (поиск + бронь), latency, throughput
- SLO с обоснованием из NFR ДЗ-2
- Error budget: **месяц** и **квартал**
- Error Budget Policy (green / yellow / red)
- PromQL для каждого SLI
