package main

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/martinohansen/hest/internal/db"
)

type gameEditPlayer struct {
	ID       int
	Name     string
	Emoji    string
	Selected bool
}

type gameEditView struct {
	Path     string
	Title    string
	GameID   int
	PlayedAt string
	WinnerID int
	SecondID int
	Players  []gameEditPlayer
	Error    string
	seasonContext
}

func buildGameEditPlayers(allPlayers []Player, participantIDs []int) []gameEditPlayer {
	selected := make(map[int]struct{}, len(participantIDs))
	for _, id := range participantIDs {
		selected[id] = struct{}{}
	}

	players := make([]gameEditPlayer, len(allPlayers))
	for i, p := range allPlayers {
		_, ok := selected[p.ID]
		players[i] = gameEditPlayer{
			ID:       p.ID,
			Name:     p.Name,
			Emoji:    p.Emoji,
			Selected: ok,
		}
	}
	return players
}

func renderGameEdit(w http.ResponseWriter, view gameEditView) {
	renderTemplate(w, "layout", view, "templates/layout.html", "templates/edit.html")
}

func (a *App) handleGameEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := requireAuth(w, r); !ok {
		return
	}

	ctx, _, err := a.loadSeasonContext(r)
	if err != nil {
		http.Error(w, "failed to load seasons", http.StatusInternalServerError)
		return
	}

	gameID, err := parseGameID(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "invalid game id", http.StatusBadRequest)
		return
	}

	game, ok, err := a.store.GetGameByID(gameID)
	if err != nil {
		http.Error(w, "failed to load game", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	allPlayers, err := a.listPlayersBase()
	if err != nil {
		http.Error(w, "failed to load players", http.StatusInternalServerError)
		return
	}

	participantIDs := make([]int, 0, len(game.Participants))
	for _, p := range game.Participants {
		participantIDs = append(participantIDs, p.ID)
	}

	view := gameEditView{
		Path:          "/game",
		Title:         "Rediger kamp #" + strconv.Itoa(gameID),
		GameID:        gameID,
		PlayedAt:      game.PlayedAt.Format(dateLayout),
		WinnerID:      game.Winner.ID,
		SecondID:      game.Second.ID,
		Players:       buildGameEditPlayers(allPlayers, participantIDs),
		Error:         "",
		seasonContext: ctx,
	}

	renderGameEdit(w, view)
}

func (a *App) handleUpdateGame(w http.ResponseWriter, r *http.Request) {
	_, ok := ensureAuthAndForm(w, r)
	if !ok {
		return
	}

	ctx, _, err := a.loadSeasonContext(r)
	if err != nil {
		http.Error(w, "failed to load seasons", http.StatusInternalServerError)
		return
	}

	gameID, err := parseGameID(r.FormValue("game_id"))
	if err != nil {
		http.Error(w, "invalid game id", http.StatusBadRequest)
		return
	}

	_, found, err := a.store.GetGameByID(gameID)
	if err != nil {
		http.Error(w, "failed to load game", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	allPlayers, err := a.listPlayersBase()
	if err != nil {
		http.Error(w, "failed to load players", http.StatusInternalServerError)
		return
	}

	ids, err := parseIDs(r.Form["player_id"])
	if err != nil {
		http.Error(w, "bad player selection", http.StatusBadRequest)
		return
	}
	uniqueIDs := db.Dedupe(ids)
	playedAt := r.FormValue("played_at")
	winnerGuess, _ := parsePlayer(r.FormValue("winner_id"))
	secondGuess, _ := parsePlayer(r.FormValue("second_id"))

	if len(uniqueIDs) < 2 {
		view := gameEditView{
			Path:          "/game",
			Title:         "Rediger kamp #" + strconv.Itoa(gameID),
			GameID:        gameID,
			PlayedAt:      playedAt,
			WinnerID:      winnerGuess,
			SecondID:      secondGuess,
			Players:       buildGameEditPlayers(allPlayers, uniqueIDs),
			Error:         "Pick at least two players.",
			seasonContext: ctx,
		}
		renderGameEdit(w, view)
		return
	}

	winnerID, err := parsePlayer(r.FormValue("winner_id"))
	if err != nil {
		view := gameEditView{
			Path:          "/game",
			Title:         "Rediger kamp #" + strconv.Itoa(gameID),
			GameID:        gameID,
			PlayedAt:      playedAt,
			WinnerID:      0,
			SecondID:      0,
			Players:       buildGameEditPlayers(allPlayers, uniqueIDs),
			Error:         "Pick a winner.",
			seasonContext: ctx,
		}
		renderGameEdit(w, view)
		return
	}

	secondID, err := parsePlayer(r.FormValue("second_id"))
	if err != nil {
		view := gameEditView{
			Path:          "/game",
			Title:         "Rediger kamp #" + strconv.Itoa(gameID),
			GameID:        gameID,
			PlayedAt:      playedAt,
			WinnerID:      winnerID,
			SecondID:      0,
			Players:       buildGameEditPlayers(allPlayers, uniqueIDs),
			Error:         "Pick a 2nd place.",
			seasonContext: ctx,
		}
		renderGameEdit(w, view)
		return
	}

	view := gameEditView{
		Path:          "/game",
		Title:         "Rediger kamp #" + strconv.Itoa(gameID),
		GameID:        gameID,
		PlayedAt:      playedAt,
		WinnerID:      winnerID,
		SecondID:      secondID,
		Players:       buildGameEditPlayers(allPlayers, uniqueIDs),
		seasonContext: ctx,
	}

	selectedPlayers, err := a.playersByIDs(uniqueIDs)
	if err != nil {
		http.Error(w, "failed to load selected players", http.StatusInternalServerError)
		return
	}
	if len(selectedPlayers) < len(uniqueIDs) {
		http.Error(w, "unknown player selected", http.StatusBadRequest)
		return
	}

	if msg := validatePlacement(winnerID, secondID, uniqueIDs); msg != "" {
		view.Error = msg
		renderGameEdit(w, view)
		return
	}

	parsedPlayedAt, msg := parsePlayedAt(playedAt)
	if msg != "" {
		view.Error = msg
		renderGameEdit(w, view)
		return
	}

	if err := a.store.UpdateGame(gameID, parsedPlayedAt, uniqueIDs, winnerID, secondID); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	redirectPath := "/game?id=" + strconv.Itoa(gameID)
	if season, explicit := seasonParamFromRequest(r); explicit {
		redirectPath += "&season=" + url.QueryEscape(season)
	}

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", redirectPath)
		return
	}

	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
}
