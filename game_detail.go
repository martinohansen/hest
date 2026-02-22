package main

import (
	"net/http"
)

type gameDetailView struct {
	Path      string
	Title     string
	Game      Game
	MaxGameID int
	seasonContext
}

func (a *App) handleGameDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, _, err := a.loadSeasonContext(r)
	if err != nil {
		http.Error(w, "failed to load seasons", http.StatusInternalServerError)
		return
	}

	gameIDStr := r.URL.Query().Get("id")
	if gameIDStr == "" {
		http.Error(w, "game id required", http.StatusBadRequest)
		return
	}

	gameID, err := parseGameID(gameIDStr)
	if err != nil {
		http.Error(w, "invalid game id", http.StatusBadRequest)
		return
	}

	gameDB, ok, err := a.store.GetGameByID(gameID)
	if err != nil {
		http.Error(w, "failed to load game", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	maxGameID, err := a.store.GetMaxGameID()
	if err != nil {
		http.Error(w, "failed to load max game ID", http.StatusInternalServerError)
		return
	}

	view := gameDetailView{
		Path:      "/game",
		Title:     "Kamp #" + gameIDStr,
		Game:      Game(gameDB),
		MaxGameID: maxGameID,
	}
	view.seasonContext = ctx

	renderTemplate(w, "layout", view, "templates/layout.html", "templates/game.html")
}
