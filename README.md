# TaskFlow

## 1. Overview

TaskFlow is a task management REST API. Users can register, log in, create projects, add tasks to those projects, and assign tasks to themselves or others.

### Tech Stack


| Layer            | Technology                                                                      |
| ---------------- | ------------------------------------------------------------------------------- |
| Language         | Go 1.25                                                                         |
| HTTP framework   | [Echo v4](https://echo.labstack.com/)                                           |
| Database         | PostgreSQL 16                                                                   |
| DB driver        | `database/sql` + [lib/pq](https://github.com/lib/pq)                            |
| Migrations       | [Goose v3](https://github.com/pressly/goose) (separate binary, embedded SQL)    |
| Auth             | JWT ([golang-jwt/jwt v5](https://github.com/golang-jwt/jwt), HS256, 24h expiry) |
| Password hashing | bcrypt (cost 12)                                                                |
| Logging          | [Zap](https://github.com/uber-go/zap) (structured JSON, production preset)      |
| Containerization | Docker (multi-stage build) + Docker Compose                                     |


### Project Structure

```
cmd/
  api/main.go               API server entry point, routing, graceful shutdown
  migrate/main.go           Migration binary for production (up/down/status/reset/etc.)
  migrate-create/main.go    Dev-only: scaffold new .sql migration files
  hashpw/main.go            Dev-only: generate bcrypt hashes for seed data
internal/
  config/                   Environment-based configuration
  database/                 PostgreSQL connection pool
  handler/                  HTTP handlers — auth, project, task
  middleware/               JWT bearer token validation
  model/                    Domain structs (User, Project, Task)
  repository/               Data access layer (raw SQL queries)
migrations/                 Goose SQL migrations + seed data (embedded via embed.FS)
integration_test.go         Integration tests
Dockerfile                  Multi-stage build (produces both taskflow + taskflow-migrate)
docker-compose.yml          PostgreSQL + migrate job + API
```

## 2. Architecture Decisions

### Why `database/sql` instead of an ORM

Go's standard `database/sql` package was chosen over ORMs like GORM or Ent. Every SQL query in the codebase is written explicitly, which means there are no hidden N+1 query problems — a common ORM pitfall where fetching a list of N parent records silently triggers N additional queries to load their related records (e.g., loading 20 projects and then the ORM quietly fires 20 separate queries to fetch each project's tasks, instead of a single `WHERE project_id IN (...)` query). With raw SQL, if 21 queries are executing, you wrote 21 queries, and the problem is immediately obvious. There are also no opaque query builders and no "magic" that makes debugging harder. When something goes wrong, you can read the exact SQL being executed. The trade-off is more boilerplate — every `SELECT` needs a manual `Scan()` call mapping columns to struct fields — but for a project of this size, that boilerplate is manageable and keeps the data access layer completely transparent.

### Repository pattern for separation of concerns

Handlers never touch `*sql.DB` directly. All SQL lives in the `repository/` package, while HTTP concerns (request parsing, response formatting, status codes) live in `handler/`. This separation means you can unit test the HTTP layer by mocking the repository interface, and you can test the repository layer against a real database without needing to spin up an HTTP server. It also makes it straightforward to swap the storage backend in the future without touching any handler code.

### Separate migration binary instead of embedding migrations in the API server

Migrations are compiled into a standalone `taskflow-migrate` binary, not run automatically when the API server starts. This is a deliberate decision for production safety. In a multi-replica deployment (e.g., 3 API pods in Kubernetes or multiple EC2 instances behind a load balancer), embedding migrations in the API server means every replica attempts to run migrations concurrently on startup. This creates race conditions — two instances might try to apply the same migration simultaneously, leading to partial applies or lock contention. By extracting migrations into a separate binary, you run it exactly once as a Kubernetes Job or ECS one-off task before rolling out the API replicas. The SQL files are still embedded into the binary via Go's `embed.FS`, so the Docker image needs no additional files on disk.

### Adding `creator_id` to the tasks table

The assignment requires that a task can be deleted by either the project owner or the task creator. The base data model in the spec doesn't include a `creator_id` field on tasks, so one was added. Without it, there would be no way to enforce the "task creator can delete their own task" rule. The spec explicitly permits adding new fields as long as the required fields are not removed or renamed.

### Why Echo over Gin or Chi

Echo was chosen for its balance of simplicity and functionality. It provides structured error handling out of the box (returning `*echo.HTTPError` from handlers instead of manually writing status codes), a clean middleware chain, and built-in request binding — all without the heavier dependency footprint of Gin. Compared to Chi, Echo offers slightly more convenience at the handler level while still being lightweight. For a project of this scope, the framework choice has minimal impact, but Echo's API ergonomics made handler code cleaner.

### Pagination on all list endpoints

Every list endpoint (`GET /projects`, `GET /projects/:id/tasks`) returns a paginated response with `{data, total, page, limit}`. This ensures clients always know how many total items exist and how many pages are available, making it straightforward to build paginated UIs or CLI tools that page through results. The defaults are page 1 with a limit of 20 items per page.

### Graceful shutdown

The API server listens for `SIGINT` and `SIGTERM` signals and initiates a graceful shutdown with a 10-second timeout. When a shutdown signal is received, the server stops accepting new connections but continues serving in-flight requests until they complete (or the 10-second deadline is reached). This prevents dropped requests during deployments and container restarts.

### What was intentionally left out

Certain features were deliberately omitted because they are outside the spec or would add complexity without being required:

- **Refresh tokens**: the spec asks for a single JWT with 24-hour expiry, so a refresh token rotation scheme was not implemented.
- **Role-based access control**: the only authorization checks needed are "project owner" and "task creator," so a full RBAC system would be over-engineering.
- **Rate limiting**: important for production but not part of the assignment scope.
- **Frontend**: the assignment is API-only.

## 3. Running Locally

Assumes the reviewer has Docker installed.

```bash
git clone https://github.com/RhoNit/taskflow-ranit-biswas.git
cd taskflow-ranit-biswas/backend
cp .env.example .env # set your env vars too
docker compose up --build
```

Docker Compose runs three services in order:

1. **postgres** — starts and waits until healthy (`pg_isready`)
2. **migrate** — runs `taskflow-migrate up` as a one-off job, applies all schema migrations and seed data, then exits
3. **api** — starts only after migrate finishes successfully

The API is available at **[http://localhost:8066](http://localhost:8066)**.

### Without Docker

Requires Go 1.25+ and a running PostgreSQL instance.

```bash
git clone https://github.com/RhoNit/taskflow-ranit-biswas.git
cd taskflow-ranit-biswas/backend
cp .env.example .env

# Edit .env — set DB_HOST=localhost, create the database:
#   createdb taskflow

# Run migrations
export $(grep -v '^#' .env | xargs)
go run ./cmd/migrate up

# Start the API server
go run ./cmd/api
```

### Environment Variables


| Variable      | Default                   | Description               |
| ------------- | ------------------------- | ------------------------- |
| `DB_HOST`     | `localhost`               | PostgreSQL host           |
| `DB_PORT`     | `5432`                    | PostgreSQL port           |
| `DB_USER`     | `taskflow`                | Database user             |
| `DB_PASSWORD` | `taskflow`                | Database password         |
| `DB_NAME`     | `taskflow`                | Database name             |
| `DB_SSLMODE`  | `disable`                 | PostgreSQL SSL mode       |
| `PORT`        | `8066`                    | API server port           |
| `JWT_SECRET`  | `change-me-in-production` | HMAC signing key for JWTs |


## 4. Running Migrations

Migrations are handled by a **separate binary** (`taskflow-migrate`), not the API server. This avoids race conditions when multiple API replicas start simultaneously in production.

### With Docker Compose

Migrations run automatically as a one-off job before the API starts. No manual steps needed.

To run other migration commands against the Dockerized database:

```bash
docker compose run --rm migrate status
docker compose run --rm migrate down
docker compose run --rm migrate version
docker compose run --rm migrate down-to 002
```

### Standalone binary

```bash
go build -o taskflow-migrate ./cmd/migrate

./taskflow-migrate up                 # Apply all pending migrations
./taskflow-migrate up-by-one          # Apply only the next pending migration
./taskflow-migrate up-to 003          # Migrate up to (and including) version 003
./taskflow-migrate down               # Roll back one migration
./taskflow-migrate down-to 001        # Roll back down to (but not including) version 001
./taskflow-migrate redo               # Roll back the last migration and re-apply it
./taskflow-migrate status             # Print migration status table
./taskflow-migrate version            # Print the current migration version
./taskflow-migrate reset              # Roll back all migrations
```

### Creating a new migration (dev only)

```bash
go run ./cmd/migrate-create add_user_avatar
# → migrations/20260413012345_add_user_avatar.sql
```

Edit the generated file, then rebuild so the new migration gets embedded into the binary.

## 5. Test Credentials

Seed migration (`004_seed_data.sql`) creates a ready-to-use account so you can log in immediately without registering:

```
Email:    test@example.com
Password: password123
```

This user owns a project called **Demo Project** with 3 tasks (one `todo`, one `in_progress`, one `done`).

> **Note on pre-computed values in seed data**
>
> The seed migration file contains a pre-computed bcrypt hash and pre-generated UUIDs rather than computing them at migration time (PostgreSQL's `gen_random_uuid()` or `crypt()` would require extensions and produce non-deterministic IDs, making it impossible to reference them in README curl examples).
>
> - **Password hash**: The bcrypt hash (cost 12) for `password123` was generated using the included dev tool:
>   ```bash
>   go run ./cmd/hashpw password123
>   # Output: $2a$12$iUovd.hNj4cHJc0Yqji2wubOvuCRRICi2uw/X65sqFxZImQ0Qvvw.
>   ```
>   This tool uses Go's `golang.org/x/crypto/bcrypt` package — the same library the API uses at runtime — so the hash is guaranteed to be compatible.
> - **UUIDs**: All UUIDs in the seed file were generated using the `uuidgen` command-line tool (available on macOS and Linux). They are valid v4 UUIDs. Using fixed UUIDs means the curl examples in this README reference real, stable IDs that work out of the box after running migrations.

## 6. API Reference

Base URL: `http://localhost:8066`

All responses use `Content-Type: application/json`. Authenticated endpoints require the `Authorization: Bearer <token>` header.

### Health Check

```bash
curl http://localhost:8066/health
```

```json
{
  "status": "ok"
}
```

---

### Auth

#### Register

```bash
curl -X POST http://localhost:8066/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Jane Doe",
    "email": "jane@example.com",
    "password": "securepass123"
  }'
```

**201 Created**

```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "name": "Jane Doe",
  "email": "jane@example.com",
  "created_at": "2026-04-10T12:00:00Z"
}
```

**400 Bad Request** (validation)

```json
{
  "error": "validation failed",
  "fields": {
    "name": "is required",
    "email": "is required",
    "password": "must be at least 6 characters"
  }
}
```

**400 Bad Request** (duplicate email)

```json
{
  "error": "validation failed",
  "fields": {
    "email": "already exists"
  }
}
```

#### Login

```bash
curl -X POST http://localhost:8066/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

**200 OK**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**401 Unauthorized**

```json
{
  "error": "invalid credentials"
}
```

> For all commands below, set the token:
>
> ```bash
> TOKEN=$(curl -s -X POST http://localhost:8066/auth/login \
>   -H "Content-Type: application/json" \
>   -d '{"email":"test@example.com","password":"password123"}' | jq -r '.token')
> ```

---

### Projects

#### List projects

```bash
curl http://localhost:8066/projects \
  -H "Authorization: Bearer $TOKEN"

# With pagination
curl "http://localhost:8066/projects?page=1&limit=10" \
  -H "Authorization: Bearer $TOKEN"
```

**200 OK**

```json
{
  "data": [
    {
      "id": "5ff382e1-a72c-47c4-9f5d-96e2ea307feb",
      "name": "Demo Project",
      "description": "A sample project to get started with TaskFlow",
      "owner_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
      "created_at": "2026-04-10T12:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "limit": 20
}
```

#### Create a project

```bash
curl -X POST http://localhost:8066/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My New Project",
    "description": "Optional description"
  }'
```

**201 Created**

```json
{
  "id": "...",
  "name": "My New Project",
  "description": "Optional description",
  "owner_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
  "created_at": "2026-04-10T12:00:00Z"
}
```

#### Get project details + tasks

```bash
curl http://localhost:8066/projects/5ff382e1-a72c-47c4-9f5d-96e2ea307feb \
  -H "Authorization: Bearer $TOKEN"
```

**200 OK**

```json
{
  "id": "5ff382e1-a72c-47c4-9f5d-96e2ea307feb",
  "name": "Demo Project",
  "description": "A sample project to get started with TaskFlow",
  "owner_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
  "created_at": "2026-04-10T12:00:00Z",
  "tasks": [
    {
      "id": "f6acc292-4153-4904-9a22-87f74e014444",
      "title": "Set up CI/CD pipeline",
      "description": "Configure GitHub Actions for automated testing",
      "status": "todo",
      "priority": "high",
      "project_id": "5ff382e1-a72c-47c4-9f5d-96e2ea307feb",
      "creator_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
      "assignee_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
      "due_date": "2026-04-30",
      "created_at": "2026-04-10T12:00:00Z",
      "updated_at": "2026-04-10T12:00:00Z"
    }
  ]
}
```

#### Update a project (owner only)

```bash
curl -X PATCH http://localhost:8066/projects/5ff382e1-a72c-47c4-9f5d-96e2ea307feb \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Renamed Project",
    "description": "Updated description"
  }'
```

**200 OK**

```json
{
  "id": "5ff382e1-a72c-47c4-9f5d-96e2ea307feb",
  "name": "Renamed Project",
  "description": "Updated description",
  "owner_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
  "created_at": "2026-04-10T12:00:00Z"
}
```

**403 Forbidden**

```json
{
  "error": "only the project owner can update this project"
}
```

#### Delete a project (owner only)

Deletes the project and all its tasks in a single transaction.

```bash
curl -X DELETE http://localhost:8066/projects/5ff382e1-a72c-47c4-9f5d-96e2ea307feb \
  -H "Authorization: Bearer $TOKEN"
```

**204 No Content** — success, empty body.

**403 Forbidden**

```json
{
  "error": "only the project owner can delete this project"
}
```

#### Get project stats

Task counts grouped by status and by assignee.

```bash
curl http://localhost:8066/projects/5ff382e1-a72c-47c4-9f5d-96e2ea307feb/stats \
  -H "Authorization: Bearer $TOKEN"
```

**200 OK**

```json
{
  "by_status": {
    "todo": 1,
    "in_progress": 1,
    "done": 1
  },
  "by_assignee": {
    "77880cdf-01f5-426d-a8fc-2bff70c9b766": 2,
    "unassigned": 1
  }
}
```

---

### Tasks

#### List tasks

Supports filtering by `status` and `assignee`, plus pagination.

```bash
# All tasks in a project
curl http://localhost:8066/projects/5ff382e1-a72c-47c4-9f5d-96e2ea307feb/tasks \
  -H "Authorization: Bearer $TOKEN"

# Filter by status
curl "http://localhost:8066/projects/5ff382e1-a72c-47c4-9f5d-96e2ea307feb/tasks?status=todo" \
  -H "Authorization: Bearer $TOKEN"

# Filter by assignee
curl "http://localhost:8066/projects/5ff382e1-a72c-47c4-9f5d-96e2ea307feb/tasks?assignee=77880cdf-01f5-426d-a8fc-2bff70c9b766" \
  -H "Authorization: Bearer $TOKEN"

# Combined filters with pagination
curl "http://localhost:8066/projects/5ff382e1-a72c-47c4-9f5d-96e2ea307feb/tasks?status=todo&page=1&limit=10" \
  -H "Authorization: Bearer $TOKEN"
```

**200 OK**

```json
{
  "data": [
    {
      "id": "f6acc292-4153-4904-9a22-87f74e014444",
      "title": "Set up CI/CD pipeline",
      "description": "Configure GitHub Actions for automated testing",
      "status": "todo",
      "priority": "high",
      "project_id": "5ff382e1-a72c-47c4-9f5d-96e2ea307feb",
      "creator_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
      "assignee_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
      "due_date": "2026-04-30",
      "created_at": "2026-04-10T12:00:00Z",
      "updated_at": "2026-04-10T12:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "limit": 20
}
```

#### Create a task

```bash
curl -X POST http://localhost:8066/projects/5ff382e1-a72c-47c4-9f5d-96e2ea307feb/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Implement login page",
    "description": "Build the frontend login form",
    "status": "todo",
    "priority": "high",
    "assignee_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
    "due_date": "2026-05-01"
  }'
```

`status` defaults to `todo` and `priority` defaults to `medium` if omitted. `description`, `assignee_id`, and `due_date` are optional.

**201 Created**

```json
{
  "id": "...",
  "title": "Implement login page",
  "description": "Build the frontend login form",
  "status": "todo",
  "priority": "high",
  "project_id": "5ff382e1-a72c-47c4-9f5d-96e2ea307feb",
  "creator_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
  "assignee_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
  "due_date": "2026-05-01",
  "created_at": "2026-04-10T12:00:00Z",
  "updated_at": "2026-04-10T12:00:00Z"
}
```

**400 Bad Request**

```json
{
  "error": "validation failed",
  "fields": {
    "title": "is required",
    "status": "must be one of: todo, in_progress, done",
    "priority": "must be one of: low, medium, high"
  }
}
```

#### Update a task (partial)

Only the fields you send are updated. Everything else stays unchanged.

```bash
curl -X PATCH http://localhost:8066/tasks/f6acc292-4153-4904-9a22-87f74e014444 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "in_progress",
    "priority": "medium"
  }'
```

**200 OK**

```json
{
  "id": "f6acc292-4153-4904-9a22-87f74e014444",
  "title": "Set up CI/CD pipeline",
  "description": "Configure GitHub Actions for automated testing",
  "status": "in_progress",
  "priority": "medium",
  "project_id": "5ff382e1-a72c-47c4-9f5d-96e2ea307feb",
  "creator_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
  "assignee_id": "77880cdf-01f5-426d-a8fc-2bff70c9b766",
  "due_date": "2026-04-30",
  "created_at": "2026-04-10T12:00:00Z",
  "updated_at": "2026-04-10T12:05:00Z"
}
```

Updatable fields: `title`, `description`, `status`, `priority`, `assignee_id`, `due_date`.

To unassign a task, send `"assignee_id": ""`.

#### Delete a task (project owner or task creator only)

```bash
curl -X DELETE http://localhost:8066/tasks/f6acc292-4153-4904-9a22-87f74e014444 \
  -H "Authorization: Bearer $TOKEN"
```

**204 No Content** — success, empty body.

**403 Forbidden**

```json
{
  "error": "only the project owner or task creator can delete this task"
}
```

---

### Error Responses

All error responses follow a consistent shape:

```json
{
  "error": "description of what went wrong"
}
```

Validation errors additionally include a `fields` object with per-field messages:

```json
{
  "error": "validation failed",
  "fields": {
    "email": "is required",
    "password": "must be at least 6 characters"
  }
}
```


| HTTP Status | Meaning                                                                                                    |
| ----------- | ---------------------------------------------------------------------------------------------------------- |
| `400`       | Validation error — request body is missing required fields or contains invalid values                      |
| `401`       | Unauthenticated — the `Authorization` header is missing, the token is malformed, or it has expired         |
| `403`       | Forbidden — the user is authenticated but does not have permission (not the project owner or task creator) |
| `404`       | Not found — the requested project or task does not exist                                                   |
| `500`       | Internal server error — an unexpected failure occurred (database error, etc.)                              |


## 7. What I'd Do With More Time

### Refresh token rotation

The current authentication uses a single JWT with a 24-hour expiry. If this token is intercepted or leaked, an attacker has full access for up to 24 hours with no way to revoke it. A more secure approach would be to issue short-lived access tokens (e.g., 15 minutes) paired with a long-lived refresh token stored server-side. On each refresh, the old refresh token is invalidated and a new one is issued. This limits the damage window of a leaked access token and allows immediate revocation by deleting the refresh token from the database.

### Struct-based validation

Currently, request validation is done manually in each handler — checking if fields are empty, if enum values are valid, etc. This works but produces repetitive code. Switching to a struct-tag-based validation library like `go-playground/validator` would let you define constraints declaratively on the request struct (e.g., `validate:"required,email"`) and validate in a single call. This reduces handler boilerplate significantly and makes it harder to forget a validation rule when adding new fields.

### Rate limiting

Auth endpoints (`/auth/login` and `//auth/register`) are currently unprotected against brute-force attacks. An attacker could attempt thousands of password combinations per second. Adding per-IP rate limiting (e.g., 10 login attempts per minute) and per-account lockout (e.g., lock after 5 failed attempts for 15 minutes) would mitigate this. In a containerized setup, this could be implemented via a middleware using a Redis-backed sliding window counter, or at the infrastructure level with an API gateway.

### Observability (tracing and metrics)

The application currently logs structured JSON via Zap, which is sufficient for basic debugging. For production, I would add OpenTelemetry distributed tracing to track requests across services (useful if TaskFlow ever becomes a multi-service system) and Prometheus metrics for operational visibility — request latency histograms, error rate counters, database connection pool usage, and migration status. These metrics would feed into Grafana dashboards and PagerDuty alerts.

### Soft deletes

When a project or task is deleted, it is permanently removed from the database. In a production system, this is risky — accidental deletions cannot be undone, and there is no audit trail. Implementing soft deletes (adding a `deleted_at` timestamp column and filtering it out in queries) would allow data recovery, support compliance requirements, and enable an "undo" feature in a future frontend. The trade-off is slightly more complex queries and the need for periodic hard-delete cleanup jobs.

### CI/CD pipeline

The project currently has no automated build or deployment pipeline. I would set up GitHub Actions with the following stages: `golangci-lint` for static analysis, `go test` against a PostgreSQL service container for integration tests, multi-stage Docker build, and image push to a container registry. On merge to `main`, the pipeline would automatically build and tag a new image, making it ready for deployment.

### Input sanitization

Task titles and descriptions accept arbitrary strings. If a frontend ever renders these as HTML, this becomes an XSS vector. Sanitizing inputs on write (stripping HTML tags) or encoding on read would prevent this. Even for an API-only service, it is good practice to reject or sanitize payloads containing script tags or HTML entities, since you cannot control how downstream consumers will render the data.

### API versioning

All endpoints currently live at the root path (`/projects`, `/tasks`). If the API ever needs breaking changes (renaming fields, changing response shapes), there is no way to do so without breaking existing clients. Adding a `/v1/` prefix now would make it possible to introduce `/v2/` in the future with a different contract while keeping `/v1/` stable for backward compatibility.
