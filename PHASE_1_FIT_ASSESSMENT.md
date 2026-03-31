# Gobooks Invoice Module — Phase 1: Design & Fit Assessment

**Date**: 2026-03-30  
**Status**: 🔴 IN REVIEW (不改代码，仅设计评审)  
**Objective**: 完整的代码审查 + 设计对齐，确定可复用组件、缺口、设计决策

---

## A. Existing Reusable Components

### A.1 Company Isolation Pattern ✅

**Components**:
- `models/company.go`: Company struct
- `models/company_membership.go`: CompanyMembership (user → company + role)
- `models/company_role.go`: CompanyRole enum (owner, admin, accountant, bookkeeper, ap, viewer)
- `web/permissions.go`: Permission matrix + Action constants

**Reusable For Invoice**:
```
✅ invoice.company_id FK + NOT NULL
✅ Query: WHERE company_id = ? (enforced in all handlers + services)
✅ Permission check: @RequireMembership + @RequirePermission(ActionInvoiceXxx)
✅ Audit: company_id always passed to WriteAuditLog
✅ User roles with granular permissions (owner/admin can *approve*, bookkeeper can *create*)
```

**Pattern To Follow**:
```go
// In handlers
companyID, ok := ActiveCompanyIDFromCtx(c)  // enforced by @ResolveActiveCompany middleware
if !ok { return c.Redirect("/select-company", ...) }

// In services
func CreateInvoice(db *gorm.DB, companyID uint, ...) error {
  // Always validate company_id ownership
  var company models.Company
  if err := db.Where("id = ?", companyID).First(&company).Error; err != nil {
    return fmt.Errorf("company not found: %w", err)
  }
  // ... proceed
}

// In DB queries
db.Where("company_id = ?", companyID).Find(&invoices)
```

---

### A.2 Audit Logging System ✅

**Components**:
- `models/audit_log.go`: AuditLog struct
- `services/audit_log.go`: WriteAuditLog* functions (5 variants)

**Signatures**:
```go
WriteAuditLog(tx *gorm.DB, action, entityType string, entityID uint, actor string, details any) error

WriteAuditLogWithContext(tx *gorm.DB, action, entityType string, entityID uint, actor string, details any, 
  companyID *uint, actorUserID *uuid.UUID) error

WriteAuditLogWithContextDetails(tx *gorm.DB, action, entityType string, entityID uint, actor string, details any,
  companyID *uint, actorUserID *uuid.UUID, before, after any) error
```

**Reusable For Invoice**:
```
✅ action = "invoice.created" | "invoice.updated" | "invoice.issued" | "invoice.sent" | "invoice.voided"
✅ entityType = "invoice"
✅ entityID = invoice.id
✅ actor = user.email (from context)
✅ companyID = pointer to company_id
✅ actorUserID = pointer to user.ID (uuid.UUID)
✅ details = map[string]any { "invoice_number", "customer_id", "amount", ... }
✅ before/after = previous state (for change tracking)
```

**Pattern To Follow**:
```go
// After transaction commits
err := services.WriteAuditLogWithContextDetails(
  tx,
  "invoice.issued",
  "invoice",
  invoice.ID,
  user.Email,
  map[string]any{
    "invoice_number": invoice.InvoiceNumber,
    "customer_id":    invoice.CustomerID,
    "amount":         invoice.Amount.StringFixed(2),
  },
  &companyID,
  &user.ID,
  nil,  // before
  map[string]any{...},  // after
)
```

---

### A.3 Posting Engine & Journal Entry ✅

**Components**:
- `services/posting_engine.go`: PostingEngine (coordinator)
- `services/invoice_post.go`: PostInvoice() (existing implementation)
- `services/bill_post.go`: PostBill() (reference implementation)
- `services/invoice_void.go`: VoidInvoice()
- `services/journal_reverse.go`: ReverseJournalEntry()

**Journal Entry Lifecycle**:
```
models/journal.go:

type JournalEntryStatus string
  JournalEntryStatusDraft     // reserved for future approval workflows
  JournalEntryStatusPosted    // active, ledger entries exist
  JournalEntryStatusVoided    // cancelled before posting (legacy)
  JournalEntryStatusReversed  // reversal JE posted, original marked reversed

type JournalEntry struct {
  ID            uint
  CompanyID     uint          // MANDATORY
  EntryDate     time.Time
  JournalNo     string
  Status        JournalEntryStatus
  SourceType    LedgerSourceType  // "invoice" | "bill" | "payment" | "manual"
  SourceID      uint              // maps back to invoice.id / bill.id / payment.id
  ReversedFromID *uint             // for reversal entries
  Lines []JournalLine
}

type JournalLine struct {
  ID             uint
  CompanyID      uint
  JournalEntryID uint
  AccountID      uint
  Debit          decimal.Decimal   // mutually exclusive with Credit
  Credit         decimal.Decimal
  Memo           string
  PartyType      PartyType  // customer | vendor
  PartyID        uint
  ReconciliationID *uint     // for bank reconciliation
}
```

**Concurrency Protection**:
```go
// Existing pattern for double-posting prevention:
// Inside PostInvoice transaction:
db.Clauses(clause.Locking{Strength: "UPDATE"}).
  Where("id = ? AND company_id = ? AND status = ?", invoiceID, companyID, InvoiceStatusDraft).
  First(&inv)  // SELECT FOR UPDATE → blocks concurrent posting
```

**Ledger Projection**:
```
services/ledger.go:
  ProjectToLedger(db, journalEntry) → creates ledger_entries from journal_lines
  
models/ledger_entry.go:
  type LedgerEntry struct {
    ID               uint
    CompanyID        uint                // redundant from JE, optimizes queries
    JournalEntryID   uint
    SourceType       LedgerSourceType
    SourceID         uint
    AccountID        uint
    PostingDate      time.Time           // copied from JE.EntryDate
    DebitAmount      decimal.Decimal
    CreditAmount     decimal.Decimal
    Status           LedgerEntryStatus   // active | reversed
    CreatedAt        time.Time
  }
  
  Status semantics: when a reversal JE is posted, original LE rows marked "reversed",
                    new LE rows created for reversal (swapped debit/credit).
```

**Reusable For Invoice**:
```
✅ Existing PostInvoice() can be called from new Invoice Module
✅ SourceType = "invoice" ← already supported
✅ Uniqueness partial index (company_id, source_type='invoice', source_id, status='posted')
  prevents double-posting of same invoice
✅ SourceID = invoice.id maps directly
✅ JournalEntry.CompanyID always enforced
✅ Ledger projection automatic when JE posted
✅ Void workflow already handled (creates reversal JE + marks original reversed)
✅ Can query ledger by source: ledger_entries WHERE source_type='invoice' AND source_id=?
```

