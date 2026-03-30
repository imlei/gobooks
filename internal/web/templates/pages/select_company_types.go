// 遵循project_guide.md
package pages

// SelectCompanyVM is used when the user must choose an active company.
type SelectCompanyVM struct {
	Rows      []SelectCompanyRowVM
	FormError string
}

// SelectCompanyRowVM is one selectable company (active membership).
type SelectCompanyRowVM struct {
	CompanyID    uint
	CompanyIDStr string
	Name         string
	RoleLabel    string
}
