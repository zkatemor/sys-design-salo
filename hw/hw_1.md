# Разбор постмортема: Cloudflare - глобальный сбой WAF (2 июля 2019)

Источник постмортема:  
https://blog.cloudflare.com/details-of-the-cloudflare-outage-on-july-2-2019/

---

# Дата и длительность

Дата инцидента: **2 июля 2019**

Начало инцидента: ~13:42 UTC  
Восстановление сервиса: ~14:12 UTC  

Общая длительность: **27 минут**

В официальном постмортеме Cloudflare указано:

> "The outage lasted for 27 minutes."

---

# Impact

Инцидент затронул глобальную инфраструктуру Cloudflare, которая используется как CDN и Web Application Firewall для большого количества интернет-сервисов.

В официальном разборе указано:

> "This resulted in a CPU spike that caused roughly 10% of Internet traffic to return 502 errors."

В результате:

- пользователи получали **HTTP 502**
- наблюдался **резкий рост latency**
- были затронуты крупные сервисы (Discord, Shopify, Medium)

Поскольку Cloudflare работает на edge-инфраструктуре, сбой имел **глобальный blast radius**.

---

# Timeline

| Время | Событие |
|------|------|
| 13:42 | Deploy нового правила WAF |
| 13:42 | CPU на edge-серверах резко возрастает |
| 13:45 | Пользователи начинают получать HTTP 502 |
| 13:46 | Cloudflare обнаруживает проблему |
| 14:09 | Проблемное правило WAF отключено |
| 14:12 | Сервис полностью восстановлен |

В постмортеме указано:

> "Three minutes after the rule was deployed we started receiving alerts."

и

> "The rule was rolled back and traffic returned to normal."

---

# Корневая причина

Непосредственным триггером инцидента стало новое правило Web Application Firewall.

В официальном постмортеме Cloudflare указано:

> "The outage was caused by a single WAF rule that contained a regular expression susceptible to catastrophic backtracking."

Catastrophic backtracking — ситуация, при которой регулярное выражение имеет **экспоненциальную вычислительную сложность** для некоторых входных строк, что приводит к резкому росту CPU.

Однако сам regex не является корневой причиной инцидента. В постмортеме отмечено:

> "The regex was the trigger, not the root cause."

Инцидент стал возможен из-за архитектурных решений системы.

---

# Проектные решения, приведшие к инциденту

## 1. Использование PCRE regex engine

Cloudflare использовал **PCRE**, который допускает catastrophic backtracking.

> "The rule was executed using the PCRE engine."

PCRE не гарантирует линейную сложность выполнения.

---

## 2. Отсутствие staged rollout

Правило было задеплоено **сразу глобально**.

> "The rule was deployed globally within seconds."

Это резко увеличило **blast radius** ошибки.

---

## 3. Отсутствие performance testing regex

В тестах проверялась только корректность выражения.

> "We tested the correctness of the rule but did not measure its CPU impact."

---

## 4. Удаление CPU protection

Ранее существовал механизм защиты от чрезмерной загрузки CPU.

> "A CPU protection mechanism had previously existed but had been removed during refactoring."

---

# Swiss Cheese анализ

Инцидент соответствует **Swiss Cheese Model**, где несколько защитных механизмов одновременно оказались неэффективными.

| Слой защиты | Почему не сработал |
|---|---|
| Regex engine | PCRE допускает catastrophic backtracking |
| CPU protection | механизм защиты был удалён |
| Performance testing | не измерялась нагрузка на CPU |
| Staged rollout | правило было задеплоено глобально |

Cloudflare также отмечает:

> "Multiple failures aligned to allow this rule to take down our edge."

---

# Action items

После инцидента Cloudflare внедрил ряд изменений.

## Переход на RE2

> "We are moving to the RE2 engine which guarantees linear time execution."

RE2 исключает catastrophic backtracking.

---

## Staged rollout

Внедрён постепенный deploy правил WAF.

---

## CPU profiling

Добавлено тестирование влияния regex на CPU.

---

## Улучшение monitoring

Добавлены механизмы обнаружения аномальной загрузки CPU.

---

# Альтернативное архитектурное решение

| Решение | Эффект |
|---|---|
| Использование RE2 | гарантирует линейную сложность regex |
| Canary rollout | уменьшает blast radius |
| CPU guard | ограничивает влияние одного правила |
| Performance testing | выявляет worst-case сценарии |

Основная архитектурная идея — **ограничение blast radius и защита инфраструктуры от пользовательских правил**.

---

# Я Python-разработчик, в основном поддерживаю backend сервисы в NLP-проектах

И этот инцедент напрямую связан с системами обработки текста.

В NLP системах часто используются:

- регулярные выражения
- правила обработки текста
- пользовательские шаблоны

Такие правила могут иметь **непредсказуемую вычислительную сложность**.

Примеры:

- сложные regex для извлечения сущностей
- правила нормализации текста
- фильтрация пользовательских запросов

В своих NLP проектах я бы применила следующие практики:

1. Использование regex-движков с гарантированной сложностью (например **RE2**).
2. Ограничение времени выполнения операций обработки текста.
3. Performance-тестирование правил обработки текста.
4. Canary deployment новых правил или моделей.
5. Ограничение CPU и памяти для пользовательских вычислений.

---

# Вывод

Инцидент Cloudflare показывает, что небольшая ошибка конфигурации может привести к глобальному сбою, если архитектура системы не ограничивает её влияние.

Основные усвоенные уроки:

- конфигурация и DSL фактически являются исполняемым кодом  
- необходимо ограничивать **blast radius изменений**  
- алгоритмическая сложность может быть источником production-инцидентов  