---

### A.4 Tax Engine ✅

**Components**:
- `models/tax.go`: TaxCode struct
  ```go
  type TaxCode struct {
    ID                           uint
    CompanyID                    uint
    Name                         string
    Rate                         decimal.Decimal  // E.g., 0.05 for 5%
    Scope                        TaxScope         // "sales" | "purchase" | "both"
    RecoveryMode                 TaxRecoveryMode  // "none" | "full" | "partial"
    RecoveryRate                 decimal.Decimal  // 0-100%
    SalesTaxAccountID            uint             // WHERE to credit tax liability
    PurchaseRecoverableAccountID *uint            // WHERE to put recoverable tax
    IsActive                     bool
  }
  ```

- `services/tax_engine.go`: CalculateTax() function
  ```go
  func CalculateTax(netAmount decimal.Decimal, code models.TaxCode) []TaxLineResult
  
  type TaxLineResult struct {
    SalesTaxAccountID uint
    TaxAmount         decimal.Decimal
  }
  ```

- `services/tax_service.go`: ComputeTax(), ComputeLineTax()

**Reusable For Invoice**:
```
✅ Per-line tax: InvoiceLine.TaxCodeID → TaxCode → calculate tax amount
✅ Tax always goes to TaxCode.SalesTaxAccountID (accounts payable / tax liability)
✅ Sales-side taxation always includes tax in Accounts Receivable total
✅ Example:
    Line: 100 @ 5% GST
    → Debit AR 105, Credit Revenue 100, Credit GST Payable 5
✅ Fragments generated separately for each tax (one credit line per tax code)
✅ Document-level totals: Subtotal (net) + TaxTotal (sum of line taxes) = Amount
```

---

### A.5 Fragment Builder & Aggregation ✅

**Components**:
- `services/fragment_builder.go`: BuildInvoiceFragments(), BuildBillFragments()
- `services/journal_aggregate.go`: AggregateJournalLines()

**Pattern**:
```go
BuildInvoiceFragments(invoice, arAccountID) → []PostingFragment

type PostingFragment struct {
  AccountID uint
  Debit     decimal.Decimal
  Credit    decimal.Decimal
  Memo      string
}

// Example: Invoice $1000 net, 5% tax ($50)
// Output:
//   DR AR 1050, CR Revenue 1000, CR Tax Payable 50

AggregateJournalLines([]PostingFragment) → []PostingFragment
  // Collapses by (AccountID, Debit/Credit side)
  // Example above would stay same (3 unique accounts)
  // But if 2 revenue accounts: still separate lines
```

**Reusable For Invoice**:
```
✅ Same pattern as existing invoice_post.go
✅ GenerateFragments → Aggregate → CreateJE → ProjectToLedger
✅ Can call BuildInvoiceFragments(...) directly in invoice service
✅ No new logic needed (already correct implementation)
```

---

### A.6 Numbering System ✅

**Components**:
- `models/numbering_settings.go`: NumberingSetting (company-scoped JSONB rules)
- `services/document_number.go`: NextDocumentNumber(), ValidateDocumentNumber()

**Pattern**:
```go
// Validation: allows A-Z, a-z, 0-9, -, #, @
ValidateDocumentNumber(s) error

// Next number generation:
NextDocumentNumber(last string, fallback string) string
  // Examples:
  // "IN001" → "IN002"
  // "INV-0099" → "INV-0100"
  // "ABC" → "ABC-001"

// Usage in invoice creation:
func SuggestNextInvoiceNumber(db *gorm.DB, companyID uint) (string, error) {
  // Fetch last invoice number
  var lastInvoice models.Invoice
  db.Where("company_id = ?", companyID)
    .Order("created_at desc")
    .First(&lastInvoice)
  
  return services.NextDocumentNumber(lastInvoice.InvoiceNumber, "IN001"), nil
}

// Uniqueness: enforced by DB UNIQUE constraint on (company_id, LOWER(invoice_number))
unique_invoices_company_number UNIQUE (company_id, invoice_number)
```

**Reusable For Invoice**:
```
✅ Existing system (already in use for invoices)
✅ Store last invoice_number in InvoiceNumber field
✅ Uniqueness enforced by DB (case-insensitive)
✅ Frontend shows suggested next number
✅ No new code needed
```

---

### A.7 SMTP & Email Infrastructure ✅

**Components**:
- `models/notification_settings.go`: CompanyNotificationSettings, SystemNotificationSettings
- `services/email_sender.go`: SendEmail(), sendViaSSL(), sendViaSTARTTLS()
- `services/email_provider.go`: EmailConfig, SendTestEmail(), EffectiveSMTPForCompany()

**SMTP Readiness Semantics**:
```go
type CompanyNotificationSettings struct {
  ID                    uint
  CompanyID             uint    // UNIQUE
  EmailEnabled          bool
  SMTPHost              string
  SMTPPort              int
  SMTPUsername          string
  SMTPPasswordEncrypted string  // encrypted in DB
  SMTPFromEmail         string
  SMTPFromName          string
  SMTPEncryption        SMTPEncryption  // none | ssl_tls | starttls

  // Readiness tracking
  EmailTestStatus       NotifTestStatus // never | success | failed
  EmailLastTestedAt     *time.Time
  EmailLastSuccessAt    *time.Time
  EmailLastError        string
  EmailConfigHash       string    // SHA-256 of current config
  EmailTestedConfigHash string    // hash at time of last successful test
  EmailVerificationReady bool      // true only if config unchanged since last success test
}

// Readiness rule (ENFORCED by backend):
// EmailVerificationReady = EmailEnabled 
//   && config complete (host + from_email + port > 0)
//   && EmailTestStatus == "success"
//   && EmailConfigHash == EmailTestedConfigHash
```

**EffectiveSMTPForCompany** (existing function):
```go
func EffectiveSMTPForCompany(db *gorm.DB, companyID uint) (cfg EmailConfig, ready bool, err error) {
  // Resolution order:
  // 1. Company override: if enabled && email.verification_ready → use it
  // 2. System default: if email.verification_ready → use it
  // 3. Neither ready → (zero EmailConfig, false)
  
  return cfg, ready, err  // ready=false means SMTP not ready, BLOCK email send
}
```

**SendEmail Signature**:
```go
func SendEmail(cfg EmailConfig, toAddr, subject, body string) error {
  // Sends plain-text email
  // Returns error if SMTP server unreachable
}
```

