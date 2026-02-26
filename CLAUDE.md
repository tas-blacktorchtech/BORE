# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## What This Is

BORE is a local AI engineering system. You give it a task, it runs Commander → Boss → Workers (all Claude CLI processes) in git worktrees, then presents you a diff to review and merge.

The app is a **Go HTTP server + React SPA**, optionally wrapped in **Electron** for a desktop app experience.

## Build & Run

```bash
# Go backend
go build -o bore-server ./cmd/bore-server   # build binary
go build ./...                               # check all packages compile
go vet ./...                                 # static analysis
go test ./...                                # run all tests

# React frontend (dev)
cd frontend && npm install && npm run dev    # Vite dev server on :5173

# React frontend (production build — embedded by Go)
cd frontend && npm run build                 # outputs to frontend/dist/

# Electron (wraps bore-server + React)
npm install                                  # root package.json
npm start                                    # spawns bore-server, opens Electron window
```

Entry point: `cmd/bore-server/main.go`

## Tech Stack

**Go backend**
- Go 1.22+ with `modernc.org/sqlite` (pure Go, no CGO)
- `net/http` stdlib for REST API + SSE
- Git worktrees via `os/exec`
- Claude CLI via `os/exec` with `-p --dangerously-skip-permissions --system`

**React frontend** (`frontend/`)
- React 19 + TypeScript + Vite
- Tailwind CSS v4
- shadcn/ui components (zinc dark theme)
- TanStack React Query for data fetching
- React Router v6

**Electron** (`electron/`)
- Spawns `bore-server` binary, reads `BORE_SERVER_URL=...` from stdout
- Opens BrowserWindow at the server URL
- IPC: `pick-folder`, `toggle-network`, `get-api-url`

## Architecture

### Agent Hierarchy
Commander → Boss → Workers. All are Claude CLI processes with structured JSON output.

- **Commander**: intake/review (fully autonomous in task runner), produces execution brief
- **Boss**: plans, spawns workers, summarises results. Never edits code.
- **Workers**: edit files and run commands in git worktrees.

### Task Lifecycle
```
pending → review → running → diff_review → completed/failed
```
All status transitions emit SSE events (`tasks_updated`, `executions_updated`) to keep the frontend live.

### Background Task Runner (`internal/app/taskrunner.go`)
Starts automatically when a cluster is opened (`OpenCluster`). Polls every 4s for `pending` tasks and runs the full pipeline:
1. Commander: clarifications → options → execution brief (fully autonomous, no user interaction)
2. Create git worktree + execution record in DB
3. Boss: plan → spawn workers in parallel (up to worker budget)
4. Workers run Claude in the worktree directory
5. Boss: summary + lessons persisted to DB
6. Set execution to `diff_review`

All steps log with `[task:N]` prefix — watch progress in the terminal where `bore-server` runs.

### Module Layout
```
cmd/bore-server/    — HTTP server entry (--bind flag, prints BORE_SERVER_URL= on start)
internal/
  app/              — App struct, cluster init/open, crash recovery, task runner
  web/              — HTTP server, REST handlers, SSE hub
  agents/           — Prompt builders (Commander/Boss/Worker) + response type parsing
  process/          — Runner (Claude CLI exec), Scheduler (worker slot semaphore)
  db/               — SQLite: Open(), migrations, typed queries
  git/              — Repo struct: branches, worktrees, diff, commit, clone, slug
  config/           — Config + State JSON load/save/validate
  logging/          — Logger with atomic level, size-based rotation
electron/           — Electron main.ts + preload.ts
frontend/           — React SPA (Vite, shadcn/ui, TanStack Query)
  src/
    pages/          — Home, Dashboard, TaskPage, ExecutionPage, CommanderPage, Settings
    components/     — layout/, ui/ (shadcn), task/, execution/, commander/
    hooks/          — useSSE (SSE event invalidation)
    lib/            — api.ts (all REST calls), types.ts
```

### Cluster Structure (inside target repo)
```
.bore/
  bore.db           — SQLite database
  config.json       — cluster config (model, worker limits, etc.)
  state.json        — lightweight UI state
  logs/             — log files
  runs/             — agent run outputs
  worktrees/        — git worktrees (one per task execution)
```

## REST API

All endpoints served by the Go HTTP server (default `:8742`).

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/status` | Cluster open status |
| GET | `/api/clusters` | Known clusters |
| POST | `/api/clusters/open` | Open existing cluster `{path}` |
| POST | `/api/clusters/init` | Init new cluster from repo `{path, name}` |
| GET | `/api/tasks` | List tasks |
| POST | `/api/tasks` | Create task |
| GET | `/api/tasks/{id}` | Get task |
| GET | `/api/executions` | List executions |
| GET | `/api/executions/{id}` | Get execution |
| GET | `/api/executions/{id}/events` | Execution event log |
| GET | `/api/executions/{id}/runs` | Agent runs |
| GET | `/api/diff/{id}` | Diff for execution |
| POST | `/api/diff/{id}/commit` | Commit diff |
| POST | `/api/diff/{id}/revert` | Revert diff |
| POST | `/api/diff/{id}/merge` | Merge diff |
| GET/PUT | `/api/brain` | Commander brain (key-value memory) |
| GET/POST | `/api/crews` | Crews |
| PUT/DELETE | `/api/crews/{id}` | Update/delete crew |
| GET/POST | `/api/threads` | Threads |
| POST | `/api/commander/chat` | Commander chat turn |
| GET | `/api/branches` | Git branches |
| GET | `/api/server/info` | Port, bind, local IPs, network mode |
| GET | `/events` | SSE stream |

## SSE Events

Frontend subscribes to `/events` and invalidates React Query caches on:
- `tasks_updated` → refetch tasks
- `executions_updated` → refetch executions
- `crews_updated`, `threads_updated`, `brain_updated`
- `cluster_opened` → refetch everything

## Code Conventions

- `context.Context` as first parameter on anything doing I/O
- No mutable package-level variables
- Error messages prefixed with package name: `fmt.Errorf("process: start: %w", err)`
- `errors.As` for type assertions on errors
- Private struct fields with getter methods (App, DB, Runner)
- `errors.Join` for multi-error aggregation
- `0o755` octal format for permissions
- `[]any` not `[]interface{}`

## Frontend Conventions

- shadcn/ui components use explicit zinc colors (`bg-zinc-900 border-zinc-700 text-zinc-100`) — CSS custom properties (`--card`, `--popover`) do not resolve with Tailwind v4 dark theme
- API base URL comes from `window.electronAPI?.getApiUrl()` or falls back to `http://localhost:8742`
- React Query keys match SSE event names (`['tasks']` invalidated by `tasks_updated`)
- Use native Electron frame (no `titleBarStyle: 'hiddenInset'`) for reliable window dragging

## Key Reference Files
- `bore_build_bundle/bore_claude-code_prompt.md` — original master spec
- `bore_build_bundle/bore_schema.sql` — SQLite schema
- `bore_build_bundle/bore_agent_prompts.md` — prompt templates + JSON formats
- `bore_build_bundle/bore_config_reference.md` — config fields
