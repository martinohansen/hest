package main

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/martinohansen/hest/internal/db"
)

type gameForm struct {
	Path     string
	Title    string
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
		Title:    "Tilføj kamp",
		Players:  players,
		PlayedAt: time.Now().Format(dateLayout),
	}
	return form
}

func (f gameForm) withError(msg string) gameForm {
	f.Error = msg
	return f
}

func (f gameForm) withSuccess(msg string) gameForm {
	f.Success = msg

	// Clear selections on success
	f.WinnerID = 0
	f.SecondID = 0
	return f
}

func (f gameForm) withSelection(winnerID, secondID int) gameForm {
	f.WinnerID = winnerID
	f.SecondID = secondID
	return f
}

func (f gameForm) withDate(playedAt string) gameForm {
	f.PlayedAt = strings.TrimSpace(playedAt)
	return f
}

func (a *App) handleAddPlayer(w http.ResponseWriter, r *http.Request) {
	_, ok := ensureAuthAndForm(w, r)
	if !ok {
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	if err := a.store.AddPlayer(name); err != nil {
		slog.Error("could not add player", "error", err)
		http.Error(w, "could not add player", http.StatusBadRequest)
		return
	}

	players, err := a.Leaderboard()
	if err != nil {
		http.Error(w, "failed to reload players", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "player_list", struct{ Players []Player }{Players: players}, "templates/new.html")
}

func (a *App) handleNewGame(w http.ResponseWriter, r *http.Request) {
	players, err := a.ListPlayers()
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
		players, listErr := a.Leaderboard()
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
	username, ok := ensureAuthAndForm(w, r)
	if !ok {
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

	winnerID, err := parsePlayer(r.FormValue("winner_id"))
	if err != nil {
		a.renderScoring(w, r, form.withError("Pick a winner."))
		return nil, false
	}
	secondID, err := parsePlayer(r.FormValue("second_id"))
	if err != nil {
		a.renderScoring(w, r, form.withSelection(winnerID, secondID).withError("a 2nd place."))
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

func (a *App) renderSelection(w http.ResponseWriter, partial bool, form gameForm) {
	if partial {
		renderTemplate(w, "new", form, "templates/new.html")
		return
	}
	renderTemplate(w, "layout", form, "templates/layout.html", "templates/new.html")
}

func (a *App) renderScoring(w http.ResponseWriter, r *http.Request, form gameForm) {
	if r.Header.Get("HX-Request") != "" {
		renderTemplate(w, "score", form, "templates/new.html")
		return
	}
	renderTemplate(w, "layout", form, "templates/layout.html", "templates/new.html")
}
