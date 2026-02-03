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
	TotalGames   int
	Rank         int
	seasonContext
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
	p.TotalGames = len(games)
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

	ctx, selectedSeason, err := a.loadSeasonContext(r)
	if err != nil {
		http.Error(w, "failed to load seasons", http.StatusInternalServerError)
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

	filter := seasonFilter(selectedSeason)
	players, err := a.Leaderboard(filter)
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
		playerDB, ok, err := a.store.PlayerTotalsByID(playerID, filter)
		if err != nil {
			http.Error(w, "failed to load player", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}
		player = Player(playerDB)
	}

	historyDB, err := a.store.PlayerGameHistory(playerID, filter)
	if err != nil {
		http.Error(w, "failed to load player history", http.StatusInternalServerError)
		return
	}

	history := make([]PlayerGameHistoryEntry, len(historyDB))
	for i, h := range historyDB {
		history[i] = PlayerGameHistoryEntry(h)
	}

	rankHistoryDB, err := a.store.PlayerRankHistory(playerID, filter)
	if err != nil {
		http.Error(w, "failed to load player rank history", http.StatusInternalServerError)
		return
	}

	rankHistory := make([]PlayerRankHistoryEntry, len(rankHistoryDB))
	for i, h := range rankHistoryDB {
		rankHistory[i] = PlayerRankHistoryEntry(h)
	}

	gamesDB, err := a.store.PlayerGames(playerID, filter)
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
	view.seasonContext = ctx

	renderTemplate(w, "layout", view, "templates/layout.html", "templates/player.html")
}
