# Movie Downloader

CLI-приложение на Go для загрузки информации о фильмах с `homeworksite.site`.

Используется **worker pool**, поэтому несколько фильмов загружаются параллельно.

## Запуск

Сборка:

```bash
go build -o movie-downloader .
```

Запуск:

```bash
./movie-downloader --from=1 --to=100
```

Windows:

```powershell
.\movie-downloader.exe --from=1 --to=100
```

## Флаги

* `--from` — ID первого фильма, обязательный.
* `--to` — ID последнего фильма, обязательный.
* `--workers` — количество воркеров, по умолчанию `10`.
* `--timeout` — таймаут HTTP-запроса, по умолчанию `5s`.

Пример:

```bash
./movie-downloader --from=1 --to=100 --workers=10 --timeout=3s
```

Результат:

```text
id — title — year — director
```

Ошибки отдельных фильмов не останавливают программу. При `Ctrl+C` незавершённые запросы отменяются через `context`.

## Производительность

Для диапазона `1..100`:

| Workers |   Время |
| ------: | ------: |
|       1 | 2.751 s |
|      10 | 0.348 s |

При `10` workers загрузка выполняется примерно в **7.9 раза быстрее**, чем при одном worker.

Измерение выполнялось в PowerShell через `Measure-Command`.
