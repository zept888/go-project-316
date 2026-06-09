# hexlet-go-crawler

[![Go Tests](https://github.com/zept888/go-project-316/actions/workflows/go-test.yml/badge.svg)](https://github.com/zept888/go-project-316/actions/workflows/go-test.yml)
[![Hexlet Check](https://github.com/zept888/go-project-316/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/zept888/go-project-316/actions/workflows/hexlet-check.yml)

## Быстрый старт

Собрать проект:

```bash
make build
```

Запустить тесты:

```bash
make test
```

Запустить обход сайта:

```bash
make run URL=https://example.com
```

Если URL не указан, будет выведена справка:

```bash
make run
```

## Запуск напрямую

```bash
go run ./cmd/hexlet-go-crawler https://example.com
```

Справка по флагам:

```bash
go run ./cmd/hexlet-go-crawler --help
```

## Глубина обхода (`--depth`)

Параметр `depth` задаёт максимальную глубину переходов по ссылкам **внутри исходного домена**.

- Стартовая страница всегда имеет `depth = 0`.
- Страницы, на которые ведут ссылки с неё, получают `depth = 1`, и так далее.
- Краулер добавляет в очередь только внутренние ссылки, у которых глубина **меньше** значения `--depth`.
- Внешние ссылки не обходятся, но могут проверяться как потенциально битые на уже скачанных страницах.

Примеры:

```bash
# Только стартовая страница
go run ./cmd/hexlet-go-crawler --depth 1 https://example.com

# Стартовая страница и её прямые внутренние ссылки
go run ./cmd/hexlet-go-crawler --depth 2 https://example.com
```

В JSON-отчёте каждая страница встречается не более одного раза, а поле `depth` показывает расстояние от `root_url`.

## Ограничение скорости (`--delay`, `--rps`)

Краулер может замедлять **все HTTP-запросы процесса** (страницы и проверки ссылок), чтобы не перегружать сайт.

| Параметр | Поле в `Options` | Описание |
|---|---|---|
| `--delay` | `Delay` | Фиксированная пауза между соседними запросами (`200ms`, `1s`) |
| `--rps` | `RPS` | Целевое число запросов в секунду для всего процесса |

- Если заданы оба параметра, **приоритет у `--rps`**.
- Без `--delay` и `--rps` искусственного замедления нет.
- Ограничение глобальное: интервал соблюдается между любыми двумя последовательными запросами.

Примеры:

```bash
# Не более одного запроса каждые 200 мс
go run ./cmd/hexlet-go-crawler --delay 200ms https://example.com

# Не более 5 запросов в секунду
go run ./cmd/hexlet-go-crawler --rps 5 https://example.com
```
