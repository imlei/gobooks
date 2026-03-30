# GoBooks

A structured, multi-company accounting system built with Go.

Designed around a single principle: **correctness before convenience**. The posting engine enforces double-entry bookkeeping, the tax engine handles recoverability at the line level, and the reconciliation engine produces auditable suggestions — the user always has final authority.

---

## Table of Contents

- [Features](#features)
- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Quick Start — Docker](#quick-start--docker)
- [Local Development](#local-development)
- [Production Deployment](#production-deployment)
- [Migration Strategy](#migration-strategy)
- [Useful Commands](#useful-commands)
- [Troubleshooting](#troubleshooting)
- [License](#license)

---

## Features

### Multi-Company Accounting

Every record carries a `company_id`. Business data never crosses company boundaries at any layer — models, services, queries, and handlers all enforce isolation explicitly.

Users belong to one or more companies via a membership model with role-based access control: **owner, admin, bookkeeper, ap, viewer**. Each role maps to a distinct permission set enforced server-side on every write route.

---

### Chart of Accounts

- Strict code prefix rules: `1xxxx` → Asset, `2xxxx` → Liability, `3xxxx` → Equity, `4xxxx` → Revenue, `5xxxx` → Cost of Sales, `6xxxx` → Expense — violations are rejected at the backend
- Root type + detail type classification
- GIFI code support with format validation
- No deletion — accounts are marked inactive; history is always preserved
- Default COA template auto-applied on company creation
- Rule-based and optional AI-assisted account name/code suggestions (AI is assistive only; suggestions never bypass validation)

---

### Posting Engine

All accounting entries flow through a single `PostingEngine` that coordinates the full lifecycle:

```
Document → Validation → Tax → Posting Fragments → Aggregation → Journal Entry → Ledger Entries
```

Journal lines are **aggregated by account before persistence** — same-account, same-side fragments are merged into canonical debit/credit lines. Fragmented journal output is never written to the database.

**Concurrency-safe posting** uses a three-layer defence:
1. Pre-flight status check before acquiring any lock
2. `SELECT FOR UPDATE` row lock + status re-validation inside the transaction
3. Unique partial index `uq_journal_entries_posted_source` as database backstop

Sentinels `ErrAlreadyPosted` and `ErrConcurrentPostingConflict` are returned to callers for clean error handling.

---

### Journal Entry Lifecycle

```
draft → posted → voided | reversed
```

Every journal entry carries `status`, `source_type`, and `source_id` — it is never a free-floating record. Reversals create a new offsetting entry and mark the original `reversed`; ledger entries are updated atomically. Voiding resets the source document status.

A **ledger entry projection layer** (`ledger_entries` table) is maintained alongside journal lines. It enables running-balance queries without full journal scans.

---

### Sales Tax Engine

- Tax codes are company-scoped with configurable rates
- Tax is calculated **per line**, then **aggregated per account** before posting
- **Sales tax**: revenue line + tax payable credit — always on the same journal entry
- **Purchase tax (recoverable)**: expense debit + recoverable ITC debit
- **Purchase tax (non-recoverable)**: tax amount folded into the expense debit
- Tax accounts are configurable per tax code

---

### Invoices (AR) and Bills (AP)

**Invoices:**
- Draft → Posted → Voided lifecycle
- Line items with per-line product/service, quantity, price, and tax code
- Document number sequencing (configurable prefix and format)
- Post generates a receivables journal entry via the posting engine

**Bills:**
- Same lifecycle as invoices (draft → posted)
- Linked to vendor; optional due date and payment terms
- Pay Bills flow records the bank-side and AP-side journal entry in one transaction
- Linking paid bills to their originating bill is tracked

**Receive Payment:**
- Records customer payment against open invoices
- Creates the bank-side debit and AR-side credit in one transaction
- Optional invoice linkage

---

### Bank Reconciliation

QuickBooks-style reconciliation UI with a four-metric summary bar (Statement Ending Balance, Beginning Balance, Cleared Balance, Difference). Finish Now is only enabled when Difference = $0.00, enforced both client-side (Alpine.js) and server-side inside a database transaction.

**Void:** Only the most recent non-voided reconciliation may be voided. Void requires a written reason, unreconciles all linked journal lines atomically, and is permanently recorded in history — not deleted.

---

### AI-Assisted Auto Match Engine

A three-layer matching engine that **suggests** reconciliation matches — the user always confirms.

**Layer 1 — Deterministic:** Exact amount match against the outstanding balance. Subset-sum search for pairs and triples of candidates that sum exactly to the target.

**Layer 2 — Heuristic scoring:** Four named signals with explicit weights:

| Signal | Weight | Logic |
|--------|--------|-------|
| `exact_amount_match` | 0.35 | Candidate amount equals outstanding balance exactly |
| `date_proximity` | 0.25 | Days between entry date and statement date |
| `source_reliability` | 0.15 | Source type (payment > invoice/bill > manual > opening balance) |
| `historical_match` | 0.25 | Confidence boost from reconciliation memory |

**Layer 3 — Structured explanation:** Every suggestion stores a `MatchExplanation` JSON with a human-readable `Summary`, named `Signals` (score + detail sentence each), `NetAmount`, and confidence `Tier`. The UI renders this in an expandable signal detail panel — there is no opaque ML output.

**Confidence tiers:** High (≥ 0.75) · Medium (≥ 0.45) · Low (< 0.45)

**Suggestion types:** `one_to_one` · `one_to_many` · `many_to_one` · `split`

**Accept / Reject flow:**
- Accept: sets status to `accepted`, updates reconciliation memory, pre-selects the suggested lines in the UI. No journal line is modified.
- Reject: sets status to `rejected`. No accounting data is touched.
- Accepted suggestions remain visible in the panel with a static badge so the user can see what is driving pre-selected checkboxes.
- Finish Now creates the Reconciliation record and sets `reconciliation_id` on the selected journal lines — that is the only point where accounting state changes.

**Reconciliation memory:** Learns from accepted suggestions. Per `(company, account, normalized_memo, source_type)` pattern: `confidence_boost` grows 0.05 per acceptance, hard-capped at 0.30. Bounded, auditable, never silently grows.

**Suggestion lifecycle:** `pending → accepted | rejected | expired | archived`. Suggestions are never deleted; they are retained for full audit history. Running Auto Match again transitions prior pending suggestions to `expired`. Voiding a reconciliation transitions its linked accepted suggestions to `archived`.

---

### Reports

- **Trial Balance** — debit/credit totals by account as of a date
- **Income Statement** — revenue vs. expense for a period
- **Balance Sheet** — assets, liabilities, and equity as of a date
- **Journal Entry Report** — filterable entry list with line detail

All reports are company-scoped and derived from posted journal lines only.

---

### Audit Log

Every critical action (post, void, reverse, reconcile, accept suggestion, etc.) writes an immutable audit log row with: action name, entity type, entity ID, actor (user email), metadata JSON, company ID, user ID, and timestamp. The audit log is append-only and viewable in the settings panel.

---

### SysAdmin Console

A separate login at `/sysadmin` with its own session — no company business data is accessible from the SysAdmin layer.

Capabilities:
- Company and user management
- Maintenance mode toggle (blocks all non-admin logins)
- Audit log viewer (system-wide)
- Runtime metrics: CPU usage, memory, database size, storage
- System log viewer

---

### Settings

- **Company profile:** name, address, currency, fiscal year
- **Document numbering:** prefix and sequence configuration per document type
- **Sales tax codes:** create and manage tax codes and rates
- **AI Connect:** optional external AI provider (OpenAI) for account suggestions — API key stored encrypted at rest
- **Notifications:** SMTP and SMS provider configuration with test endpoints
- **Security:** session timeout and related settings
- **Members:** invite users to the company; manage roles

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.23 |
| Web framework | Fiber v2 |
| ORM | GORM |
| Database | PostgreSQL (production) · SQLite (testing) |
| Templates | Templ |
| CSS | Tailwind CSS |
| Client interactivity | Alpine.js |
| Money arithmetic | shopspring/decimal |
| UUIDs | google/uuid |
| Crypto | golang.org/x/crypto |

---

## Architecture

```
cmd/
  gobooks/          — web server entrypoint
  gobooks-migrate/  — migration runner (run before app start)
  gobooks-reset/    — dev database reset utility
  verifydb/         — database verification tool

internal/
  config/           — environment variable loading
  db/               — database connection + GORM AutoMigrate + SQL file migrations
  models/           — GORM structs: accounts, journal, ledger, invoices, bills,
                      banking, reconciliation suggestions, tax, users, audit
  services/         — all business logic: posting engine, tax engine, reports,
                      reconciliation match engine, memo normalisation, document
                      numbering, audit helpers, AI integration, notifications
  web/
    handlers/       — per-feature HTTP handlers (banking, invoices, bills, accounts, …)
    routes.go       — all routes with middleware composition
    templates/
      layout/       — base HTML layout
      ui/           — reusable UI components (sidebar, forms, badges)
      pages/        — per-page Templ components + view models

migrations/         — 022 numbered SQL files; applied in order via schema_migrations table
```

### Dual-layer architecture

**Business app** — one active company per session; all data is company-scoped. Routes are protected by `RequireAuth` + `RequireMembership` + `RequirePermission(Action)` middleware chains.

**SysAdmin** — separate session, separate login, system-level only. Cannot write business data.

---

## Project Structure

```
gobooks/
├── cmd/
│   ├── gobooks/            Web server entry point
│   ├── gobooks-migrate/    Migration runner
│   ├── gobooks-reset/      Dev DB reset
│   └── verifydb/           DB verification
├── internal/
│   ├── config/             Env config
│   ├── db/                 DB connection + migrate
│   ├── models/             GORM models (31 files)
│   ├── services/           Business logic (54+ files)
│   └── web/
│       ├── templates/      Templ components + page VMs
│       ├── routes.go       Route definitions
│       └── *_handlers.go   HTTP handlers per feature area
├── migrations/             SQL migration files (001–021)
├── static/                 Compiled Tailwind CSS
├── docker-compose.yml
├── Dockerfile
└── .env.example
```

---

## Quick Start — Docker

**Prerequisites:** Docker Desktop installed and running.

```bash
docker compose up --build
```

Docker automatically:
1. Waits for PostgreSQL to pass its health check
2. Runs `gobooks-migrate` (GORM AutoMigrate + all SQL migrations) to completion
3. Starts the application

Open: [http://localhost:6768](http://localhost:6768)

On first run the Setup page appears to create the initial company and admin account.

---

## Local Development

**Prerequisites:** Go 1.23+, Node.js 18+, PostgreSQL 14+

**1. Configure environment**

```bash
cp .env.example .env
```

Edit `.env` with local PostgreSQL credentials. See `.env.example` for all supported variables.

**2. Install frontend dependencies**

```bash
npm install
```

**3. Build Tailwind CSS**

```bash
npm run build:css
```

Watch mode (separate terminal):

```bash
npm run dev:css
```

**4. Run migrations**

```bash
go run ./cmd/gobooks-migrate
```

Applies both GORM AutoMigrate and all SQL files in `migrations/`. Idempotent — safe to run repeatedly.

**5. Run the application**

```bash
go run ./cmd/gobooks
```

Open: [http://localhost:6768](http://localhost:6768)

---

## Production Deployment

Always run the migration binary before the application binary:

```bash
# Step 1 — apply all migrations (exits 0 on success)
./gobooks-migrate

# Step 2 — start the application
./gobooks
```

With Kubernetes or a process manager, use `gobooks-migrate` as an init container or pre-start hook.

---

## Migration Strategy

Gobooks uses a two-phase explicit migration model:

| Phase | Tool | What it does |
|-------|------|-------------|
| 1 — GORM AutoMigrate | `db.Migrate()` | Creates/alters tables based on model structs (never drops columns) |
| 2 — SQL file migrations | `db.ApplySQLMigrations()` | Applies `migrations/*.sql` files tracked in `schema_migrations` |

`cmd/gobooks-migrate` runs both phases in order and is the canonical migration entry point. The app server runs Phase 1 only on startup as a local-dev safety net; SQL migrations must be applied separately in production.

---

## Useful Commands

| Task | Command |
|------|---------|
| Start full stack (Docker) | `docker compose up --build` |
| Stop stack | `docker compose down` |
| Run migrations only (Docker) | `docker compose run --rm migrate` |
| Run migrations (local) | `go run ./cmd/gobooks-migrate` |
| Run app (local) | `go run ./cmd/gobooks` |
| Build CSS once | `npm run build:css` |
| Watch CSS | `npm run dev:css` |
| Reset dev DB | `go run ./cmd/gobooks-reset` |

---

## Troubleshooting

**`go` command not found** — Install Go 1.23+ and add it to `PATH`.

**`docker` command not found** — Install Docker Desktop and restart the terminal.

**Database connection error** — Check `.env`: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`.

**Page has no styles** — Run `npm run build:css`.

**Migration error on startup** — Run `go run ./cmd/gobooks-migrate` manually and inspect the output. The app server only runs Phase 1; SQL migrations may be pending.

---

## License

**SPDX-License-Identifier: AGPL-3.0-only**

GoBooks is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**. See [`LICENSE.md`](LICENSE.md) for the full text.

- You are free to use, modify, and distribute this software.
- If you run a modified version of GoBooks as a network service, you must make the source code of your modifications available to users.

### Commercial License

For use in a commercial SaaS product without complying with AGPL requirements, contact **TAXDEEP CORP.** at [info@taxdeep.com](mailto:info@taxdeep.com).

### Trademark

**GoBooks** is a trademark of **TAXDEEP CORP.** and may not be used in derivative products without permission.
