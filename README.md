# YouTube Automator

A semi-automated YouTube publishing pipeline. Finished videos go into S3 with metadata.
The app schedules each video and publishes it to YouTube at its set time, and a dashboard shows queue status.

## Architecture

```
┌─────────────┐      POST /videos       ┌─────────────┐
│  Next.js UI │ ───────────────────────▶│   Go API    │
│ (dashboard) │◀─────────────────────── │   (gin)     │
└─────────────┘      GET /videos        └──────┬──────┘
                                                │ enqueue (ProcessAt=scheduled_at)
                                                ▼
                                         ┌─────────────┐
                                         │ Redis/asynq │
                                         └──────┬──────┘
                                                │ job fires at scheduled time
                                                ▼
                                         ┌─────────────┐        ┌──────────┐
                                         │  Go Worker  │◀──────▶│    S3    │
                                         └──────┬──────┘        └──────────┘
                                                │ videos.insert
                                                ▼
                                         ┌─────────────┐
                                         │ YouTube API │
                                         └─────────────┘
                       ┌─────────────┐
                       │  Postgres   │◀── both API and worker read/write here
                       └─────────────┘
```

- **The API and the worker run as separate processes.** The API stays stateless and scales horizontally.
  The worker handles the slow work (S3 download and YouTube upload), and you scale it by adding replicas.
- **asynq handles scheduling instead of cron.** Each video's `scheduled_at` becomes a `ProcessAt` job,
  which gives retries with backoff and avoids double-publishing.
- **Postgres is the source of truth.** A video's status lives in `videos.status`, so the dashboard keeps
  history after a job has left Redis.

## Build status

| Phase | Scope | Status |
|-------|-------|--------|
| 1 | Backend foundation: config, JSON logging, error handling, `/health` and `/ready`, schema | ✅ done |
| 2 | Videos API: `POST /videos` and paginated `GET /videos`, with jobs enqueued in asynq | ⏳ next |
| 3 | YouTube OAuth channel connection, with tokens encrypted at rest | ⏳ |
| 4 | Worker: download from S3, upload to YouTube, retries, and a YouTube quota guard | ⏳ |
| 5 | Next.js dashboard | ⏳ |
| 6 | API authentication, rate limiting, and YouTube Analytics polling | ⏳ |

Engineering rules for every change are in [skills.md](skills.md).

## Project layout

```
.
├── backend/                     Go API (and, from Phase 4, the worker)
│   ├── cmd/api/                 API entry point: wires dependencies only
│   ├── internal/
│   │   ├── apperr/              typed domain errors
│   │   ├── config/              env-var loading and validation
│   │   ├── db/migrations/       versioned SQL migrations (up + down)
│   │   ├── httpapi/             router, middleware, error handler, /health and /ready
│   │   └── logging/             JSON logger with secret redaction
│   ├── scripts/check-coverage.sh
│   ├── .golangci.yml            lint rules and limits from skills.md §3
│   └── Dockerfile
├── .github/workflows/           CI pipeline
├── docker-compose.yml           Postgres, Redis, migration tool, API
├── .env.example                 template for your local .env
└── skills.md                    engineering rules for every change
```

## Prerequisites

