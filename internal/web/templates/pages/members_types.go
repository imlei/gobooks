// 遵循project_guide.md
package pages

// MembersVM is Settings → Members (list + invite).
type MembersVM struct {
	HasCompany bool

	Active string

	ReadOnly bool

	Members      []MemberRow
	Invitations  []InvitationRow

	FormError  string
	EmailError string
	RoleError  string

	Email string
	Role  string

	Created bool
}

// MemberRow is one active company membership row for display.
type MemberRow struct {
	Email string
	Role  string
	Since string
}

// InvitationRow is one pending invitation for display.
type InvitationRow struct {
	Email       string
	Role        string
	Expires     string
	InvitedBy   string
	Created     string
	IsExpired   bool
}
