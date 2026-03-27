// 遵循产品需求 v1.0
package models

import "fmt"

// CompanyRole is the PostgreSQL enum company_role (role lives only on membership).
type CompanyRole string

const (
	CompanyRoleOwner      CompanyRole = "owner"
	CompanyRoleAdmin      CompanyRole = "admin"
	CompanyRoleBookkeeper CompanyRole = "bookkeeper"
	CompanyRoleAccountant CompanyRole = "accountant"
	CompanyRoleAP         CompanyRole = "ap"
	CompanyRoleViewer     CompanyRole = "viewer"
)

func (r CompanyRole) Valid() bool {
	switch r {
	case CompanyRoleOwner,
		CompanyRoleAdmin,
		CompanyRoleBookkeeper,
		CompanyRoleAccountant,
		CompanyRoleAP,
		CompanyRoleViewer:
		return true
	default:
		return false
	}
}

func ParseCompanyRole(s string) (CompanyRole, error) {
	r := CompanyRole(s)
	if !r.Valid() {
		return "", fmt.Errorf("invalid company role: %q", s)
	}
	return r, nil
}
