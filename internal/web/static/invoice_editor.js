// invoice_editor.js — Alpine component for the invoice line-items editor.
// v=1
function invoiceEditor() {
  return {
    lines: [],
    products: [],
    taxCodes: [],
    terms: "net_30",
    invoiceDate: "",
    dueDate: "",
    dueDateEditable: false,

    init() {
      const el = this.$el;
      this.products = JSON.parse(el.dataset.products || "[]");
      this.taxCodes = JSON.parse(el.dataset.taxCodes || "[]");
      this.terms = el.dataset.initialTerms || "net_30";
      this.invoiceDate = el.dataset.initialDate || "";
      this.dueDate = el.dataset.initialDueDate || "";
      this.dueDateEditable = this.terms === "custom";

      const initial = JSON.parse(el.dataset.initialLines || "[]");
      if (initial.length > 0) {
        this.lines = initial;
      } else {
        this.addLine();
      }
    },

    // ── Line management ──────────────────────────────────────────────────────

    addLine() {
      this.lines.push({
        product_service_id: "",
        description: "",
        qty: "1",
        unit_price: "0.00",
        tax_code_id: "",
        line_net: "0.00",
      });
    },

    removeLine(idx) {
      if (this.lines.length > 1) {
        this.lines.splice(idx, 1);
      }
    },

    onProductChange(idx, psId) {
      if (!psId) return;
      const ps = this.products.find((p) => String(p.id) === String(psId));
      if (!ps) return;
      const line = this.lines[idx];
      // Only pre-fill description when blank so manual edits are preserved.
      if (!line.description) line.description = ps.name;
      line.unit_price = ps.default_price;
      if (ps.default_tax_code_id) {
        line.tax_code_id = String(ps.default_tax_code_id);
      }
      this.calcLine(idx);
    },

    calcLine(idx) {
      const line = this.lines[idx];
      const qty = parseFloat(line.qty) || 0;
      const price = parseFloat(line.unit_price) || 0;
      line.line_net = (qty * price).toFixed(2);
    },

    subtotal() {
      const s = this.lines.reduce(
        (acc, l) => acc + (parseFloat(l.line_net) || 0),
        0
      );
      return s.toFixed(2);
    },

    // ── Terms / due-date auto-computation ────────────────────────────────────

    onTermsChange(val) {
      this.terms = val;
      this.dueDateEditable = val === "custom";
      if (val !== "custom") {
        this.dueDate = this._computeDueDate(this.invoiceDate, val);
      }
    },

    onDateChange(val) {
      this.invoiceDate = val;
      if (this.terms !== "custom") {
        this.dueDate = this._computeDueDate(val, this.terms);
      }
    },

    _computeDueDate(dateStr, terms) {
      const days = {
        net_15: 15,
        net_30: 30,
        net_60: 60,
        due_on_receipt: 0,
      }[terms];
      if (days === undefined) return "";
      const d = new Date(dateStr);
      if (isNaN(d.getTime())) return "";
      d.setDate(d.getDate() + days);
      return d.toISOString().slice(0, 10);
    },
  };
}
