-- 079_expense_posting_and_inventory.sql
-- IN.2: Expense becomes a first-class posted business document AND
-- stock-item expense lines produce inventory movements (Rule #4).
--
-- Pre-IN.2 state
-- --------------
-- Expense was a save-only memo: no Status lifecycle, no JournalEntry
-- link, no posting path. The only consumer was the task-invoice draft
-- reinvoice flow. Stock-item ProductService references on
-- expense_lines were persisted but never produced an inventory effect
-- — a silent-swallow Rule #4 violation (IN.0 §2A).
--
-- What IN.2 installs
-- ------------------
-- 1. `expenses.status TEXT` — draft | posted | voided lifecycle
-- 2. `expenses.journal_entry_id BIGINT` — link to the posted JE
-- 3. `expenses.warehouse_id BIGINT` — header warehouse routing for
--    inventory effect on stock lines (Q3 decision: header, defaulted
--    but visible)
-- 4. `expenses.posted_at TIMESTAMPTZ` / `voided_at TIMESTAMPTZ` —
--    lifecycle timestamps matching Bill/Receipt/Shipment pattern
-- 5. `expense_lines.qty NUMERIC(10,4)` — authoritative quantity when
--    a stock item is picked; inventory.ReceiveStock consumes this
--    value at post time
-- 6. `expense_lines.unit_price NUMERIC(18,4)` — per-unit cost; same
--    role as BillLine.UnitPrice under legacy bill-forms-inventory
--
-- Existing rows
-- -------------
-- Existing expense rows carry status='draft' (the column default);
-- they remain exactly as they were — unposted memos. Operators may
-- edit and post them after IN.2 ships. Existing expense_lines carry
-- qty=1 and unit_price=0 (column defaults); on any subsequent save
-- the service layer will re-compute these from the form input. No
-- retroactive JE is generated for pre-IN.2 expenses; nothing is
-- "repaired" in the ledger by virtue of this migration.
--
-- Rule #4 dispatch (controlled-mode rejection)
-- --------------------------------------------
-- Under `companies.receipt_required=true`, Expense post rejects any
-- stock-item line loudly (ErrExpenseStockItemRequiresReceipt) so the
-- Expense backdoor cannot become a Receipt-first bypass. That check
-- lives in the service layer; this migration installs only the
-- schema. See IN.0 charter §2A Q2.

ALTER TABLE expenses
    ADD COLUMN IF NOT EXISTS status            TEXT        NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS journal_entry_id  BIGINT,
    ADD COLUMN IF NOT EXISTS warehouse_id      BIGINT,
    ADD COLUMN IF NOT EXISTS posted_at         TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS voided_at         TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_expenses_status           ON expenses(status);
CREATE INDEX IF NOT EXISTS idx_expenses_journal_entry_id ON expenses(journal_entry_id);
CREATE INDEX IF NOT EXISTS idx_expenses_warehouse_id     ON expenses(warehouse_id);

ALTER TABLE expense_lines
    ADD COLUMN IF NOT EXISTS qty         NUMERIC(10,4)  NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS unit_price  NUMERIC(18,4)  NOT NULL DEFAULT 0;
