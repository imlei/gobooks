## GoBooks (Go + Fiber + GORM + Templ)

**Version:** 0.0.1

GoBooks is a simple accounting web app focused on core bookkeeping workflows:

- Company Setup
- Chart of Accounts
- Journal Entry
- Invoices and Bills
- Banking (Reconcile / Receive Payment / Pay Bills)
- Reports (Trial Balance, Income Statement, Balance Sheet)
- Audit Log and Reverse Entry

The codebase follows the product guide and keeps implementation straightforward.

## Tech Stack

- Go
- Fiber
- GORM
- PostgreSQL
- Templ
- Tailwind CSS
- HTMX
- Alpine.js
- shopspring/decimal (money precision)

## Quick Start (Recommended): Docker

### 1) Prerequisites

- Docker Desktop installed and running

### 2) Run

From `d:\Coding\gobooks`:

```bash
docker compose up --build
```

### 3) Open App

- <http://localhost:6768>

On first run, you will see the Setup page and can create your company profile.

## Local Development (Without Docker)

### 1) Prerequisites

- Go 1.22+
- Node.js 18+
- PostgreSQL 14+

### 2) Configure environment

```bash
copy .env.example .env
```

Update `.env` with your local PostgreSQL values.

### 3) Install frontend dependencies

```bash
npm install
```

### 4) Build Tailwind CSS

```bash
npm run build:css
```

For development watch mode (optional, another terminal):

```bash
npm run dev:css
```

### 5) Run backend

```bash
go run ./cmd/gobooks
```

### 6) Open app

- <http://localhost:6768>

## Useful Commands

- Run app with Docker:
  - `docker compose up --build`
- Stop Docker stack:
  - `docker compose down`
- Run backend locally:
  - `go run ./cmd/gobooks`
- Build CSS once:
  - `npm run build:css`
- Watch CSS:
  - `npm run dev:css`

## Project Structure (High Level)

- `cmd/gobooks/` - app entrypoint
- `internal/version/` - release version string (`0.0.1`)
- `internal/config/` - environment config loading
- `internal/db/` - database connection + migration
- `internal/models/` - GORM models
- `internal/services/` - business logic (reports, payments, reverse, audit helpers)
- `internal/web/` - Fiber server, routes, handlers, templates
- `internal/web/templates/` - Templ components and pages

## Troubleshooting

- `go` command not found:
  - Install Go and add it to PATH.
- `docker` command not found:
  - Install Docker Desktop and restart terminal.
- Database connection error:
  - Check `.env` values (`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`).
- Page has no styles:
  - Run `npm run build:css` (or `npm run dev:css` in development).