**Reusable For Invoice**:
```
✅ Before sending invoice email:
   cfg, ready, err := EffectiveSMTPForCompany(db, companyID)
   if !ready { return fmt.Errorf("SMTP not ready") }
   
✅ To send email:
   err := SendEmail(cfg, customer.Email, subject, body)
   
✅ Frontend: check ready status before showing "Send" button
   (but backend MUST also check before accepting request)

✅ Email can include PDF as attachment (need to extend SendEmail or create new SendEmailWithAttachment)
   Current SendEmail only handles plain-text; future phase will add attachment support
```

---

### A.8 Permissions & Access Control ✅

**Components**:
- `web/permissions.go`: Action constants, Permission constants, rolePermissions map

**Permission Matrix**:
```
Owner       → all
Admin       → all
Accountant  → AR, AP, Approve, Reports, Audit
Bookkeeper  → AR, AP, Reports, Audit (NO Approve)
AP          → AP only
Viewer      → Reports only
```

**Action Constants** (relevant to Invoice):
```go
const (
  ActionInvoiceView    = "invoice:view"    // view list/detail (membership sufficient)
  ActionInvoiceCreate  = "invoice:create"  // create/edit draft
  ActionInvoiceUpdate  = "invoice:update"  // edit existing
  ActionInvoiceDelete  = "invoice:delete"  // delete draft (TO IMPLEMENT)
  ActionInvoiceApprove = "invoice:approve" // post/void (requires higher permission)
)
```

**Required Permissions for Action**:
```
ActionInvoiceView    → PermARAccess
ActionInvoiceCreate  → PermARAccess
ActionInvoiceUpdate  → PermARAccess
ActionInvoiceDelete  → PermARAccess (only draft, not posted)
ActionInvoiceApprove → PermApproveTransactions
```

**Reusable For Invoice**:
```
✅ In handlers: @RequirePermission(ActionInvoiceXxx)
✅ In templates: {{ if can "invoice:approve" }} Show Post/Void buttons {{ end }}
✅ No new permission types needed for MVP
✅ Can add granular permissions in future (send, download PDF, view payment status, etc.)
```

---

### A.9 Data Models — Customer, Product, Account ✅

**Customer** (`models/party.go`):
```go
type Customer struct {
  ID          uint
  CompanyID   uint
  Name        string
  Address     string  // optional
  PaymentTerm string  // e.g., "Net 30"
  CreatedAt   time.Time
}
// Validation: CompanyID + Name uniqueness (app-level, NOT DB constraint)
```

**ProductService** (`models/product_service.go`):
```go
type ProductService struct {
  ID               uint
  CompanyID        uint
  Name             string
  Type             ProductServiceType  // "service" | "non_inventory"
  Description      string
  DefaultPrice     decimal.Decimal
  RevenueAccountID uint                // MANDATORY for posting
  DefaultTaxCodeID *uint               // optional, pre-fills invoice lines
  IsActive         bool
}
```

**Account** (`models/account.go`):
```go
type Account struct {
  ID           uint
  CompanyID    uint
  Code         string                  // E.g., "4000" for revenue
  Type         AccountType             // asset | liability | equity | revenue | expense
  Name         string
  IsActive     bool
  ... more fields ...
}
// Uniqueness: (company_id, LOWER(code)) unique
```

**Reusable For Invoice**:
```
✅ Customer: link customer → invoice (address, payment terms can be copied to invoice)
✅ ProductService: link product → invoice line (revenue account, default tax pre-fill)
✅ Account: revenue account from ProductService validates in posting
✅ Key requirement: All three MUST belong to same company_id (validated in services)
```

---

### A.10 Logo Upload / File Storage ✅

**Components**:
- `web/company_logo_handlers.go`: handleCompanyLogoUpload(), handleCompanyLogoServe()
- `models/company.go`: Company.LogoPath field

**Pattern**:
```go
// Upload (POST):
// Saves file to: data/{companyID}/profile/logo.{ext}
// Updates company.logo_path

// Serve (GET):
// Reads from: data/{companyID}/profile/logo.{ext}
// Protected by @RequireMembership (auth + company isolation)
```

**Reusable For Invoice (Templates)**:
```
✅ For storing template logos/images:
   Can follow same pattern: data/{companyID}/templates/{templateID}/logo.{ext}
✅ Or store as base64 in DB (simpler, but DB bloat)
✅ MVP: store file path in invoices_templates.logo_image_id → can point to file
```

---

## B. Gaps (缺失项目)

### B.1 Invoice Templates ❌

**What's Missing**:
1. No `invoices_templates` table
2. No `invoices_templates_line_items` table (template column visibility config)
3. No InvoiceTemplate Go model
4. No template service (CRUD, set-default, config management)
5. No template editor UI
6. No template selection in invoice editor
7. No template rendering engine (HTML → formatted invoice view)

**Impact**: Cannot render customized invoice layouts; each company sees same template.

---

### B.2 PDF Export ❌

**What's Missing**:
1. No PDF generation library integrated (wkhtmltopdf, gofpdf, chromedp, etc.)
2. No HTML → PDF service
3. No PDF download route
4. No PDF preview route
5. No attachment support in email_sender.go (currently plain-text only)

**Impact**: Cannot export invoices to PDF; no downloadable file for customers.

---

### B.3 Invoice Email Logs ❌

**What's Missing**:
1. No `invoice_email_logs` table
2. No email tracking model
3. No send log service
4. No send email route
5. No email precondition checks (SMTP ready, customer has email, etc.)
6. No email retry logic

**Impact**: Cannot track which invoices were sent, to whom, success/failure.

---

### B.4 Invoice Status Model ❌

**What's Missing**: 
Current invoice status is: draft → sent → paid → voided

But Gobooks needs more explicit state machine:
- draft (editable, can delete)
- issued (locked, generates JE, cannot modify, can void)
- sent (issued + mailed to customer)
- partially_paid (received payment < total)
- paid (received payment >= total)
- overdue (due_date < today && unpaid)
- void (reversed, voided)

**Current limitation**: Status is just a text field, no state machine logic.

---

### B.5 Business Rules Engine ❌

**What's Missing**:
- Status transition validation (can only issue draft; can only send issued; etc.)
- Posted invoice immutability (except void)
- Void reasons and tracking
- Payment status updates (auto mark paid when received payment >= amount)
- Due date tracking and overdue determination

---

### B.6 Template Rendering Engine ❌

**What's Missing**:
- Server-side HTML template rendering
- Data binding (invoice → template variables)
- Multi-template support
- Template preview without saving
- Template section visibility toggling (show/hide tax summary, notes, payment info)

---

## C. Required Design Decisions

### C.1 Invoice Status Model

**Decision**: What states should invoice have?

