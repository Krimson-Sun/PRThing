# Тестовое задание: PR Reviewer Assignment Service

## Задача

Нужно реализовать HTTP‑сервис, который:

- управляет командами (`Team`) и пользователями (`User`);
- создаёт Pull Request’ы (`PullRequest`) и автоматически назначает ревьюеров из команды автора;
- позволяет переназначать ревьюера;
- позволяет смотреть список PR, назначенных конкретному ревьюеру;
- возвращает статистику назначений;
- поддерживает массовую деактивацию участников команды с безопасным переназначением открытых PR.

Взаимодействие — через REST API, описанный в `openapi.yml`.

## Сущности

### User

- Поля: `user_id`, `username`, `team_name`, `is_active`.
- Только активный пользователь (`is_active = true`) может быть ревьюером.

### Team

- Поле: уникальное `team_name`.
- Набор участников — список `User`.

### PullRequest

- Поля: `pull_request_id`, `pull_request_name`, `author_id`, `status` (`OPEN` или `MERGED`).
- `assigned_reviewers`: массив 0..2 `user_id`.

### Бизнес‑правила

1. При создании PR автоматически выбираются до двух активных ревьюеров из **команды автора**, сам автор исключается.
2. Переназначение заменяет одного ревьюера на другого **активного участника той же команды**, исключая автора и уже назначенных.
3. После перевода PR в `MERGED` список ревьюеров менять нельзя.
4. Если доступных кандидатов меньше двух, назначается доступное количество (0/1).

## Основной функционал (реализовано)

- `POST /team/add` — создать команду с участниками.
- `GET /team/get` — получить команду с участниками.
- `POST /users/setIsActive` — изменить флаг активности пользователя.
- `GET /users/getReview` — получить список PR, где пользователь назначен ревьюером.
- `POST /pullRequest/create` — создать PR и автоматически назначить ревьюеров.
- `POST /pullRequest/merge` — пометить PR как `MERGED` (операция идемпотентна).
- `POST /pullRequest/reassign` — заменить одного ревьюера в PR на другого из команды.
- `GET /stats/assignments` — вернуть статистику:
  - `by_user[user_id] = количество назначений`;
  - `by_pr[pull_request_id] = количество ревьюеров`.
- `POST /users/deactivateTeamMembers` — массово деактивировать участников команды и безопасно переназначить их открытые PR.

Все контракты строго соответствуют `openapi.yml` (включая схемы ошибок и enum кодов).

## Нефункциональные требования (реализовано)

- Хранилище: PostgreSQL 15+, миграции goose (`migrations/*.sql`).
- Язык: Go 1.21+.
- Архитектура: Clean Architecture — слои `domain/`, `repository/`, `service/`, `handler/`, плюс `cmd/pr-service/main.go` для DI.
- Логирование: zap (`internal/logger`, `internal/app/middleware/logging.go`, `recovery.go`, `errors.go`).
- Конфигурация: `config.yaml` + `internal/config/config.go`, переопределение через ENV в Docker.
- Docker/Docker Compose: `Dockerfile` + `docker-compose.yml` поднимают Postgres, сервис (порт 8080) и Swagger UI (порт 8081).

## Дополнительные задания (реализовано)

### 1. Эндпоинт статистики

`GET /stats/assignments`:

```json
{
  "by_user": {
    "u1": 5,
    "u2": 3
  },
  "by_pr": {
    "pr-1001": 2,
    "pr-1002": 1
  }
}
```

Реализация: репозиторий `PRRepository.GetAssignmentStatsByUser/ByPR`, сервис `pullrequest.Service.GetAssignmentStats`, хендлер `internal/handler/stats.go`.

### 2. Нагрузочное тестирование

Бенчмарк `BenchmarkBulkDeactivateTeamMembers` в `internal/service/user/service_test.go` моделирует 20 активных пользователей и 50 открытых PR.  
Результат на тестовом стенде: ~86 000 ns/op (≈0.086 ms) — намного лучше порога 100 ms для средних объёмов.

### 3. Массовая деактивация и безопасное переназначение PR

- Доменная структура `Reassignment` (`internal/domain/reassignment.go`):  
  `Reassignment { pull_request_id, old_user_id, new_user_id }`.
- Сервис `user.Service.BulkDeactivateTeamMembers`:
  - деактивирует заданных пользователей команды;
  - находит все открытые PR, где они были ревьюерами;
  - подбирает новых ревьюеров из активных членов команды, исключая автора и текущих ревьюеров;
  - фиксирует все перестановки в `[]Reassignment`;
  - всё выполняется атомарно в рамках транзакции.
- Эндпоинт `POST /users/deactivateTeamMembers`:
  - Request:
    ```json
    { "team_name": "backend", "user_ids": ["u2", "u3"] }
    ```
  - Response (схематично):
    ```json
    {
      "team_name": "backend",
      "deactivated_user_ids": ["u2", "u3"],
      "reassignments": [
        {
          "pull_request_id": "pr-1001",
          "old_user_id": "u2",
          "new_user_id": "u4"
        }
      ],
      "team_members": [
        { "user_id": "u1", "username": "Alice", "is_active": true },
        { "user_id": "u2", "username": "Bob",   "is_active": false },
        ...
      ]
    }
    ```

### 4. HTTP E2E тест

`internal/e2e/http_e2e_test.go` поднимает полноценный HTTP‑стек (handlers + middleware) на `httptest.Server`, используя in‑memory репозитории, и выполняет сценарий end‑to‑end:

- `POST /team/add` — создание команды.
- `POST /pullRequest/create` (2 раза) — создание PR.
- `POST /pullRequest/reassign` — переназначение ревьюера.
- `POST /pullRequest/merge` (дважды) — проверка идемпотентности.
- `GET /stats/assignments` — проверка статистики.
- `POST /users/deactivateTeamMembers` — массовая деактивация и reassignment.
- `GET /users/getReview` — проверка, что PR ушёл от старого ревьюера к новому.

Запуск: обычный `go test ./...`.

## Проверка и запуск

### Тесты

```bash
go test ./...
go test -run=^$ -bench BulkDeactivateTeamMembers ./internal/service/user
```

### Линтер

```bash
golangci-lint run
```

Конфигурация — файл `.golangci.yml` в корне.

### Docker

```bash
docker compose up --build
```

После старта:

- Health‑check: `GET http://localhost:8080/health`
- Swagger UI: `http://localhost:8081`
- OpenAPI: `http://localhost:8080/openapi.yml`

