package dashboard

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

func loadTemplates() (*template.Template, error) {
	funcs := template.FuncMap{
		"join": formatEmployees,
	}
	return template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
}
