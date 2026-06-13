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

## Повторные попытки (`--retries`)

Параметр `--retries` задаёт максимальное число **повторных** попыток после неудачного запроса. Поле в `Options` — `Retries`. Всего выполняется не более `retries + 1` обращений к одному URL.

Повтор выполняется только для временных ошибок:

- сетевые сбои;
- `429 Too Many Requests`;
- `5xx` (серверные ошибки по [RFC 7231, раздел 6.6](https://datatracker.ietf.org/doc/html/rfc7231#section-6.6)).

Коды `4xx` (кроме `429`) повторно не запрашиваются. Между попытками есть пауза 100ms, чтобы не создавать всплеск запросов. В отчёте (в том числе в `broken_links`) фиксируется результат **последней** попытки.

Примеры:

```bash
# До 3 обращений к URL (1 основной + 2 повтора)
go run ./cmd/hexlet-go-crawler --retries 2 https://example.com

# Без повторов
go run ./cmd/hexlet-go-crawler --retries 0 https://example.com
```

## Формат JSON-отчёта

Краулер выводит отчёт в формате [JSON](https://www.json.org/json-en.html). CLI печатает **только** JSON без дополнительного текста; в конце добавляется перевод строки. Флаг `--indent-json` влияет лишь на форматирование (пробелы и переносы), но не на содержимое и порядок ключей.

Эталонная структура отчёта:

```json
{
  "root_url": "https://example.com",
  "depth": 1,
  "generated_at": "2024-06-01T12:34:56Z",
  "pages": [
    {
      "url": "https://example.com",
      "depth": 0,
      "http_status": 200,
      "status": "ok",
      "error": "",
      "seo": {
        "has_title": true,
        "title": "Example title",
        "has_description": true,
        "description": "Example description",
        "has_h1": true
      },
      "broken_links": [
        {
          "url": "https://example.com/missing",
          "status_code": 404,
          "error": "Not Found"
        }
      ],
      "assets": [
        {
          "url": "https://example.com/static/logo.png",
          "type": "image",
          "status_code": 200,
          "size_bytes": 12345,
          "error": ""
        }
      ],
      "discovered_at": "2024-06-01T12:34:56Z"
    }
  ]
}
```

Все ключи обязательны. Пустые строки допускаются, но ключ не должен отсутствовать (например, `error` присутствует всегда).

### Поля верхнего уровня

| Поле | Описание |
|---|---|
| `root_url` | URL, с которого начался обход |
| `depth` | Значение параметра `--depth` (максимальная глубина внутренних переходов) |
| `generated_at` | Время формирования отчёта в формате ISO 8601 (UTC) |
| `pages` | Массив отчётов по каждой обработанной странице |

### Поля страницы (`pages[]`)

| Поле | Описание |
|---|---|
| `url` | Адрес страницы |
| `depth` | Глубина от `root_url` (стартовая страница — `0`) |
| `http_status` | HTTP-код ответа при загрузке страницы |
| `status` | `"ok"` при успешной загрузке (2xx), иначе `"error"` |
| `error` | Текст ошибки; пустая строка при успехе |
| `seo` | Результаты SEO-анализа HTML |
| `broken_links` | Список битых ссылок, найденных на странице |
| `assets` | Список статических ресурсов (`img`, `script`, `link[rel=stylesheet]`) |
| `discovered_at` | Время успешной загрузки страницы (ISO 8601, UTC); пусто при ошибке |

### SEO (`seo`)

| Поле | Описание |
|---|---|
| `has_title` | Есть ли непустой `<title>` |
| `title` | Текст заголовка |
| `has_description` | Есть ли непустой `<meta name="description">` |
| `description` | Содержимое meta description |
| `has_h1` | Есть ли хотя бы один непустой `<h1>` |

### Битая ссылка (`broken_links[]`)

| Поле | Описание |
|---|---|
| `url` | Адрес ссылки |
| `status_code` | HTTP-код ответа (или `0` при сетевой ошибке) |
| `error` | Текст ошибки; для HTTP-ошибок — стандартное описание статуса (например, `"Not Found"`) |

### Ресурс (`assets[]`)

| Поле | Описание |
|---|---|
| `url` | Адрес ресурса |
| `type` | Тип: `"image"`, `"script"` или `"style"` |
| `status_code` | HTTP-код ответа |
| `size_bytes` | Размер в байтах (из `Content-Length` или длины тела ответа) |
| `error` | Текст ошибки; пустая строка при успехе |

Пример с форматированием:

```bash
go run ./cmd/hexlet-go-crawler --indent-json https://example.com
```
