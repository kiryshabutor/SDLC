# Графические материалы SRS на SysMLv2

Исходные модели для рисунков Приложения А документа [`../srs.md`](../srs.md). Готовые PNG лежат в [`../images/`](../images/).

| Файл | Содержание | PNG |
|------|-----------|-----|
| `context.sysml` | Контекстная диаграмма: Система, её компоненты и внешние сервисы | `context.png` |
| `use-cases.sysml` | Варианты использования и акторы | `use-cases.png` |
| `ui-kit.sysml` | Библиотека элементов интерфейса для макетов | — |
| `ui-01-auth.sysml` … `ui-06-profile.sysml` | Макеты экранов | `ui-01-auth.png` … `ui-06-profile.png` |

## Пересборка изображений

Нужен только Docker. Из каталога `srs/sysml`:

```sh
sh tools/render-all.sh
```

Скрипт собирает образ из [`tools/Dockerfile`](tools/Dockerfile), а именно SysML v2 Pilot Implementation 0.62.0 (Jupyter-ядро `jupyter-sysml-kernel`), Graphviz и Java 21. Затем он загружает модели в ядро и выполняет для каждого рисунка команду `%viz` (см. [`tools/render.py`](tools/render.py)):

- контекстная диаграмма — `%viz --view=interconnection --style=LR Context::ctx`;
- варианты использования — `%viz --view=case --style=LR UseCases`;
- макеты — `%viz --view=interconnection`. Для макетов в сгенерированный ядром PlantUML-код добавляются невидимые связи, чтобы элементы `Row` и `ListItem` выстраивались в строку. На саму модель это не влияет.

Модели можно также открыть в JupyterLab с ядром SysML или в Eclipse с плагином SysML v2 Pilot Implementation.