| Tool | Version | Needed for | Required? |
|------|---------|------------|-----------|
| [Git](https://git-scm.com/downloads) | any recent | cloning, committing | yes |
| [Docker Desktop](https://www.docker.com/products/docker-desktop/) (or Docker Engine + Compose v2 on Linux) | Compose v2 | running Postgres, Redis and the API | yes |
| [Go](https://go.dev/dl/) | **1.25.13** or newer | building and testing the backend | for development |
| [golangci-lint](https://golangci-lint.run/) | **v2.4.0** | lint and complexity checks | for development |
| Bash | any | running `scripts/check-coverage.sh` (Git Bash on Windows) | for development |
| curl | any | checking the endpoints (included in Windows 10+, macOS and most Linux distros) | optional |

> Node.js is not needed yet. It will be added in Phase 5 for the dashboard.

### Windows 10/11

Run these in **PowerShell**:

```powershell
winget install --id Git.Git -e            # includes Git Bash
winget install --id Docker.DockerDesktop -e
winget install --id GoLang.Go -e
```

After installing:

1. **Restart your terminal** (and VS Code) so `git`, `go` and `docker` are on your `PATH`.
2. **Open Docker Desktop** and wait until it shows *Engine running*. If it asks, enable the WSL 2 backend (Docker's
   installer handles this, but it may require a reboot). Docker Desktop must be running whenever you use `docker`.
3. Install the linter with Go. The `@v2.4.0` pin matches CI:
   ```powershell
   go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0
   ```
   This places `golangci-lint.exe` in `%USERPROFILE%\go\bin`. Add that folder to your `PATH` if `golangci-lint`
   is not found.

### macOS

This uses [Homebrew](https://brew.sh/):

```bash
brew install git go
brew install --cask docker        # then open Docker.app once and wait for "Engine running"
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0
export PATH="$PATH:$(go env GOPATH)/bin"   # add this line to ~/.zshrc to make it permanent
```

### Linux (Ubuntu/Debian)

```bash
sudo apt update && sudo apt install -y git curl
# Docker Engine + Compose plugin: https://docs.docker.com/engine/install/ubuntu/
sudo usermod -aG docker "$USER"   # then log out and back in so docker works without sudo
# Go 1.25.13+: https://go.dev/doc/install (the distro package is often too old)
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Verify the installs

```bash
git --version
docker version              # must show both "Client" and "Server" sections
docker compose version      # must be v2.x
go version                  # go1.25.13 or newer
golangci-lint --version     # 2.4.0
```

If `docker version` shows only the client and an error like `open //./pipe/dockerDesktopLinuxEngine`, Docker Desktop
is not running. Start it and try again.

## Local setup (first run)

### 1. Clone the repository

```bash
git clone https://github.com/sriharsha2/Project-YT.git
cd Project-YT
```

### 2. Create your `.env`

```bash
cp .env.example .env          # PowerShell: Copy-Item .env.example .env
```

Open `.env` and replace `change-me` with a password of your choice. The password appears **twice**: once in
`POSTGRES_PASSWORD` and once inside `DATABASE_URL`. Both must match.

`.env` is git-ignored. Never commit it.

### 3. Start the stack

```bash
docker compose up --build -d
```

This builds the API image and starts three containers: `postgres`, `redis` and `api`. The first build downloads
images and Go modules, so it takes a few minutes. Check that all containers are up:

```bash
docker compose ps             # postgres and redis should say "healthy"
```

### 4. Create the database schema

Migrations never run automatically at startup, so schema changes are always applied on purpose:

```bash
docker compose --profile tools run --rm migrate up
```

You should see `1/u init`. Running it again prints `no change`, which is fine.

### 5. Check that it works

```bash
curl http://localhost:8081/health
# {"status":"ok"}

curl http://localhost:8081/ready
# {"status":"ok","checks":{"postgres":"ok","redis":"ok"}}
```

On Windows PowerShell, use `curl.exe` instead of `curl`. Plain `curl` is an alias for `Invoke-WebRequest` there.

If `/ready` returns `"unavailable"` for a check, see [Troubleshooting](#troubleshooting).

## Running the backend

There are two ways to run the backend. Pick one.

### Option A: everything in Docker

This is the simplest option and is what the first-run steps above use.

```bash
docker compose up --build -d      # after code changes, rebuild with --build
docker compose logs -f api        # follow the API's JSON logs
docker compose down               # stop (data is kept)
```

### Option B: API on your machine, dependencies in Docker

This option gives the fastest edit-and-run loop. Start only Postgres and Redis, then run the API with `go run`.
Inside Docker the hosts are `postgres` and `redis`, but from your machine both are `localhost`.

```bash
docker compose up -d postgres redis
docker compose stop api           # if it was running, so port 8080 is free
```

**Bash (macOS, Linux, Git Bash):**

```bash
cd backend
export DATABASE_URL="postgres://ytauto:<your-password>@localhost:5432/ytauto?sslmode=disable"
export REDIS_ADDR="localhost:6379"
export LOG_LEVEL=debug
go run ./cmd/api
```

**PowerShell:**

```powershell
cd backend
$env:DATABASE_URL = "postgres://ytauto:<your-password>@localhost:5432/ytauto?sslmode=disable"
$env:REDIS_ADDR = "localhost:6379"
$env:LOG_LEVEL = "debug"
go run ./cmd/api
```

Press `Ctrl+C` to stop. The API finishes in-flight requests before exiting.

## Development workflow

Run these from `backend/`. They are the same checks CI runs, so run them before you push:

```bash
gofmt -l .                       # lists unformatted files (should print nothing); fix with: gofmt -w .
go vet ./...                     # static checks
golangci-lint run ./...          # lint + size/complexity limits from skills.md §3
go test ./...                    # quick test run
bash scripts/check-coverage.sh   # full tests + 100% coverage gate
```

On Windows, run `check-coverage.sh` from **Git Bash**. To browse coverage line by line after running the script:

```bash
go tool cover -html=coverage.out
```

### Git workflow

`main` is protected, so work on a branch (see [skills.md](skills.md) §10):

```bash
git checkout -b feat/short-description
# ...make changes, run the checks above...
git add -A
git commit -m "feat: short description"
git push -u origin feat/short-description
```

Then open a pull request on GitHub. CI (`.github/workflows/backend.yml`) runs on every push and PR:

- format check
- lint
- vet
- the coverage gate
- `govulncheck`
- a gitleaks secret scan
- a Docker build

### Database tasks

```bash
docker compose --profile tools run --rm migrate up         # apply all pending migrations
docker compose --profile tools run --rm migrate down 1     # roll back the latest migration
docker compose --profile tools run --rm migrate version    # show the current version
docker compose exec postgres psql -U ytauto -d ytauto      # open a SQL shell (\dt lists tables, \q quits)
docker compose down -v                                     # stop AND delete all data (fresh start)
```

New migrations go in `backend/internal/db/migrations/` as a pair named `NNNN_name.up.sql` and
`NNNN_name.down.sql`, numbered after the latest one.

## Troubleshooting

| Symptom | Cause and fix |
|---------|---------------|
| `open //./pipe/dockerDesktopLinuxEngine: The system cannot find the file specified` | Docker Desktop is not running. Start it and wait for *Engine running*. |
| `Bind for 127.0.0.1:5432 failed: port is already allocated` (or 6379, 8081) | Another Postgres, Redis or app is using the port. Stop it, or change the left-hand port in `docker-compose.yml`. |
| API exits with `invalid configuration: DATABASE_URL is required ...` | A required variable is missing. For Docker, check `.env`; for Option B, check the variables exported in your terminal. |
| `/ready` shows `"postgres":"unavailable"` | The password in `DATABASE_URL` doesn't match `POSTGRES_PASSWORD`, or you used host `postgres` outside Docker (use `localhost`). Postgres only reads its password on **first** start. If you changed it later, run `docker compose down -v` to reset. |
| `/ready` shows `"redis":"unavailable"` | Redis isn't running (`docker compose up -d redis`), or `REDIS_ADDR` has the wrong host (`redis:6379` inside Docker, `localhost:6379` outside). |
| `migrate` fails with `relation ... already exists` / `Dirty database version` | A migration failed halfway. Run `migrate force <last-good-version>`, then `migrate up`. Locally, `docker compose down -v` is the simplest reset. |
| `golangci-lint: command not found` | Add Go's bin folder to `PATH`: `%USERPROFILE%\go\bin` on Windows, `$(go env GOPATH)/bin` on macOS/Linux. |
| `go: ... requires go >= 1.26` | You tried to upgrade a dependency past what Go 1.25 supports. Pin the older version, or install a newer Go and bump the `go` line in `go.mod`. |
| `git push` fails with `Password authentication is not supported` | GitHub needs a browser login or a personal access token, not your password. Remove the saved `git:https://github.com` entry in Windows Credential Manager, then push again and sign in through the browser window that opens. |

### Configuration

| Variable | Required | Default | Notes |
|----------|----------|---------|-------|
| `DATABASE_URL` | yes | | `postgres://` URL |
| `REDIS_ADDR` | yes | | `host:port` |
| `HTTP_ADDR` | no | `:8080` | |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn` or `error` |

If any value is missing or invalid, the process refuses to start and lists every problem at once.

## Scaling notes

Two things will limit you first:

- **YouTube's API quota.** The default quota is about 6 uploads per day per project.
- **Your content production capacity.**

This codebase's throughput will not be the bottleneck. The architecture (a stateless API with queue-based workers) is what allows scaling.
Prove the content model on one channel before building for many.
