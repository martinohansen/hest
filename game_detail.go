package main

import (
	"net/http"
	"strconv"
)

type gameDetailView struct {
	Path  string
	Title string
	Game  Game
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

	gameID, err := strconv.Atoi(gameIDStr)
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

	view := gameDetailView{
		Path:  "/game",
		Title: "Kamp #" + gameIDStr,
		Game:  Game(gameDB),
	}
	view.seasonContext = ctx

	renderTemplate(w, "layout", view, "templates/layout.html", "templates/game.html")
}
