package main

import (
	"net/http"

	"github.com/martinohansen/hest/internal/db"
)

type PlayerGameHistoryEntry db.PlayerGameHistoryEntry

type playerDetailView struct {
	Path        string
	Player      Player
	GameHistory []PlayerGameHistoryEntry
	HasGames    bool
}

func newPlayerDetailView(player Player) playerDetailView {
	return playerDetailView{
		Path:   "/player",
		Player: player,
	}
}

func (p playerDetailView) withGameHistory(history []PlayerGameHistoryEntry) playerDetailView {
	p.GameHistory = history
	p.HasGames = len(history) > 0
	return p
}

func (a *App) handlePlayerDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerIDStr := r.URL.Query().Get("id")
	if playerIDStr == "" {
		http.Error(w, "player id required", http.StatusBadRequest)
		return
	}

	playerID, err := parsePlayer(playerIDStr)
	if err != nil {
		http.Error(w, "invalid player id", http.StatusBadRequest)
		return
	}

	players, err := a.playersByIDs([]int{playerID})
	if err != nil || len(players) == 0 {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}
	player := players[0]

	historyDB, err := a.store.PlayerGameHistory(playerID)
	if err != nil {
		http.Error(w, "failed to load player history", http.StatusInternalServerError)
		return
	}

	history := make([]PlayerGameHistoryEntry, len(historyDB))
	for i, h := range historyDB {
		history[i] = PlayerGameHistoryEntry(h)
	}

	view := newPlayerDetailView(player).withGameHistory(history)
	renderTemplate(w, "layout", view, "templates/layout.html", "templates/player.html")
}