**Options**:

**Option A: Simple (current)**
```
draft → sent → paid / voided
```
- ✅ Simple
- ❌ No "issued" separation (accountant approval → post to books vs. send to customer)
- ❌ No overdue tracking
- ❌ No partial payment tracking

**Option B: Full State Machine (RECOMMENDED)**
```
draft
  ↓ (issue) → issued (JE created, locked)
  ↓ (send) → sent
  ↓ (receive payment) → partially_paid
  ↓ (receive full) → paid
  ↓ (past due) → overdue (side status)
  
  void (any state → audit trail, no JE reverse yet)
```

**Semantics**:
- **draft**: Editable, no JE, can delete
- **issued**: JE created (posted), no edit, cannot delete, can send/void
- **sent**: issued + emailed to customer, save send log
- **partially_paid**: issued + payment < total, track balance due
- **paid**: issued + payment >= total
- **overdue**: issued + due_date < today && payment < total (derived, not stored)
- **void**: Any state + reversal JE, audit trail, immutable

**Recommendation**: Go with **Option B** (full state machine) because:
1. Aligns with accounting standards (issued = posted to books)
2. Supports partial payments future phase
3. Better audit trail

---

### C.2 Invoice Number Generation Strategy

**Decision**: How to generate invoice numbers?

**Options**:

**Option A: Automatic Sequential** (current pattern)
```go
LastInvoice.Number: "IN001"
NextNumber: "IN002"  // automatic
```
- ✅ Simple, no gaps, testable
- ✅ Displayed to user before save
- ❌ Not customizable per company

**Option B: Company-Configured Template**
```
Company1: "INV-YYYY-NNNN" → "INV-2026-0001"
Company2: "QB-NNNN" → "QB-0001"
```
- ✅ Flexible
- ✅ Common in QuickBooks
- ❌ Complex to implement
- ❌ Requires numbering_settings refactor

**Option C: Manual Entry (always)**
```
User types: "INV-ABC-001"
Duplicate check: enforce uniqueness (company_id, invoice_number case-insensitive)
```
- ✅ Full control
- ✅ Backward compatible
- ❌ User error prone (gaps, duplicates)

**Recommendation**: Stick with **Option A** (Automatic Sequential)
- Existing system already does this
- Good UX (suggested number pre-filled)
- No need to change now; Option B can be Phase 2

---

### C.3 Template Storage Strategy

**Decision**: How to store template configurations and rendered HTML?

**Options**:

**Option A: Config Only (RECOMMENDED)**
```sql
invoices_templates:
  config_json: {
    "accent_color": "#3498db",
    "show_logo": true,
    "show_company_address": true,
    "show_tax_summary": true,
    "footer_text": "Thank you...",
    "email_subject_template": "Invoice {invoice_number}",
    ...
  }
```
- HTML rendered at display time from template config + invoice data
- ✅ Data always fresh (if account name changes, updates automatically)
- ✅ No HTML blob mixed with business data
- ✅ Follows guideline: "template ≠ data"
- ❌ Slower rendering (but acceptable, invoice HTML is small)

**Option B: Store HTML Snapshots**
```sql
invoices_templates:
  html_template: "<html>...</html>"
```
- HTML stored once per template
- ✅ Faster rendering
- ❌ VIOLATES guideline: "no HTML blobs for business truth"
- ❌ Cannot update company logo and affect all invoices retroactively
- ❌ Hard to maintain, upgrade, or change styles globally

**Recommendation**: **Option A** (Config Only)
- Cleaner architecture
- Follows PROJECT_GUIDE.md principle

---

### C.4 PDF Generation Strategy

**Decision**: What library + approach for PDF generation?

**Options**:

**Option A: wkhtmltopdf (External Binary)**
```go
// Render HTML → wkhtmltopdf binary → PDF bytes
import "github.com/SebastiaanKlippert/go-wkhtmltopdf"

pdf, err := converter.Convert()  // calls wkhtmltopdf CLI
```
- ✅ High quality output (Webkit rendering)
- ✅ Supports CSS, images, complex layouts
- ❌ Requires system dependency (wkhtmltopdf binary installation)
- ❌ Slower (spawns external process)
- ❌ Not ideal for cloud/serverless (need binary in container)

**Option B: Go Native Library (gofpdf)**
```go
import "github.com/go-pdf/fpdf"

pdf := fpdf.New("P", "mm", "A4", "")
pdf.AddPage()
pdf.SetFont("Arial", "B", 16)
pdf.Cell(0, 10, "Invoice")
```
- ✅ Pure Go, no external dependencies
- ✅ Simple installation
- ✅ Portable (cloud-friendly)
- ❌ Limited HTML/CSS support
- ❌ Requires programmatic PDF building (more code)
- ❌ Less professional output

**Option C: Headless Chrome (chromedp)**
```go
import "github.com/chromedp/chromedp"

// Launch headless Chrome, render HTML, output PDF
```
- ✅ Same quality as wkhtmltopdf
- ✅ Better maintained (Chromium ecosystem)
- ❌ Heavy (Chrome process)
- ❌ Slower startup
- ❌ More resource intensive

**Recommendation**: Start with **Option A** (wkhtmltopdf)
- Best quality for invoice rendering
- Mature, proven library
- Can add Option B as fallback later if needed
- For deployment: add wkhtmltopdf to Dockerfile / system setup docs

---

### C.5 Email Sending Strategy

**Decision**: Synchronous vs. Asynchronous?

**Options**:

**Option A: Synchronous (RECOMMENDED for MVP)**
```go
func (s *Server) handleInvoiceSend(c *fiber.Ctx) error {
  // Block user until email sent or error
  err := services.SendInvoiceEmail(...)
  if err != nil {
    return c.Status(400).JSON(error("Email failed: " + err.Error()))
  }
  return c.JSON(success("Email sent"))
}
```
- ✅ Simple
- ✅ User knows immediately if failed
- ✅ Easier to debug
- ❌ Slow UX if SMTP slow (e.g., 1-2 second delay)
- ❌ Request timeout risk if SMTP hangs

**Option B: Asynchronous (Job Queue)**
```go
// User clicks Send → job queued → function returns immediately
// Background worker picks up job, sends email, logs result
```
- ✅ Fast UX
- ✅ Retry logic easier
- ✅ Can batch send
- ❌ Complex (need job scheduler)
- ❌ User doesn't know immediate result

**Recommendation**: **Option A** (Synchronous) for MVP
- SMTP is usually fast (< 1 sec)
- Simpler to implement and debug
- Can add async/job queue in Phase 4+

---

### C.6 Journal Posting Trigger Timing

**Decision**: When does posting to journal happen?

