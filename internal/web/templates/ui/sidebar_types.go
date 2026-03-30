// 遵循project_guide.md
package ui

// SidebarVM is a small view-model for consistent sidebar rendering.
// Keeping it explicit helps keep UI behavior predictable.
type SidebarVM struct {
	Active     string
	HasCompany bool
	UserEmail  string // optional; shown in top-bar user menu when set
}

