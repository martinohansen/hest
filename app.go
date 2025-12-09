package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/martinohansen/hest/internal/db"
)

const dateLayout = "2006-01-02"

type App struct {
	store *db.Store
}

type (
	Player db.Player
	Game   db.Game
)

func newApp(store *db.Store) *App {
	return &App{store: store}
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	staticServer := http.FileServer(http.FS(staticContent))
	mux.Handle("/static/", http.StripPrefix("/static/", staticServer))
	mux.HandleFunc("/", a.handleLeaderboard)
	mux.HandleFunc("/games", a.handleGames)
	mux.HandleFunc("/games/save", a.handleSaveGame)
	mux.HandleFunc("/games/save-and-new", a.handleSaveAndNewGame)
	mux.HandleFunc("/new", a.handleNewGame)
	mux.HandleFunc("/new/score", a.handleScoreGame)
	mux.HandleFunc("/players", a.handleAddPlayer)
	return mux
}

func (a *App) Leaderboard() ([]Player, error) {
	players, err := a.store.ListPlayersByPoints()
	if err != nil {
		return nil, err
	}

	result := make([]Player, len(players))
	for i, p := range players {
		result[i] = Player(p)
	}
	return result, nil
}

func (a *App) ListPlayers() ([]Player, error) {
	players, err := a.store.ListPlayersByName()
	if err != nil {
		return nil, err
	}

	result := make([]Player, len(players))
	for i, p := range players {
		result[i] = Player(p)
	}
	return result, nil
}

func (a *App) listGames() ([]Game, error) {
	games, err := a.store.ListGames()
	if err != nil {
		return nil, err
	}

	result := make([]Game, len(games))
	for i, g := range games {
		result[i] = Game(g)
	}
	return result, nil
}

func (a *App) playersByIDs(ids []int) ([]Player, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	players, err := a.store.PlayersByIDs(ids)
	if err != nil {
		return nil, err
	}
	if len(players) == 0 {
		return nil, nil
	}

	result := make([]Player, len(players))
	for i, p := range players {
		result[i] = Player(p)
	}
	return result, nil
}

func parseIDs(values []string) ([]int, error) {
	var ids []int
	for _, v := range values {
		id, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Return player ID from string or error
func parsePlayer(id string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(id))
}

func validatePlacement(winnerID, secondID int, participantIDs []int) string {
	if winnerID == 0 || secondID == 0 {
		return "Pick a winner and a 2nd place."
	}
	if winnerID == secondID {
		return "Winner and 2nd place must be different players."
	}

	idSet := make(map[int]struct{}, len(participantIDs))
	for _, id := range participantIDs {
		idSet[id] = struct{}{}
	}
	if _, ok := idSet[winnerID]; !ok {
		return "Winner must be part of the game."
	}
	if _, ok := idSet[secondID]; !ok {
		return "2nd place must be part of the game."
	}
	return ""
}

func parsePlayedAt(raw string) (time.Time, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Now(), ""
	}
	playedAt, err := time.Parse(dateLayout, raw)
	if err != nil {
		return time.Time{}, "Invalid date."
	}
	return playedAt, ""
}