**Options**:

**Option A: On Issue (RECOMMENDED)**
```
invoice.status: draft → issued
  → Trigger: PostInvoice()
  → Creates JournalEntry, LedgerEntries
  → Immutable from here (except void)
```
- ✅ Clear separation: draft = no accounting, issued = posted
- ✅ Matches accounting standards
- ✅ "Send" is operational, not accounting action
- ✅ Can void at any point (audit trail preserved)

**Option B: On Send**
```
invoice.status: draft → sent
  → Trigger: PostInvoice()
```
- ❌ Sends email to customer AFTER posting (wrong order)
- ❌ What if send fails? JE already created but customer unaware

**Option C: Separate Issue + Post Steps**
```
invoice.status: draft → issued (no JE yet)
  → User clicks "Post to Books"
  → invoice.status: issued → posted (JE created)
```
- ✅ Extra control
- ❌ More UI complexity
- ❌ User confusion (two buttons)

**Recommendation**: **Option A** (Post on Issue)
- Invoice "issue" means it's official + posted to books
- "Send" is just operational notification

---

### C.7 Line-Level Snapshots

**Decision**: Store snapshots of Product/Account/Tax in invoice_lines?

**Options**:

**Option A: Snapshot Strategy (RECOMMENDED)**
```go
invoiceline.LineNet = 100
invoiceline.TaxCodeIDSnapshot = 5  // pointing to tax_code.id
invoiceline.RevenueAccountIDSnapshot = 4000  // pointing to account.id
invoiceline.TaxNameSnapshot = "GST 5%"
invoiceline.TaxRateSnapshot = 0.05
invoiceline.RevenueAccountNameSnapshot = "Product Revenue"
```
- ✅ Line is immutable after posting
- ✅ If product/account/tax later modified, invoice still reflects original
- ✅ Audit trail preserved
- ✅ Posting always uses snapshot, never live data

**Option B: FK Only**
```go
invoiceline.ProductServiceID = 1  // FK to product_services
invoiceline.TaxCodeID = 5          // FK to tax_codes
// Fetch current product name, tax rate at render time
```
- ❌ If product renamed, old invoices show new name (misleading)
- ❌ If tax rate changed, recalculating totals gives wrong result
- ❌ Cannot reliably reconstruct historical invoice state

**Recommendation**: **Option A** (Snapshot Strategy)
- Aligns with "immutable posted documents" principle
- Prevents historical data distortion
- More complex schema but safer

---

### C.8 Void Rules & Reversal Strategy

**Decision**: How to handle voiding invoices?

**Options**:

**Option A: Reversal JE (Current Gobooks Pattern)**
```
Invoice1 (draft) → issued
  → JE1 created (DR AR 100, CR Revenue 100)

User voids Invoice1
  → No direct delete of JE1
  → Instead: create JE2 (reversal, opposite entries: CR AR 100, DR Revenue 100)
  → Mark JE1.Status = reversed
  → Mark LE entries from JE1.Status = reversed
  → Create new LE entries from JE2 (reversed entries)
  → Invoice1.Status = voided, but remains in DB
  → Audit log: "invoice.voided", reason, by whom, when
```
- ✅ Audit trail complete
- ✅ All transactions traceable
- ✅ Tax department can reconcile
- ✅ Already implemented in Gobooks (VoidInvoice)

**Option B: Delete**
```
// DELETE from invoices, journal_entries, ledger_entries (cascading)
```
- ❌ Violates audit trail principle
- ❌ Cannot reconstruct historical state
- ❌ Against accounting standards

**Recommendation**: **Option A** (Reversal JE)
- Already implemented in Gobooks
- Use existing VoidInvoice() service

---

## D. Proposed Schema

### D.1 invoices_templates

```sql
CREATE TABLE invoices_templates (
  id BIGSERIAL PRIMARY KEY,
  company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  is_default BOOLEAN NOT NULL DEFAULT false,
  is_active BOOLEAN NOT NULL DEFAULT true,
  config_json JSONB NOT NULL DEFAULT '{}',
  
  created_by UUID REFERENCES users(id),
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  UNIQUE (company_id, is_default) WHERE is_default = true,
  INDEX (company_id),
  INDEX (is_active)
);

-- config_json structure:
{
  "accent_color": "#3498db",
  "show_logo": true,
  "show_company_address": true,
  "show_shipping_address": false,
  "show_tax_summary": true,
  "show_notes": true,
  "show_footer": true,
  "footer_text": "Thank you for your business!",
  "show_payment_instructions": true,
  "payment_instructions": "Payment due by [due_date]",
  "email_subject_template": "Invoice {invoice_number}",
  "email_body_template": "Please find your invoice attached...",
  "table_columns": [
    {"name": "description", "label": "Description", "visible": true},
    {"name": "qty", "label": "Qty", "visible": true},
    {"name": "unit_price", "label": "Unit Price", "visible": true},
    {"name": "line_tax", "label": "Tax", "visible": true},
    {"name": "line_total", "label": "Total", "visible": true}
  ]
}
```

---

### D.2 invoices (extended)

Current Invoice model already has most fields. Need to add:

```sql
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS (
  template_id BIGINT REFERENCES invoices_templates(id) ON DELETE SET NULL,
  -- Snapshots (for immutability after posting)
  customer_email_snapshot VARCHAR(255),
  customer_name_snapshot VARCHAR(255),
  company_address_snapshot TEXT,
  -- Tracking (new)
  issued_at TIMESTAMPTZ,
  sent_at TIMESTAMPTZ,
  viewed_at TIMESTAMPTZ,  -- future: customer portal access
  voided_at TIMESTAMPTZ,
  void_reason TEXT,
  voided_by UUID REFERENCES users(id),
  -- Payment tracking (future)
  paid_amount NUMERIC(18,2) DEFAULT 0,
  balance_due_amount NUMERIC(18,2) DEFAULT 0,
  
  INDEX (template_id),
  INDEX (status),
  INDEX (issued_at),
  INDEX (due_date)
);
```

---

### D.3 invoices (status values refined)

```sql
-- PostgreSQL enum (or CHECK constraint)
ALTER TABLE invoices 
  ALTER COLUMN status TYPE TEXT CHECK (status IN (
    'draft',
    'issued',
    'sent',
    'partially_paid',
    'paid',
    'overdue',
    'void'
  ));
```

---

### D.4 invoices_email_logs

