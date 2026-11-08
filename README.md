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

Выполнена итерация 1 плана v3 (02.11–08.11.2026): каркас проекта, CI, миграции, поиск и добавление игр через API, `/healthz`, журналы, резервное копирование. Подробности: раздел 7 плана.

## Структура

```text
cmd/api, cmd/worker      точки входа API-сервера и фонового обработчика
internal/domain          сущности и чистые правила (без ввода-вывода)
internal/app             сценарии использования (UC-04, UC-05, UC-08)
internal/source          интерфейсы источников; steam/ и mock/ — реализации
internal/store/postgres  репозитории, миграции goose
internal/httpapi         REST API /api/v1 и /healthz
internal/platform        конфигурация, журналы JSON, ограничитель частоты, повторы
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

```bash
curl "http://localhost:8080/api/v1/games/search?q=elden%20ring"
curl -X POST http://localhost:8080/api/v1/wishlist \
     -d '{"ref":"https://store.steampowered.com/app/1245620/ELDEN_RING/"}'
curl http://localhost:8080/api/v1/wishlist
```

До итерации 2 (регистрация и сессии) запросы выполняются от имени демонстрационного пользователя `DEV_USER_EMAIL`.

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
