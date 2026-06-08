# hexlet-go-crawler

### Hexlet tests and linter status:
[![Actions Status](https://github.com/zept888/go-project-316/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/zept888/go-project-316/actions)

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
