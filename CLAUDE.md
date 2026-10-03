# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

HomeBox is a home inventory/organization app: a Go backend (API + embedded web server) and a Vue/Nuxt frontend, backed by SQLite (default) or PostgreSQL. This is `akempkens/homebox`, a personal fork of `sysadminsmedia/homebox` — treat it as a working copy for your own changes rather than an upstream contribution target. `.github/AGENTS.md` and `.github/instructions/*.md` are upstream Copilot-review conventions; they're low priority here unless you're specifically touching test conventions they cover.

All development commands are run through `task` (Taskfile.yml) — check there before assuming a raw `go`/`pnpm` invocation is right.

## Common commands

Setup (installs Go tools, backend deps, frontend deps):
```bash
task setup
```

Run the backend dev server (SQLite, auto-runs codegen first):
```bash
task go:run
```
Postgres variant: `task go:run:postgresql` (expects a local Postgres at `localhost:5432`, db/user/pass all `homebox`).

Run the frontend dev server:
```bash
task ui:dev
```

Backend tests:
```bash
task go:test                       # all packages
task go:test -- ./internal/data/repo/... -run TestSomething   # scoped/single test
task go:coverage                   # -race + coverage report
```

Backend lint/tidy:
```bash
task go:lint
task go:tidy
```

Frontend checks:
```bash
task ui:check                      # nuxt typecheck
task ui:fix                        # eslint + prettier, writes fixes
task ui:watch                      # vitest watch mode
```
CI lint is strict: `pnpm run lint:ci` fails the build at more than 1 warning, so run `task ui:fix` before considering frontend work done.
Frontend tests directly via pnpm (vitest), scoped to one file:
```bash
cd frontend && pnpm exec vitest run --config ./test/vitest.config.ts path/to/file.test.ts
```

E2E (Playwright, builds backend+frontend and runs against a live server):
```bash
task test:e2e
```

Codegen — required any time you touch `ent` schemas or backend API handlers/DTOs, since the frontend types and Swagger docs are generated, not hand-written:
```bash
task db:generate          # ent codegen from backend/internal/data/ent/schema
task swag                 # regenerate Swagger/OpenAPI from handler annotations
task typescript-types     # regenerate frontend/lib/api/types from swagger.json
task generate             # all three, in the right order
```

Full pre-PR sweep (what CI expects):
```bash
task pr
```

## Architecture

