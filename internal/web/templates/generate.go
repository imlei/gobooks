// 遵循project_guide.md
package templates

// This file enables `go generate ./...` to build templ components.
//
// Release build prerequisite:
//   1. Run `go generate ./...` from the repo root before `go build` or `go test`.
//   2. Commit the generated `*_templ.go` files so clean CI / release builds do not
//      depend on a preinstalled local templ binary.
//
//go:generate go run github.com/a-h/templ/cmd/templ@v0.3.1001 generate

