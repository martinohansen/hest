package main

import (
	"net/http"
)

type gamesView struct {
	Path       string
	Title      string
	Games      []Game
	TotalGames int
	seasonContext
}

func newGameView() gamesView {
	return gamesView{
		Path:  "/games",
		Title: "Kampe",
	}
}

func (g gamesView) withGames(games []Game) gamesView {
	g.Games = games
	g.TotalGames = len(games)
	return g
}

func (a *App) handleGames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, selectedSeason, err := a.loadSeasonContext(r)
	if err != nil {
		http.Error(w, "failed to load seasons", http.StatusInternalServerError)
		return
	}

	games, err := a.listGames(seasonFilter(selectedSeason))
	if err != nil {
		http.Error(w, "failed to load games", http.StatusInternalServerError)
		return
	}

	page := newGameView().withGames(games)
	page.seasonContext = ctx
	renderTemplate(w, "layout", page, "templates/layout.html", "templates/games.html")
}
