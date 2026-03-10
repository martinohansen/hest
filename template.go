package main

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/carlmjohnson/versioninfo"
)

func renderTemplate(w http.ResponseWriter, tplName string, data any, files ...string) {
	if err := render(tplName, w, data, files...); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func render(tplName string, w http.ResponseWriter, data any, files ...string) error {
	for i, f := range files {
		files[i] = filepath.Clean(f)
	}
	funcs := template.FuncMap{
		"add":      func(a, b int) int { return a + b },
		"subtract": func(a, b int) int { return a - b },
		"version":  func() string { return versioninfo.Revision[:7] },
		"hasEasterEgg": func(easterEggs, name string) bool {
			if easterEggs == "" {
				return false
			}
			for _, e := range strings.Split(easterEggs, ",") {
				if strings.TrimSpace(e) == name {
					return true
				}
			}
			return false
		},
	}
	tpl, err := template.New(filepath.Base(files[0])).Funcs(funcs).ParseFS(templateFS, files...)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tpl.ExecuteTemplate(w, tplName, data)
}
