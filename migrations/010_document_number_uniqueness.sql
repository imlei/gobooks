-- Enforce case-insensitive document-number uniqueness at the database layer.
-- Invoice Number: unique within company (case-insensitive).
CREATE UNIQUE INDEX IF NOT EXISTS uq_invoices_company_invoice_number_ci
ON invoices (company_id, lower(invoice_number));

-- Bill Number: unique within company + vendor (case-insensitive).
CREATE UNIQUE INDEX IF NOT EXISTS uq_bills_company_vendor_bill_number_ci
ON bills (company_id, vendor_id, lower(bill_number));
