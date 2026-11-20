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

Выполнены итерации 1 и 2 плана v3: каркас проекта, CI, миграции, поиск и добавление игр через API, `/healthz`, журналы, резервное копирование; регистрация, вход и серверные сессии, выбор региона, настройки уведомлений, удаление учётной записи, файлы локализации. Подробности: раздел 7 плана.

## Структура

```text
cmd/api, cmd/worker      точки входа API-сервера и фонового обработчика
internal/domain          сущности и чистые правила (без ввода-вывода)
internal/app             сценарии использования (UC-01, UC-02, UC-04, UC-05, UC-08, UC-15)
internal/source          интерфейсы источников; steam/ и mock/ — реализации
internal/store/postgres  репозитории, миграции goose
internal/httpapi         REST API /api/v1 и /healthz
internal/platform        конфигурация, журналы JSON, ограничитель частоты, повторы, Argon2id, токены
internal/i18n            файлы локализации (locales/ru.json)
pkg/steamstore           клиент Steam Store API (переиспользуемый)
migrations/              SQL-миграции
deploy/backup.sh         резервное копирование БД
```

## Запуск

Нужны Docker и Docker Compose.

```bash
cp .env.example .env          # задайте POSTGRES_PASSWORD
docker compose up -d --build
curl http://localhost:8080/healthz
```

Без Steam (имитированные цены): `TEST_MODE=true` в `.env`.

### Примеры запросов

API использует cookie сессии `sid` и защиту CSRF: изменяющие запросы передают заголовок `X-CSRF-Token` со значением cookie `csrf` (оно же возвращается в поле `csrf_token` при входе).

```bash
B=http://localhost:8080/api/v1
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
| `GET /me`, `PATCH /me`, `DELETE /me` | Профиль; регион и каналы уведомлений; удаление с паролем |
| `GET /regions` | Список регионов |
| `GET /games/search?q=`, `GET /wishlist`, `POST /wishlist` | Поиск и вишлист |

## Разработка

```bash
go test ./...                                   # модульные тесты
TEST_DATABASE_URL=postgres://… go test ./...    # плюс интеграционные с PostgreSQL
golangci-lint run ./...                         # v2, конфигурация .golangci.yml
./uml/render.sh                                 # пересборка UML-диаграмм
```

Резервное копирование: контейнер `backup` раз в сутки; вручную `docker compose exec backup sh /usr/local/bin/backup.sh`, восстановление `… backup.sh --restore /backups/<файл>`.

## Лицензия

Учебный проект, см. [LICENSE.md](LICENSE.md).