```sql
CREATE TABLE invoices_email_logs (
  id BIGSERIAL PRIMARY KEY,
  company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  invoice_id BIGINT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
  
  to_email VARCHAR(255) NOT NULL,
  cc_email VARCHAR(255),
  subject TEXT NOT NULL,
  body TEXT NOT NULL,
  
  send_status VARCHAR(20) NOT NULL CHECK (send_status IN ('pending', 'sent', 'failed', 'bounced')),
  error_message TEXT,
  
  sent_at TIMESTAMPTZ,
  created_by UUID NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  INDEX (company_id),
  INDEX (invoice_id),
  INDEX (send_status),
  INDEX (created_at)
);

-- Typically: one row per send (can resend same invoice multiple times)
```

---

### D.5 invoices_line (enhanced with snapshots)

```sql
ALTER TABLE invoice_lines ADD COLUMN IF NOT EXISTS (
  revenue_account_id BIGINT,  -- REQUIRED for posting, snapshot of product.revenue_account_id
  revenue_account_snapshot_json TEXT,  -- {id, code, name, account_type}
  
  tax_code_snapshot_json TEXT,  -- {id, name, rate, sales_tax_account_id}
  
  FOREIGN KEY (revenue_account_id) REFERENCES accounts(id),
  INDEX (revenue_account_id)
);
```

---

## E. Proposed Service Boundaries

### E.1 invoice_template.go

```go
package services

// CreateTemplate creates a new template for a company
func CreateTemplate(db *gorm.DB, companyID uint, req *CreateTemplateRequest) (*InvoiceTemplate, error) {
  // Validate companyID exists
  // Validate template name not empty
  // If is_default=true, unset any existing default
  // Create row
}

// LoadTemplate fetches one template
func LoadTemplate(db *gorm.DB, companyID, templateID uint) (*InvoiceTemplate, error) {
  // Validate company ownership
  // Return template or error
}

// GetDefaultTemplate returns company's default (or system global default)
func GetDefaultTemplate(db *gorm.DB, companyID uint) (*InvoiceTemplate, error)

// ListTemplates lists all active templates for company
func ListTemplates(db *gorm.DB, companyID uint) ([]InvoiceTemplate, error)

// UpdateTemplate updates config, name, activeity
func UpdateTemplate(db *gorm.DB, companyID, templateID uint, req *UpdateTemplateRequest) error

// SetDefaultTemplate marks one as default, unsets others
func SetDefaultTemplate(db *gorm.DB, companyID, templateID uint) error

// DeactivateTemplate soft-deletes (sets is_active=false)
func DeactivateTemplate(db *gorm.DB, companyID, templateID uint) error
```

---

### E.2 invoice_pdf.go (NEW)

```go
package services

// GenerateInvoicePDF generates PDF from invoice + template
func GenerateInvoicePDF(db *gorm.DB, companyID, invoiceID uint, templateID *uint) ([]byte, error) {
  // Load invoice (with lines, customer, company, tax codes, products)
  // Resolve template (if nil, use default)
  // Render HTML
  // Convert to PDF via wkhtmltopdf
  // Return PDF bytes
}

// RenderInvoiceHTML generates HTML for preview/PDF
func RenderInvoiceHTML(invoice *Invoice, template *InvoiceTemplate, company *Company) (string, error) {
  // Using Go text/template
  // Binds invoice data to template config
  // Returns formatted HTML string
}
```

---

### E.3 invoice_notification.go (NEW)

```go
package services

// SendInvoiceEmail sends invoice to customer via SMTP
func SendInvoiceEmail(
  db *gorm.DB,
  companyID, invoiceID uint,
  recipientEmail string,  // override customer email if needed
  actor string,           // user email
  userID *uuid.UUID,
) error {
  // 1. Check SMTP ready: EffectiveSMTPForCompany(db, companyID)
  //    if not ready, return error (DO NOT SEND)
  // 2. Load invoice + template
  // 3. Generate PDF
  // 4. Build email (subject, body from template; attach PDF)
  // 5. Send via SendEmail()
  // 6. Log to invoices_email_logs (with status=sent or failed)
  // 7. Update invoice.sent_at = now()
  // 8. Write audit log: "invoice.sent"
  // 9. Return error if any
}

// GetNotificationSettings fetches company notification prefs
func GetNotificationSettings(db *gorm.DB, companyID uint) (*InvoiceNotificationSettings, error)

// SaveNotificationSettings updates notification prefs
func SaveNotificationSettings(db *gorm.DB, companyID uint, settings *InvoiceNotificationSettings) error
```

---

### E.4 invoice_lifecycle.go (NEW - Status Transitions)

```go
package services

// IssueInvoice transitions draft → issued + posts to journal
func IssueInvoice(
  db *gorm.DB,
  companyID, invoiceID uint,
  actor string,
  userID *uuid.UUID,
) error {
  // 1. Load invoice (must be draft)
  // 2. Validate: customer exists, lines non-empty, amounts valid
  // 3. Validate: all revenue accounts OK
  // 4. Call PostInvoice() → creates JE
  // 5. Update invoice.Status = issued, invoice.IssuedAt = now()
  // 6. Audit log: "invoice.issued"
  // 7. Return
}

// VoidInvoice creates reversal JE + marks as void
func VoidInvoice(db *gorm.DB, companyID, invoiceID uint, voidReason string, actor string, userID *uuid.UUID) error {
  // 1. Load invoice (must not already voided)
  // 2. If not posted: just update status to void, no JE needed
  // 3. If posted: call ReverseJournalEntry() → creates reversal JE
  // 4. Update invoice.Status=void, invoice.VoidedAt=now(), invoice.VoidReason=reason
  // 5. Audit log: "invoice.voided"
}

// CanIssueInvoice checks if invoice eligible for issue
func CanIssueInvoice(invoice *Invoice) (bool, error)

// CanVoidInvoice checks if invoice eligible for void
func CanVoidInvoice(invoice *Invoice) (bool, error)

// TransitionToSent marks issued → sent after email send succeeds
func TransitionToSent(db *gorm.DB, companyID, invoiceID uint) error {
  // Update invoice.SentAt = now() (if not already sent)
}
```

---

### E.5 invoice_validation.go (NEW)

```go
package services

// PreIssueValidation checks invoice before posting
func PreIssueValidation(db *gorm.DB, invoice *Invoice, companyID uint) []string {
  // returns []errors (empty = pass)
  // Checks:
  //   - customer exists + belongs to company
  //   - lines non-empty
  //   - all line amounts valid (qty > 0, price >= 0)
  //   - all revenue accounts exist + belong to company
  //   - all tax codes exist + belong to company (if set)
  //   - total = subtotal + tax (recalculate to verify)
}

// PreSendValidation checks invoice before sending email
func PreSendValidation(db *gorm.DB, invoice *Invoice, companyID uint) []string {
  // returns []errors
  // Checks:
  //   - invoice must be issued (not draft)
  //   - customer has email address
  //   - SMTP is ready
}
```

