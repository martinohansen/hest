package main

import (
	"net/http"

	"github.com/martinohansen/hest/internal/db"
)

type PlayerGameHistoryEntry db.PlayerGameHistoryEntry
type PlayerRankHistoryEntry db.PlayerRankHistoryEntry

type playerDetailView struct {
	Path         string
	Title        string
	Player       Player
	GameHistory  []PlayerGameHistoryEntry
	RankHistory  []PlayerRankHistoryEntry
	Games        []Game
	HasGames     bool
	TotalPlayers int
	Rank         int
}

func newPlayerDetailView(player Player, rank int) playerDetailView {
	return playerDetailView{
		Path:   "/player",
		Title:  player.Name,
		Player: player,
		Rank:   rank,
	}
}

func (p playerDetailView) withGameHistory(history []PlayerGameHistoryEntry) playerDetailView {
	p.GameHistory = history
	p.HasGames = len(history) > 0
	return p
}

func (p playerDetailView) withRankHistory(history []PlayerRankHistoryEntry) playerDetailView {
	p.RankHistory = history
	return p
}

func (p playerDetailView) withGames(games []Game) playerDetailView {
	p.Games = games
	return p
}

func (p playerDetailView) withTotalPlayers(total int) playerDetailView {
	p.TotalPlayers = total
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

	players, err := a.Leaderboard()
	if err != nil {
		http.Error(w, "failed to load players", http.StatusInternalServerError)
		return
	}

	// Find player and its rank
	var player Player
	var rank int
	for i, p := range players {
		if p.ID == playerID {
			player = p
			rank = i + 1
			break
		}
	}

	if player == (Player{}) {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	historyDB, err := a.store.PlayerGameHistory(playerID)
	if err != nil {
		http.Error(w, "failed to load player history", http.StatusInternalServerError)
		return
	}

	history := make([]PlayerGameHistoryEntry, len(historyDB))
	for i, h := range historyDB {
		history[i] = PlayerGameHistoryEntry(h)
	}

	rankHistoryDB, err := a.store.PlayerRankHistory(playerID)
	if err != nil {
		http.Error(w, "failed to load player rank history", http.StatusInternalServerError)
		return
	}

	rankHistory := make([]PlayerRankHistoryEntry, len(rankHistoryDB))
	for i, h := range rankHistoryDB {
		rankHistory[i] = PlayerRankHistoryEntry(h)
	}

	gamesDB, err := a.store.PlayerGames(playerID)
	if err != nil {
		http.Error(w, "failed to load player games", http.StatusInternalServerError)
		return
	}

	games := make([]Game, len(gamesDB))
	for i, g := range gamesDB {
		games[i] = Game(g)
	}

	view := newPlayerDetailView(player, rank).
		withGameHistory(history).
		withRankHistory(rankHistory).
		withGames(games).
		withTotalPlayers(len(players))

	renderTemplate(w, "layout", view, "templates/layout.html", "templates/player.html")
}
