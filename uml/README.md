# UML-модель (PlantUML)

Динамическая модель «Охотника за скидками в Steam», релиз R1. Правила построения: [ADR-0010](../adr/0010-uml-dynamic-model-plantuml.md), описание в SDD: DV-016.

| ID | Файл | Диаграмма |
|----|------|-----------|
| UC (1/2) | `use-cases.puml` | Прецеденты: учётная запись и вишлист |
| UC (2/2) | `use-cases-processing.puml` | Прецеденты: отслеживание цен и уведомления |
| AD-01 | `activity-poll-cycle.puml` | Активность: цикл опроса цен |
| AD-02 | `activity-add-game.puml` | Активность: добавление игры по ссылке или appid |
| SM-01 | `state-wishlist-item.puml` | Состояния элемента вишлиста |
| SM-02 | `state-outbox.puml` | Состояния записи outbox |
| SD-01 | `sequence-add-game.puml` | Последовательность: добавление игры |
| SD-02 | `sequence-discount-notify.puml` | Последовательность: скидка и доставка дайджеста |

Описания актёров и потоки событий: [docs/use-case-model.md](../docs/use-case-model.md). Глоссарий: [docs/glossary.md](../docs/glossary.md). Дополнительные спецификации: [docs/supplementary-spec.md](../docs/supplementary-spec.md).

## Сборка изображений

```bash
./uml/render.sh   # нужен Docker и plantuml.jar (переменная PLANTUML_JAR)
```

Скрипт запускает PlantUML в образе `sysml-render` (Java 21 + Graphviz, см. `srs/sysml/tools/Dockerfile`) и кладёт PNG в `uml/images/`. В VS Code диаграммы можно просматривать расширением PlantUML (Alt+D).
