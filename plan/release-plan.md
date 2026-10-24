# План реализации R1

Для веб-сервиса «Охотник за скидками в Steam»

Подготовил Бутор К. П., гр. 450503

БГУИР, кафедра ЭВМ

## История изменений

| Name | Date | Reason For Changes | Version |
|------|------|--------------------|---------|
| Бутор К. П. | 24.10.2026 | Бэклог функций с приоритетами и оценками по SRS v0.2 и SDD v0.1 | 0.1 |

---

## 1. Исходные данные

| Параметр | Значение |
|----------|----------|
| Длина итерации | 1 неделя |
| Срок R1 (дедлайн) | 6 итераций |
| Команда | 1 разработчик уровня middle (Бутор К. П.) |
| Рабочее время | 40 ч в неделю, из них 8 ч на планирование, демонстрацию Заказчику, ревью и исправление ошибок |
| Ёмкость итерации | 32 ч разработки |
| Ёмкость R1 | 6 × 32 = **192 ч** |
| Ставка middle | 15 USD/ч |
| Ставка junior | 7 USD/ч |
| Стоимость R1 при одном разработчике | 6 × 40 ч × 15 USD = **3600 USD** |

Оценки даны в идеальных часах разработки и включают модульные тесты. Шкала приоритетов: **10** — без этого продукт не работает, **20** — важно для R1, **30** — нужно, но можно упростить, **40** — желательно, **50** — можно отложить.

## 2. Бэклог функций

| ID | Feature | Priority | Оценка, ч | Зависит от | Требования SRS | SDD |
|----|---------|:--------:|:---------:|------------|----------------|-----|
| F-01 | Project skeleton, Docker Compose, CI, DB migrations | 10 | 16 | — | REQ-INST-001…003, REQ-BUILD-001…002, REQ-MAINT-002 | DV-003, DV-014 |
| F-02 | User registration, login, sessions | 10 | 20 | F-01 | REQ-FUNC-001, 003, REQ-SEC-001, 003, 005, 007 | DV-005, DV-008 |
| F-03 | Email verification, password reset, mail outbox and SMTP | 20 | 14 | F-02 | REQ-FUNC-002, 004, REQ-SEC-006, REQ-INT-010, REQ-REL-005 | DV-008, DV-013 |
| F-04 | Region selection | 10 | 4 | F-02 | REQ-FUNC-005 | DV-011 |
| F-05 | Game search and add by link or appid | 10 | 12 | F-02 | REQ-FUNC-008, 009, 011, REQ-INT-006 | DV-005, DV-006 |
| F-06 | Wishlist view, sorting, filtering, removal | 10 | 14 | F-05 | REQ-FUNC-012…014, 016 | DV-005 |
| F-07 | Notification conditions per game | 10 | 8 | F-06 | REQ-FUNC-015 | DV-010 |
| F-08 | Price polling worker with rate limiting and price history | 10 | 28 | F-01, F-05 | REQ-FUNC-017, 018, REQ-INT-007, REQ-REL-001, 002, 006, REQ-PERF-004, REQ-POC-002 | DV-007, DV-012 |
| F-09 | Notification engine and in-app notification center | 10 | 14 | F-07, F-08 | REQ-FUNC-025, 026, 028 | DV-010, DV-011 |
| F-10 | Email digest after each cycle and unsubscribe link | 20 | 10 | F-03, F-09 | REQ-FUNC-027, 029 | DV-013 |
| F-11 | Daily digest mode | 40 | 6 | F-10 | REQ-FUNC-006, 027 | DV-013 |
| F-12 | Game page with price chart and historical low | 20 | 18 | F-08 | REQ-FUNC-021…023 | DV-005, DV-010 |
| F-13 | Steam wishlist import | 30 | 14 | F-06, F-08 | REQ-FUNC-010, REQ-INT-008 | DV-006, DV-009 |
| F-14 | ITAD history backfill and world sales block | 30 | 16 | F-12 | REQ-FUNC-019, 024, REQ-INT-009, REQ-REL-004, REQ-COMP-002 | DV-006 |
| F-15 | Manual "Check prices now" | 30 | 6 | F-08 | REQ-FUNC-020 | DV-009 |
| F-16 | Test mode with mock price source | 20 | 8 | F-08 | REQ-FUNC-030 | DV-006, DV-014 |
| F-17 | Notification settings and account deletion | 30 | 6 | F-02 | REQ-FUNC-006, 007, REQ-COMP-001 | DV-004 |
| F-18 | Health check, structured logs, backups | 30 | 10 | F-01 | REQ-OBS-001…004, REQ-AVAIL-002, 003 | DV-014 |
| F-19 | UI polish: responsive layout, accessibility, empty and error states | 40 | 12 | F-06, F-12 | REQ-INT-002…005, REQ-PORT-002 | DV-003 |
| F-20 | Security hardening and acceptance tests (ZAP, coverage, load test) | 20 | 14 | все | REQ-SEC-009, REQ-MAINT-003, REQ-PERF-001…004 | — |
| F-21 | Localization files for UI and emails | 50 | 4 | F-01 | REQ-INT-001 | DV-003 |
| | **Итого** | | **254** | | | |

## 3. Сравнение с ёмкостью

| | Часы | Итерации по 32 ч |
|---|---|---|
| Оценка всего бэклога | 254 | 7,9 |
| Ёмкость до дедлайна | 192 | 6 |
| **Не помещается** | **62** | **≈ 2** |

Весь объём R1 при одном разработчике занимает около восьми недель против шести по сроку. Перед составлением плана по итерациям приоритеты и состав R1 согласуются с Заказчиком.

### 3.1 Почему крупные функции крупные

| Функция | Оценка | Из чего складывается |
|---------|:------:|----------------------|
| F-08 Price polling worker | 28 ч | Клиент Steam с пакетными запросами (6 ч), ограничитель 150 запросов за 5 мин и пауза после 429 (5 ч), повторы и изоляция ошибок (3 ч), запись истории только при изменении и идемпотентность (6 ч), блокировка единственного цикла и планировщик (3 ч), проверка размера пакета `appdetails` REQ-POC-002 (2 ч), статистика цикла (3 ч) |
| F-02 Registration, login | 20 ч | Формы и валидация на клиенте (5 ч), хэширование Argon2id и сессии (5 ч), CSRF (2 ч), блокировка после 5 попыток и одинаковые ответы (4 ч), тесты безопасности (4 ч) |
| F-12 Game page | 18 ч | Эндпоинты страницы и истории (4 ч), ступенчатый график с периодами и подсказками (8 ч), исторический минимум и пометка «История неполная» (3 ч), выделение участков ITAD (3 ч) |
| F-14 ITAD backfill | 16 ч | Клиент ITAD: поиск ID, история, минимум (6 ч), фоновая загрузка с повторами (4 ч), блок «Скидки в мире» (4 ч), атрибуция и работа без ключа (2 ч) |
| F-13 Wishlist import | 14 ч | Разбор двух форматов ссылки и ResolveVanityURL (4 ч), получение вишлиста и лимит 500 (3 ч), фоновое задание с прогрессом (4 ч), итог «добавлено, было, пропущено» и повторный импорт (3 ч) |

## 4. План по итерациям

Составляется после согласования приоритетов с Заказчиком (первый раунд обсуждения в pull request).
