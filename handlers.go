package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/martinohansen/hest/internal/db"
)

const dateLayout = "2006-01-02"

func (a *App) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	players, err := a.listPlayers()
	if err != nil {
		http.Error(w, "loading leaderboard", http.StatusInternalServerError)
		return
	}

	page := struct {
		Path    string
		Players []Player
	}{
		Path:    "/",
		Players: players,
	}
	a.renderTemplate(w, "layout", page, "templates/layout.html", "templates/leaderboard.html")
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

	page := struct {
		Path  string
		Games []Game
	}{
		Path:  "/games",
		Games: games,
	}
	a.renderTemplate(w, "layout", page, "templates/layout.html", "templates/games.html")
}

func (a *App) handleAddPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, ok := a.requireAuth(w, r); !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	if err := a.store.AddPlayer(name); err != nil {
		http.Error(w, "could not add player (maybe duplicate?)", http.StatusBadRequest)
		return
	}

	players, err := a.listPlayers()
	if err != nil {
		http.Error(w, "failed to reload players", http.StatusInternalServerError)
		return
	}

	a.renderTemplate(w, "player_list", struct{ Players []Player }{Players: players}, "templates/new.html")
}

func (a *App) handleNewGame(w http.ResponseWriter, r *http.Request) {
	players, err := a.listPlayers()
	if err != nil {
		http.Error(w, "failed to load players", http.StatusInternalServerError)
		return
	}

	partial := r.URL.Query().Get("partial") == "1"
	a.renderSelection(w, partial, newGameForm(players))
}

func (a *App) handleScoreGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	ids, err := parseIDs(r.Form["player_id"])
	if err != nil {
		http.Error(w, "bad player selection", http.StatusBadRequest)
		return
	}
	uniqueIDs := db.Dedupe(ids)
	if len(uniqueIDs) < 2 {
		players, listErr := a.listPlayers()
		if listErr != nil {
			http.Error(w, "pick at least two players", http.StatusBadRequest)
			return
		}
		a.renderSelection(w, true, newGameForm(players).withError("Pick at least two players."))
		return
	}

	players, err := a.playersByIDs(uniqueIDs)
	if err != nil {
		http.Error(w, "failed to load selected players", http.StatusInternalServerError)
		return
	}
	if len(players) < len(uniqueIDs) {
		http.Error(w, "unknown player selected", http.StatusBadRequest)
		return
	}

	a.renderScoring(w, r, newGameForm(players))
}

func (a *App) handleSaveGame(w http.ResponseWriter, r *http.Request) {
	_, ok := a.saveGameCommon(w, r)
	if !ok {
		return
	}

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", "/")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleSaveAndNewGame(w http.ResponseWriter, r *http.Request) {
	players, ok := a.saveGameCommon(w, r)
	if !ok {
		return
	}

	// Reset the form with the same players for a new game
	a.renderScoring(w, r, newGameForm(players).withSuccess("Kamp tilføjet"))
}

func (a *App) saveGameCommon(w http.ResponseWriter, r *http.Request) ([]Player, bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return nil, false
	}

	username, ok := a.requireAuth(w, r)
	if !ok {
		return nil, false
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return nil, false
	}

	ids, err := parseIDs(r.Form["player_id"])
	if err != nil {
		http.Error(w, "bad player selection", http.StatusBadRequest)
		return nil, false
	}

	uniqueIDs := db.Dedupe(ids)
	if len(uniqueIDs) < 2 {
		http.Error(w, "pick at least two players", http.StatusBadRequest)
		return nil, false
	}

	players, err := a.playersByIDs(uniqueIDs)
	if err != nil {
		http.Error(w, "failed to load players", http.StatusInternalServerError)
		return nil, false
	}
	if len(players) < len(uniqueIDs) {
		http.Error(w, "unknown player selected", http.StatusBadRequest)
		return nil, false
	}

	form := newGameForm(players).withDate(r.FormValue("played_at"))

	winnerID, err := parseRequiredID(r.FormValue("winner_id"))
	if err != nil {
		a.renderScoring(w, r, form.withError("Pick a winner."))
		return nil, false
	}
	secondID, err := parseRequiredID(r.FormValue("second_id"))
	if err != nil {
		a.renderScoring(w, r, form.withSelection(winnerID, secondID).withError("Pick a winner and a 2nd place."))
		return nil, false
	}

	form = form.withSelection(winnerID, secondID)
	if msg := validatePlacement(winnerID, secondID, uniqueIDs); msg != "" {
		a.renderScoring(w, r, form.withError(msg))
		return nil, false
	}

	playedAt, msg := parsePlayedAt(form.PlayedAt)
	if msg != "" {
		a.renderScoring(w, r, form.withError(msg))
		return nil, false
	}

	if err := a.store.AddGame(playedAt, uniqueIDs, winnerID, secondID, username); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return nil, false
	}

	return players, true
}

type gameForm struct {
	Path     string
	Players  []Player
	PlayedAt string
	Error    string
	Success  string
	WinnerID int
	SecondID int
}

func newGameForm(players []Player) gameForm {
	form := gameForm{
		Path:     "/new",
		Players:  players,
		PlayedAt: time.Now().Format(dateLayout),
	}
	form.fillDefaults()
	return form
}

func (f *gameForm) fillDefaults() {
	if f.PlayedAt == "" {
		f.PlayedAt = time.Now().Format(dateLayout)
	}
	if f.Path == "" {
		f.Path = "/new"
	}
}

func (f gameForm) withError(msg string) gameForm {
	f.Error = msg
	f.fillDefaults()
	return f
}

func (f gameForm) withSelection(winnerID, secondID int) gameForm {
	f.WinnerID = winnerID
	f.SecondID = secondID
	f.fillDefaults()
	return f
}

func (f gameForm) withDate(playedAt string) gameForm {
	f.PlayedAt = strings.TrimSpace(playedAt)
	f.fillDefaults()
	return f
}

func (f gameForm) withSuccess(msg string) gameForm {
	f.Success = msg
	f.WinnerID = 0
	f.SecondID = 0
	f.fillDefaults()
	return f
}

// Rendering helpers for the multi-step flow.
func (a *App) renderSelection(w http.ResponseWriter, partial bool, form gameForm) {
	form.fillDefaults()
	if partial {
		a.renderTemplate(w, "new", form, "templates/new.html")
		return
	}
	a.renderTemplate(w, "layout", form, "templates/layout.html", "templates/new.html")
}

func (a *App) renderScoring(w http.ResponseWriter, r *http.Request, form gameForm) {
	form.fillDefaults()
	if r.Header.Get("HX-Request") != "" {
		a.renderTemplate(w, "score", form, "templates/new.html")
		return
	}
	a.renderTemplate(w, "layout", form, "templates/layout.html", "templates/new.html")
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

func parseRequiredID(value string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(value))
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