**Backend** (`backend/`), Go, entrypoint `backend/app/api/main.go`:
- `app/api` — HTTP server wiring: `main.go` (config load, DB connect, service construction), `routes.go` (chi router, embeds the built frontend as static files under `static/public`), `middleware.go`, `app.go`. Handlers live in `app/api/handlers/v1` (versioned REST API) and `handlers/debughandlers`.
- `internal/data/ent` — the [ent](https://entgo.io) ORM: schemas in `ent/schema/*.go` define the data model (users, groups, entities/items, locations via entity hierarchy, tags, attachments, maintenance entries, notifiers, API keys, etc.); everything else under `ent/` is generated code (`task db:generate`). Never hand-edit generated ent files — edit the schema and regenerate.
- `internal/data/repo` — repository layer wrapping ent queries; one `repo_*.go` file per aggregate (items/entities, tags, groups, users, tokens, maintenance, exports, notifiers…). This is the layer that owns query composition and cross-cutting concerns like tenant/group scoping (`repo_authz.go`).
- `internal/data/migrations` — raw SQL migrations, split into `sqlite3/` and `postgres/` — **a schema change requires both**, they are not derived from each other.
- `internal/core/services` — business logic layer, called by handlers, calling repos. `all.go` wires services together; `service_*.go` per domain (entities, groups, users, exports, item attachments…). `reporting/` handles the async eventbus-based reporting/notification pipeline.
- `internal/core/currencies` — static currency reference data.
- `internal/sys/config` — typed app configuration loaded via `ardanlabs/conf`, env-var driven (`HBOX_*` prefix, see Taskfile `env:` block for the dev defaults and `docker-compose.yml`/README for prod usage).
- `internal/sys/{otel,analytics,validate}` — observability (OpenTelemetry), analytics, and request validation helpers.
- `internal/web/{mid,adapters}` — shared HTTP middleware and adapter glue used by the handler layer.
- `pkgs/` — standalone helper packages with no app-specific dependencies (hasher, mailer, labelmaker (PDF label generation), faker, textutils, set, cgofreesqlite).

Request flow is roughly: `routes.go` → `handlers/v1` (parses/validates, calls a service) → `core/services` (business rules) → `data/repo` (ent queries) → `data/ent`. In practice many handlers skip the service layer and call `ctrl.repo.X` directly for simple CRUD/reads; only put logic in `core/services` when there's real business logic to orchestrate (validation, multi-repo coordination, event publishing) — don't assume every handler goes through a service.

Handlers are built with the adapter helpers in `internal/web/adapters` (`Command`/`CommandID`/`Action`/`ActionID`/`Query`/`QueryID`) rather than raw `http.HandlerFunc` bodies — pick the adapter matching whether the route has a path UUID, a request body, and/or query params; only file uploads and a few complex multi-param queries bypass adapters with a manual `func(w http.ResponseWriter, r *http.Request) error`. Every handler needs swaggo doc comments (`@Summary`/`@Tags`/`@Router`/…) since that's what `task swag`/`task typescript-types` generate from. Multi-tenancy is enforced via `services.NewContext(r.Context())`, which yields a `Context` carrying `GID`/`UID`; repo and service calls take this and scope queries by group — always thread it through rather than a bare `context.Context`.

**Frontend** (`frontend/`), Nuxt 4 + Vue 3 + ShadCN (via `shadcn-nuxt`) + Tailwind:
- `pages/` — file-based routes (item, location, collection, tag, template, reports, label, assets, home, the `a/` auth pages).
- `components/` — organized by domain (Item, Location, Collection, Entity, Maintenance, Tag, Template, Search, Scanner) plus `Base`/`Form`/`App`/`global` for generic building blocks and `ui/` for ShadCN primitives (excluded from lint via `--ignore-pattern`, treat as vendored).
- `lib/api` — generated API client/types (`lib/api/types`, from `task typescript-types`) plus hand-written request wrappers in `lib/requests`.
- `stores/` — Pinia stores for client-side state.
- `composables/`, `middleware/`, `plugins/` — standard Nuxt extension points (auth middleware, i18n setup, PWA plugin, etc.).
- `locales/` — i18n JSON translation files (also managed upstream via Weblate).
- `test/` — Vitest unit tests plus `test/e2e` Playwright specs and `test/upgrade` (data-upgrade regression tests).

The frontend never talks to ent/repo directly — it only calls the generated API client, which hits the Go backend's `handlers/v1` routes, typically via the `useUserApi()` composable (authenticated, typed against `lib/api/types`). In dev, backend and frontend run as separate servers (`task go:run` + `task ui:dev`); in production/CI builds the frontend is built and embedded into the Go binary's static files (`routes.go`'s `public` embed, populated by `go:ci:with-frontend`).

Components under `components/` and composables under `composables/` are auto-imported (no explicit `import` needed) — nesting maps to naming, e.g. `components/Item/Card.vue` → `<ItemCard />`. Pages use Nuxt file-based routing (`pages/item/[id].vue` → `/item/:id`).

## Key cross-cutting notes

- Changing anything in `ent/schema` → run `task db:generate`, then usually `task swag` + `task typescript-types` if the API surface also changed, and add matching SQL migrations under **both** `migrations/sqlite3` and `migrations/postgres`.
- Handler doc comments (swaggo annotations) are the source of truth for `swagger.json`, which is in turn the source of truth for the frontend's TypeScript API types — don't hand-edit `frontend/lib/api/types`.
- `UNSAFE_DISABLE_PASSWORD_PROJECTION` is a dev-only convenience (plaintext password hashing) used by the `go:run*`/CI tasks that start a live server, and is deliberately **not** set globally in the Taskfile — it must never leak into `go:test`/`go:coverage`, which need to exercise real argon2id hashing.
