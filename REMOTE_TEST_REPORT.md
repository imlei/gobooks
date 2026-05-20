# Balanciz PR Remote Test Report

Initial test date: 2026-05-19
Follow-up remote verification: 2026-05-20 UTC
Remote host: 64.69.37.115
Branch tested: `codex/simpletask-module-integration`
Initial commit tested: `e6d0b24 fix task monthly report route`
Latest commit verified: `62ec64e fix pdf attachment availability checks`

## Scope

This test verified the SimpleTask-derived module integration on a real Ubuntu test machine without replacing the existing running Balanciz/GoBooks deployment. The tested areas were:

- isolated clone and build on the remote machine
- frontend dependency/build pipeline
- templ generation
- Go build and focused Go tests
- PostgreSQL migrations on an isolated database
- app startup on an isolated port
- module feature toggles for Task, Employee, Payroll, and Cheque
- authenticated route access for owner and restricted viewer roles
- global/advanced search authorization for sensitive employee/payroll records

## Remote Environment

- OS: Ubuntu 24.04.4 LTS
- Kernel: Linux 6.8.0-31-generic x86_64
- CPU: 2 vCPU
- Memory: about 2 GB RAM, 1 GB swap
- Go: 1.26.0
- Node: 20.20.2
- npm: 10.8.2
- PostgreSQL: 16
- Existing service: `gobooks.service`, listening on `:6768`
- Test clone: `/opt/balanciz-pr7`
- Test database: `balanciz_pr7`
- Test app port: `:6770`

The existing `/opt/gobooks` directory is not a clean Balanciz checkout. It points to `https://github.com/imlei/gobooks.git`, is behind origin, and contains many local changes/untracked files. I therefore used an isolated clone and isolated database to avoid damaging the current test service.

## Build And Test Results

Passed:

- `git clone --branch codex/simpletask-module-integration https://github.com/taxdeep/Balanciz.git /opt/balanciz-pr7`
- `npm ci`
- `npm run build:css`
- `npm run typecheck:react`
- `npm run build:react`
- `go run github.com/a-h/templ/cmd/templ@v0.3.1001 generate`
- `go build -p=1 -o /tmp/balanciz-pr7-test/balanciz ./cmd/balanciz`
- `go build -p=1 -o /tmp/balanciz-pr7-test/balanciz-migrate ./cmd/balanciz-migrate`
- targeted Task/Search tests:
  - `go test -p=1 -timeout=180s ./internal/services ./internal/web -run 'Task|Tasks|task|SmartPicker|GlobalSearch|Search'`
- follow-up key package tests on `62ec64e`:
  - `go test -p=1 -timeout=240s ./internal/db ./internal/services ./internal/services/pdf ./internal/web -count=1`
- follow-up binary rebuild on `62ec64e`:
  - `go build -p=1 -o /tmp/balanciz-pr7-test/balanciz ./cmd/balanciz`
  - `go build -p=1 -o /tmp/balanciz-pr7-test/balanciz-migrate ./cmd/balanciz-migrate`

Notes:

- Vite reported a large chunk warning for `pdf_template_editor.js`; this is a warning only, not a build failure.
- Initial full key package test command `go test -p=1 -timeout=240s ./internal/db ./internal/services ./internal/web` returned non-zero because two existing invoice PDF email attachment tests failed in `internal/services/invoice_send_attachment_test.go`. The failures were:
  - `TestSendInvoiceByEmail_AttachPDFTrue_GeneratorAvailable`
  - `TestSendInvoiceByEmail_SharedFilenameLogic`
- Follow-up fix added `pdf_templates` migration/seeding to the attachment test fixture and changed `PDFGeneratorAvailable()` to check whether the shared chromedp engine can actually initialise Chrome, not just whether a Chrome executable exists.
- Follow-up validation now passes locally and on the remote test machine:
  - `go test ./internal/services -run 'TestSendInvoiceByEmail_AttachPDF|TestGetInvoiceSendDefaults_PDFAvailableField|TestPDFGeneratorAvailable_IsSharedTruth' -count=1`
  - `go test ./internal/services -count=1`
  - `go test ./internal/web ./internal/db -count=1`
  - `go test -p=1 -timeout=240s ./internal/db ./internal/services ./internal/services/pdf ./internal/web -count=1`

## Migration Result