---

## F. Proposed Status Model

### F.1 Invoice Status State Machine (Go Constants)

```go
package models

type InvoiceStatus string

const (
  InvoiceStatusDraft       InvoiceStatus = "draft"        // editable, no JE
  InvoiceStatusIssued      InvoiceStatus = "issued"       // JE created, locked
  InvoiceStatusSent        InvoiceStatus = "sent"         // issued + emailed
  InvoiceStatusPartiallyPaid InvoiceStatus = "partially_paid"  // issued + payment < total
  InvoiceStatusPaid        InvoiceStatus = "paid"         // issued + payment >= total
  InvoiceStatusOverdue     InvoiceStatus = "overdue"      // issued + due_date < today && unpaid
  InvoiceStatusVoid        InvoiceStatus = "void"         // reversed, immutable
)

// State Transition Rules (enforced in service layer):
// draft → issued           (action: issue invoice)
// issued → sent            (action: send email)
// issued → partially_paid  (derived: payment recorded)
// partially_paid → paid    (derived: more payment received)
// issued/sent/paid → overdue (derived: scheduler/query)
// draft → void             (action: void - no JE reverse)
// issued/sent/paid → void  (action: void - creates reversal JE)
```

---

### F.2 Status Derivation

Some statuses are derived, not directly set:

```go
// Function to compute current status from invoice state
func ComputeCurrentStatus(inv *Invoice) InvoiceStatus {
  if inv.VoidedAt != nil {
    return InvoiceStatusVoid
  }
  
  if inv.IssuedAt == nil {
    return InvoiceStatusDraft
  }
  
  // issued or later
  if inv.PaidAmount >= inv.Amount {
    return InvoiceStatusPaid
  }
  
  if inv.PaidAmount > 0 {
    return InvoiceStatusPartiallyPaid
  }
  
  // Check if overdue
  if inv.DueDate != nil && time.Now().After(*inv.DueDate) {
    return InvoiceStatusOverdue
  }
  
  // Check if sent
  if inv.SentAt != nil {
    return InvoiceStatusSent
  }
  
  return InvoiceStatusIssued
}
```

---

## G. Proposed Posting Integration Approach

### G.1 Existing PostInvoice Flow (already works)

```
1. Handler receives: POST /invoices/:id/post
2. Calls: services.PostInvoice(db, companyID, invoiceID, actor, userID)
3. PostInvoice:
   a. Load invoice + lines preloaded
   b. Pre-flight checks (customer, accounts, tax codes valid)
   c. Resolve AR account
   d. BuildInvoiceFragments(invoice, arAccountID) → []PostingFragment
   e. AggregateJournalLines(fragments) → []PostingFragment
   f. Validate double-entry (debit == credit)
   g. BEGIN transaction
      i. SELECT FOR UPDATE on invoice (prevent concurrent posting)
      ii. Re-check status == draft (inside lock)
      iii. INSERT journal_entries header (source_type='invoice', source_id=inv.id)
      iv. INSERT journal_lines (one per aggregated fragment)
      v. ProjectToLedger() → INSERT ledger_entries
      vi. UPDATE invoices SET status='sent', journal_entry_id=je.id  [NOW CALLED "issued"]
      vii. WriteAuditLog(...)
   h. COMMIT
```

**Current code**:
- `services/invoice_post.go`: PostInvoice()
- `services/fragment_builder.go`: BuildInvoiceFragments()
- `services/journal_aggregate.go`: AggregateJournalLines()
- `services/ledger.go`: ProjectToLedger()

---

### G.2 Integration Points (NEW CODE)

For Invoice Module, we need to call existing functions correctly:

**New Service**: `invoice_lifecycle.go::IssueInvoice()`
```go
func IssueInvoice(db *gorm.DB, companyID, invoiceID uint, actor string, userID *uuid.UUID) error {
  // 1. Validation
  inv, err := loadAndValidateInvoice(db, companyID, invoiceID)
  if err != nil {
    return err
  }
  
  // 2. Call existing PostInvoice (does both JE creation + validation)
  err = PostInvoice(db, companyID, invoiceID, actor, userID)
  if err != nil {
    return err
  }
  
  // 3. Update invoice tracking fields
  err = db.Model(&inv).Updates(map[string]interface{}{
    "issued_at": time.Now(),
    "status": InvoiceStatusIssued,
  }).Error
  
  // 4. Audit
  WriteAuditLogWithContext(db, "invoice.issued", "invoice", invoiceID, actor, 
    map[string]any{"invoice_number": inv.InvoiceNumber}, &companyID, userID)
  
  return nil
}
```

---

### G.3 Concurrency & Idempotency

Existing PostInvoice already handles:
- SELECT FOR UPDATE (concurrency lock)
- Unique partial index on (company_id, source_type='invoice', source_id)
- Status re-check inside transaction

**No new code needed** for this part.

---

## H. Risks / Unknowns

### H.1 Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|--------------|--------|-----------|
| wkhtmltopdf not available in deployment | Medium | Cannot generate PDF | Document installation in README; add fallback gofpdf option |
| SMTP roundtrip timeout in HTTP request | Low | User sees "timeout" | Add 10s timeout + async email option in Phase 3+ |
| PDF generation performance (large invoices) | Low | Slow UI | Cache PDF or lazy-generate |
| Template HTML injection / XSS | Medium | Security issue | Escape all template variables; use safe templating |
| Double-posting race condition | Very Low | Duplicate JE | Existing SELECT FOR UPDATE + unique index handles |

---

### H.2 Design Risks

| Risk | Probability | Impact | Mitigation |
|------|--------------|--------|-----------|
| Status model not flexible enough | Low | Future phases blocked | Support custom statuses/hooks? Not for MVP |
| Snapshot strategy incompatible with corrections | Low | Cannot fix posted lines | Accept as-is for MVP; future "adjust" document |
| Template system too rigid | Medium | User frustration | Plan custom CSS override in Phase 2+ |
| Email without attachment support | Low | Need to extend SendEmail | Extend email_sender.go to support attachments |

---

### H.3 Unknowns

| Item | Needs Clarification |
|------|---------------------|
| **Payment tracking scope** | Does Phase 1 include full payment recording, or just tracking fields? |
| **Recurring invoices** | Defer to Phase 5, confirmed? |
| **Customer portal** | Defer entirely, confirmed? |
| **Multi-currency** | Not in MVP, confirmed? |
| **Draft deletion** | Should draft invoices be soft-deleted (marked inactive) or hard-deleted? |
| **Partial send** | If email fails, update invoice status to "sent" anyway, or fail transaction? |

