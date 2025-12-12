package main

import (
	"net/http"
)

type gamesView struct {
	Path  string
	Title string
	Games []Game
}

func newGameView() gamesView {
	return gamesView{
		Path:  "/games",
		Title: "Kampe",
	}
}

func (g gamesView) withGames(games []Game) gamesView {
	g.Games = games
	return g
}

func (a *App) handleGames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	games, err := a.listGames()
	if err != nil {
		http.Error(w, "failed to load games", http.StatusInternalServerError)
		return
	}

	page := newGameView().withGames(games)
	renderTemplate(w, "layout", page, "templates/layout.html", "templates/games.html")
}
