// Package assets holds the static files embedded into the binary that are
// consumed by the internal packages (email templates, osquery bootstrap
// scripts, and the Casbin RBAC model). It is a leaf package — it imports only
// the standard library — and exposes the assets as sub-filesystems / strings so
// consumers receive them via constructor parameters rather than reaching into
// package globals.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed email/*.tmpl
var emailFS embed.FS

//go:embed osquery/*.tmpl
var osqueryFS embed.FS

//go:embed rbac/rbac_model.conf
var rbacFS embed.FS

// EmailTemplates returns the alert email templates rooted at their filenames
// (email.html.tmpl, email.text.tmpl).
func EmailTemplates() (fs.FS, error) { return fs.Sub(emailFS, "email") }

// BootstrapTemplates returns the osquery bootstrap script templates rooted at
// their filenames (linux.sh.tmpl, macos.sh.tmpl, windows.ps1.tmpl).
func BootstrapTemplates() (fs.FS, error) { return fs.Sub(osqueryFS, "osquery") }

// RBACModel returns the Casbin model definition.
func RBACModel() (string, error) {
	b, err := fs.ReadFile(rbacFS, "rbac/rbac_model.conf")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
