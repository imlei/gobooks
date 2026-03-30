// bill_editor.js — Alpine component for the bill line-items editor.
// v=1
function billEditor() {
  return {
    lines: [],
    accounts: [],
    taxCodes: [],
    terms: "net_30",
    billDate: "",
    dueDate: "",
    dueDateEditable: false,

    init() {
      const el = this.$el;
      this.accounts = JSON.parse(el.dataset.accounts || "[]");
      this.taxCodes = JSON.parse(el.dataset.taxCodes || "[]");
      this.terms = el.dataset.initialTerms || "net_30";
      this.billDate = el.dataset.initialDate || "";
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
        expense_account_id: "",
        description: "",
        amount: "0.00",
        tax_code_id: "",
        line_net: "0.00",
      });
    },

    removeLine(idx) {
      if (this.lines.length > 1) {
        this.lines.splice(idx, 1);
      }
    },

    calcLine(idx) {
      const line = this.lines[idx];
      line.line_net = (parseFloat(line.amount) || 0).toFixed(2);
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
        this.dueDate = this._computeDueDate(this.billDate, val);
      }
    },

    onDateChange(val) {
      this.billDate = val;
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