---

## I. Recommended Implementation Order

### I.1 Phasing

```
Phase 1: Data Model & Migrations
  ├─ Create invoices_templates table
  ├─ Create invoices_email_logs table
  ├─ Extend invoices table (template_id, snapshots, tracking fields)
  └─ Create Go models & GORM associations

Phase 2: Core Invoice Lifecycle
  ├─ Implement invoice_lifecycle.go (IssueInvoice, VoidInvoice, status transitions)
  ├─ Implement invoice_validation.go (pre-flight checks)
  ├─ Implement invoice_template.go (CRUD services)
  ├─ Unit tests for all business rules
  └─ Handler updates (POST /invoices/:id/issue, error handling)

Phase 3: Template & PDF
  ├─ Setup wkhtmltopdf integration
  ├─ Implement invoice_pdf.go (RenderInvoiceHTML, GenerateInvoicePDF)
  ├─ Create invoice template HTML (modern + classic variants)
  ├─ Handlers: GET /invoices/:id/pdf, GET /invoices/:id/preview-pdf
  ├─ UI: template editor page, PDF preview modal
  └─ Tests: template rendering, PDF generation

Phase 4: Email & Notifications
  ├─ Implement invoice_notification.go (SendInvoiceEmail)
  ├─ Extend email_sender.go for PDF attachment support
  ├─ Create invoices_email_logs logging
  ├─ Handlers: POST /invoices/:id/send, settings page
  ├─ SMTP readiness validation (pre-check)
  ├─ UI: send button, email history modal
  └─ Tests: SMTP mocking, error scenarios

Phase 5: UI & User Experience
  ├─ Update invoice list page (add template selection, sent status)
  ├─ Update invoice editor (template selector, issue button)
  ├─ Create template management page (Settings → Templates)
  ├─ Add invoice detail view (show JE, sent log, payments)
  ├─ Markdown emails (body formatting)
  └─ Polish UX

Phase 6: Testing & Hardening
  ├─ Integration tests (end-to-end issue→send→pay)
  ├─ Concurrency tests (double-posting, race conditions)
  ├─ Permission tests (cross-company access denial)
  ├─ Edge cases (large invoices, special chars, timezones)
  ├─ Performance tests (PDF generation, email queueing)
  └─ Deployment testing (environment setup, wkhtmltopdf validation)
```

---

### I.2 Critical Path (MVP)

**Minimum to enable first customer invoice**:
1. ✅ Invoice models (data + migrations)
2. ✅ Template system (CRUD + rendering)
3. ✅ PDF export (HTML → PDF)
4. ✅ Email send (with SMTP check)
5. ✅ Status lifecycle (draft → issued → sent)
6. ✅ Permissions (who can issue/send)
7. ✅ Basic UI (template editor, send button)

**Not critical for MVP**:
- Payment tracking (fields only, service later)
- Reminders (Phase 6)
- Recurring (Phase 5)
- Customer portal (Phase 7)
- Advanced analytics / reporting

---

## J. Proposed File Structure (New Files)

```
internal/models/
  invoices_template.go          [NEW] InvoiceTemplate struct + validation
  invoice_line_extended.go      [EXTEND] Add snapshots to InvoiceLine
  
internal/services/
  invoice_template.go           [NEW] Template CRUD service
  invoice_lifecycle.go          [NEW] Issue/Void/Status transitions
  invoice_validation.go         [NEW] Pre-flight checks
  invoice_pdf.go                [NEW] PDF generation
  invoice_notification.go       [NEW] Email sending + logs
  email_sender.go               [EXTEND] Add attachment support
  
internal/web/
  invoice_templates_handlers.go [NEW] Template CRUD handlers
  invoices_handlers.go          [EXTEND] Add issue/send handlers
  
internal/web/templates/pages/
  invoice_templates_list.templ  [NEW] Template list page
  invoice_templates_edit.templ  [NEW] Template edit page
  invoices_list.templ           [UPDATE] Add sent/status columns
  invoices_detail.templ         [UPDATE] Add issue/send buttons, email log
  
migrations/
  024_invoice_templates.sql     [NEW] Create invoices_templates table
  025_invoices_extended.sql     [NEW] Add tracking fields to invoices
  026_invoice_email_logs.sql    [NEW] Create invoices_email_logs table
```

---

## K. Design Decisions NOT Made Yet (For Approval)

| Decision | Options | Recommendation | Needs Sign-Off |
|----------|---------|-----------------|-----------------|
| **Status model** | Simple (draft→sent→paid) vs Full (draft→issued→sent→...) | Full state machine | ✅ YES |
| **PDF library** | wkhtmltopdf vs gofpdf vs chromedp | wkhtmltopdf | ✅ YES |
| **Template storage** | Config only vs HTML blob | Config only | ✅ YES |
| **Email async** | Sync vs Async/JobQueue | Sync for MVP | ✅ YES |
| **Posting trigger** | On issue vs On send | On issue | ✅ YES |
| **Snapshot strategy** | Full snapshot vs FK only | Full snapshot | ✅ YES |
| **Payment scope** | Record only vs Reconciliation | Record only | ✅ Confirm |
| **Draft deletion** | Hard delete vs Soft (inactive) | Hard delete for now | ✅ Confirm |

---

## L. Conclusion

### L.1 Summary

Gobooks already has:
- ✅ Posting engine, tax engine, fragment builder
- ✅ Company isolation, audit trail, permissions
- ✅ SMTP infrastructure
- ✅ Basic invoice CRUD

Gobooks is missing:
- ❌ Invoice templates (render customizations)
- ❌ PDF export (wkhtmltopdf integration)
- ❌ Email sending (SendEmail + email logs)
- ❌ State machine / lifecycle rules
- ❌ Email attachment support

### L.2 Readiness Assessment

**Ready to start Phase 2** (Data Model & Migrations) if:
- ✅ Design decisions approved (Section K)
- ✅ Unknown risks clarified (Section H.3)
- ✅ Team agrees on status model (draft→issued→sent)
- ✅ wkhtmltopdf approved as PDF solution
- ✅ No architectural blockers identified

### L.3 Next Steps

1. **Review this assessment** (30 min)
2. **Approve design decisions** (Section K)
3. **Clarify unknowns** (Section H.3)
4. **Green light Phase 2** → Start migrations + models
5. **Kick off Phase 2 spec document** (data migrations + Go models)

---

**Assessment Completed**: 2026-03-30 14:00 UTC  
**Status**: 🔴 AWAITING APPROVAL  
**Next Phase**: Phase 2 (Data Model & Migration)