Passed:

- `/tmp/balanciz-pr7-test/balanciz-migrate`
- follow-up idempotent migration run on `62ec64e`

Observed:

- `schema_migrations`: 103 applied migrations
- Public tables after migration: 175
- Core module tables present:
  - `tasks`
  - `task_invoice_sources`
  - `employees`
  - `payroll_runs`
  - `payroll_entries`
  - `payroll_entry_earnings`
  - `payroll_earning_codes`
  - `payroll_remittances`
  - `cheque_bank_accounts`
  - `cheques`
  - `company_features`
  - `search_documents`

Fresh-database migration produced some expected pre-AutoMigrate “relation does not exist” log noise while guard SQL skipped missing legacy tables, but the migration finished successfully with exit code 0.

## Runtime Smoke Test

Passed:

- App started from the PR binary on `http://127.0.0.1:6770`
- `/healthz`: 200
- `/readyz`: 200
- `/version`: 200
- follow-up `62ec64e` smoke also passed on `:6770`
- Existing service on `:6768` remained running
- Temporary PR app was stopped after testing

Unauthenticated protected routes returned redirects before bootstrap/login:

- `/tasks`: 303
- `/payroll/runs`: 303
- `/api/global-search?q=payroll`: 303

## Feature Toggle And Owner Access

Created an isolated bootstrap owner/company through `/setup/bootstrap`.

Before enabling modules:

- `/tasks`: 404
- `/employees`: 404
- `/payroll/runs`: 404
- `/cheques`: 404
- `/api/global-search?q=task`: 200 with empty candidates

Enabled through `/settings/company/features/enable`:

- `task`: 303 redirect after successful POST
- `employee`: 303 redirect after successful POST
- `payroll`: 303 redirect after successful POST
- `cheque`: 303 redirect after successful POST

After enabling modules, owner access passed:

- `/tasks`: 200
- `/tasks/new`: 200
- `/tasks/monthly-report`: 200
- `/tasks/billable-work/report`: 200
- `/employees`: 200
- `/payroll/runs`: 200
- `/payroll/reports/summary`: 200
- `/cheques`: 200
- `/api/global-search?q=payroll`: 200
- `/advanced-search?q=payroll`: 200

## Permission And Search Privacy Smoke Test

Created a viewer-role test user and inserted sensitive search projection rows:

- employee row containing `Jane Private`
- payroll entry row containing `Jane Private Payroll`, `1234.56`, and `777.77`

Viewer access results:

- `/tasks`: 200
- `/tasks/new`: 403
- `/api/tasks/export`: 403
- `/employees`: 403
- `/payroll/runs`: 403
- `/payroll/reports/summary`: 403
- `/payroll/reports/employee-history`: 403
- `/cheques`: 403
- `/api/global-search?q=Jane`: 200, no sensitive matches
- `/advanced-search?q=Jane&type=payroll_entry`: 200, no sensitive matches

Owner search control check:

- `/api/global-search?q=Jane`: 200 and returned the seeded employee/payroll data
- `/advanced-search?q=Jane&type=payroll_entry`: 200 and returned the seeded payroll entry data

Conclusion: the search authorization path is behaving correctly for this smoke test. Sensitive employee/payroll search documents are visible to owner-level access and hidden from a viewer lacking Employee/Payroll detail permissions.

## Current Delivery Assessment

The PR is usable for an internal test/staging delivery:

- isolated remote build succeeds
- migration succeeds on PostgreSQL
- runtime starts successfully
- Task, Employee, Payroll, and Cheque modules are feature-gated
- Task routes and reports smoke successfully
- Payroll/Employee/Cheque route permissions block a low-permission user
- global and advanced search respect sensitive entity permissions in the tested scenario
- the invoice PDF attachment test gap found during remote testing has been fixed and re-verified on the remote test machine

Remaining hardening before production:

- optionally reduce startup migration/log verbosity on fresh databases
- run a browser-level UI pass with screenshots for layout consistency after the backend smoke is clean

## Cleanup

- Temporary app process on `:6770` was stopped.
- Existing `gobooks.service` on `:6768` remained running.
- Test artifacts were left intentionally for repeatability:
  - `/opt/balanciz-pr7`
  - `/tmp/balanciz-pr7-test`
  - PostgreSQL database/user `balanciz_pr7`
