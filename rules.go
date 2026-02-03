package main

import "net/http"

type rulesView struct {
	Path  string
	Title string
	seasonContext
}

func newRulesView() rulesView {
	return rulesView{
		Path:  "/rules",
		Title: "Regler",
	}
}

func (a *App) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, _, err := a.loadSeasonContext(r)
	if err != nil {
		http.Error(w, "loading seasons", http.StatusInternalServerError)
		return
	}

	view := newRulesView()
	view.seasonContext = ctx
	renderTemplate(w, "layout", view, "templates/layout.html", "templates/rules.html")
}
