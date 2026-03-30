// 遵循project_guide.md
package pages

// SetupViewModel is a small struct used by the setup page template.
// Keeping it simple makes templates easy to read and maintain.
type SetupViewModel struct {
	Active string
	Values SetupFormValues
	Errors SetupFormErrors
}

// SetupFormValues holds the user's input so we can re-render it when validation fails.
type SetupFormValues struct {
	CompanyName    string
	EntityType     string
	BusinessType   string
	AddressLine    string
	City           string
	Province       string
	PostalCode     string
	Country        string
	BusinessNumber string
	Industry       string
	IncorporatedDate string
	FiscalYearEnd  string
	// AccountCodeLength: "4".."12"; empty means default 4 at submit.
	AccountCodeLength string
}

// SetupFormErrors holds simple validation messages for the form.
type SetupFormErrors struct {
	Form           string
	CompanyName    string
	EntityType     string
	BusinessType   string
	AddressLine    string
	City           string
	Province       string
	PostalCode     string
	Country        string
	BusinessNumber string
	Industry       string
	IncorporatedDate string
	FiscalYearEnd  string
	AccountCodeLength string
}

func (e SetupFormErrors) HasAny() bool {
	return e.Form != "" ||
		e.CompanyName != "" ||
		e.EntityType != "" ||
		e.BusinessType != "" ||
		e.AddressLine != "" ||
		e.City != "" ||
		e.Province != "" ||
		e.PostalCode != "" ||
		e.Country != "" ||
		e.BusinessNumber != "" ||
		e.Industry != "" ||
		e.IncorporatedDate != "" ||
		e.FiscalYearEnd != "" ||
		e.AccountCodeLength != ""
}

