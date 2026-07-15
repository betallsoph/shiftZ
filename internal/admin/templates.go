package admin

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

func loadTemplates() (*template.Template, error) {
	return template.ParseFS(templateFS, "templates/*.html")
}

type templateSet struct {
	root *template.Template
}

func (t *templateSet) render(w http.ResponseWriter, name string, data any) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	return t.root.ExecuteTemplate(w, name, data)
}
