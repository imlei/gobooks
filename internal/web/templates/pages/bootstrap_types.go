// 遵循project_guide.md
package pages

// BootstrapViewModel drives the one-time bootstrap page (first user + first company).
type BootstrapViewModel struct {
	Active string
	Values BootstrapFormValues
	Errors BootstrapFormErrors
}

// BootstrapFormValues extends the company setup fields with owner account fields.
type BootstrapFormValues struct {
	SetupFormValues
	Email             string
	Password          string
	PasswordConfirm   string
	DisplayName       string
}

// BootstrapFormErrors extends setup errors with account field messages.
type BootstrapFormErrors struct {
	SetupFormErrors
	Email           string
	Password        string
	PasswordConfirm string
	DisplayName     string
}

func (e BootstrapFormErrors) HasAny() bool {
	return e.SetupFormErrors.HasAny() ||
		e.Email != "" ||
		e.Password != "" ||
		e.PasswordConfirm != "" ||
		e.DisplayName != ""
}
