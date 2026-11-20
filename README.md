# Охотник за скидками в Steam

Проект по дисциплине «Жизненный цикл разработки программного обеспечения», БГУИР, кафедра ЭВМ.

- Автор: Бутор Кирилл Павлович, гр. 450503, вариант 4
- Язык: Go (серверная часть), Vue 3 (веб-клиент, с итерации 3)
- Хранилище: PostgreSQL 16, развёртывание Docker Compose

Веб-сервис отслеживает цены игр из вишлиста пользователя в выбранном регионе Steam (BY, RU, KZ, US, PL), хранит историю цен и уведомляет по email и в Telegram, когда выполняется заданное условие: любая скидка, скидка от N %, цена ниже X, исторический минимум.

## Документация

| Документ | Расположение |
|----------|--------------|
| Требования (SRS) | [srs/srs.md](srs/srs.md) |
| Дизайн (SDD) | [sdd/sdd.md](sdd/sdd.md) |
| Архитектурные решения | [adr/](adr/) |
| План реализации R1 | [plan/release-plan.md](plan/release-plan.md) |
| Глоссарий | [docs/glossary.md](docs/glossary.md) |
| Дополнительные спецификации | [docs/supplementary-spec.md](docs/supplementary-spec.md) |
| Модель прецедентов | [docs/use-case-model.md](docs/use-case-model.md) |
| UML-диаграммы (PlantUML) | [uml/](uml/) |

## Состояние

Выполнены итерации 1–3 плана v3: каркас, CI, миграции, поиск и добавление игр, `/healthz`, журналы, резервное копирование; регистрация, вход, сессии, регион, настройки, удаление учётной записи, локализация; подтверждение email, сброс пароля и отправка писем через outbox; веб-клиент с вишлистом, сортировкой, фильтрами и условиями уведомлений; привязка Telegram. Подробности: раздел 7 плана.

## Структура

```text
cmd/api, cmd/worker      точки входа API-сервера и фонового обработчика
internal/domain          сущности и чистые правила (без ввода-вывода)
internal/app             сценарии использования (UC-01, UC-02, UC-04, UC-05, UC-08, UC-15)
internal/source          интерфейсы источников; steam/ и mock/ — реализации
internal/store/postgres  репозитории, миграции goose
internal/httpapi         REST API /api/v1 и /healthz
internal/platform        конфигурация, журналы JSON, ограничитель частоты, повторы, Argon2id, токены
internal/i18n            файлы локализации (locales/ru.json): сообщения API, письма, бот
internal/mail            отправка писем по SMTP (STARTTLS / SMTPS)
internal/telegram        бот: привязка по коду, /stop
pkg/telegram             клиент Telegram Bot API (переиспользуемый)
web/                     веб-клиент: Vue 3, Vite, Vue Router, Pinia
deploy/Caddyfile         веб-клиент и прокси к API, HTTPS
pkg/steamstore           клиент Steam Store API (переиспользуемый)
migrations/              SQL-миграции
deploy/backup.sh         резервное копирование БД
```

## Запуск

Нужны Docker и Docker Compose.

```bash
cp .env.example .env          # задайте POSTGRES_PASSWORD
docker compose up -d --build
curl http://localhost/healthz
```

Сайт открывается по адресу http://localhost. Cookie сессии имеют атрибут `Secure`, поэтому по HTTP вход работает только через `localhost`; на сервере задайте доменное имя в `SITE_ADDRESS`, и Caddy включит HTTPS.

Демонстрация без Steam и без настоящей почты: в `.env` задайте `TEST_MODE=true`, `SMTP_HOST=mailpit`, `SMTP_PORT=1025`, `SMTP_TLS=none`, `APP_BASE_URL=http://localhost` и запустите `docker compose --profile test up -d --build`. Письма появятся в Mailpit: http://localhost:8025.

### Примеры запросов

API использует cookie сессии `sid` и защиту CSRF: изменяющие запросы передают заголовок `X-CSRF-Token` со значением cookie `csrf` (оно же возвращается в поле `csrf_token` при входе).

```bash
B=http://localhost/api/v1
J=(-s -b jar -c jar -H 'Content-Type: application/json')

curl "${J[@]}" $B/auth/register -d '{"email":"me@example.com","password":"secret123",
     "password_confirm":"secret123","region":"BY","consent":true}'
CSRF=$(awk '$6=="csrf"{print $7}' jar)

curl "${J[@]}" "$B/games/search?q=elden%20ring"
curl "${J[@]}" -H "X-CSRF-Token: $CSRF" $B/wishlist \
     -d '{"ref":"https://store.steampowered.com/app/1245620/ELDEN_RING/"}'
curl "${J[@]}" $B/wishlist
curl "${J[@]}" -X PATCH -H "X-CSRF-Token: $CSRF" $B/me -d '{"region":"PL","confirm_region_reset":true}'
curl "${J[@]}" -X POST  -H "X-CSRF-Token: $CSRF" $B/auth/logout
```

| Метод и путь | Назначение |
|--------------|------------|
| `POST /auth/register`, `POST /auth/login`, `POST /auth/logout` | Регистрация, вход, выход |
| `POST /auth/verify-email`, `POST /auth/resend-verification` | Подтверждение email |
| `POST /auth/password-reset`, `POST /auth/password-reset/confirm` | Сброс пароля по ссылке из письма |
| `GET /me`, `PATCH /me`, `DELETE /me` | Профиль; регион и каналы уведомлений; удаление с паролем |
| `POST /me/telegram/link`, `DELETE /me/telegram` | Код привязки Telegram; отвязка |
| `GET /regions` | Список регионов |
| `GET /games/search?q=`, `GET /wishlist?sort=&filter=`, `POST /wishlist` | Поиск и вишлист |
| `PATCH /wishlist/{appid}`, `DELETE /wishlist/{appid}` | Условие уведомления; удаление игры |

## Разработка

```bash
go test ./...                                   # модульные тесты
TEST_DATABASE_URL=postgres://… go test ./...    # плюс интеграционные с PostgreSQL
golangci-lint run ./...                         # v2, конфигурация .golangci.yml
cd web && npm ci && npm run dev                 # клиент на :5173, API проксируется на :8080
cd web && npm run lint && npm test              # eslint и vitest
./uml/render.sh                                 # пересборка UML-диаграмм
```

Резервное копирование: контейнер `backup` раз в сутки; вручную `docker compose exec backup sh /usr/local/bin/backup.sh`, восстановление `… backup.sh --restore /backups/<файл>`.

## Лицензия

Учебный проект, см. [LICENSE.md](LICENSE.md).
