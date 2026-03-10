package main

import "net/http"

type shortcutsView struct {
	Path  string
	Title string
	seasonContext
}

func newShortcutsView() shortcutsView {
	return shortcutsView{
		Path:  "/shortcuts",
		Title: "Genveje",
	}
}

func (a *App) handleShortcuts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, _, err := a.loadSeasonContext(r)
	if err != nil {
		http.Error(w, "loading seasons", http.StatusInternalServerError)
		return
	}

	view := newShortcutsView()
	view.seasonContext = ctx
	renderTemplate(w, "layout", view, "templates/layout.html", "templates/shortcuts.html")
}
