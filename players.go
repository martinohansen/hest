package main

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

type playersView struct {
	Path    string
	Title   string
	Players []Player
	seasonContext
}

func (a *App) handlePlayers(w http.ResponseWriter, r *http.Request) {
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

	players, err := a.listPlayersBase()
	if err != nil {
		http.Error(w, "failed to load players", http.StatusInternalServerError)
		return
	}

	view := playersView{
		Path:    "/players",
		Title:   "Spillere",
		Players: players,
	}
	view.seasonContext = ctx

	renderTemplate(w, "layout", view, "templates/layout.html", "templates/players.html")
}

func (a *App) handleUpdatePlayer(w http.ResponseWriter, r *http.Request) {
	_, ok := a.ensureAuthAndForm(w, r)
	if !ok {
		return
	}

	ids := r.Form["id"]
	names := r.Form["name"]
	emojis := r.Form["emoji"]
	easterEggsList := r.Form["easter_eggs"]

	if len(ids) != len(names) || len(ids) != len(emojis) || len(ids) != len(easterEggsList) {
		http.Error(w, "form data mismatch", http.StatusBadRequest)
		return
	}

	for i, idStr := range ids {
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil || id <= 0 {
			http.Error(w, "invalid player id", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(names[i])
		if name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}

		emoji := strings.TrimSpace(emojis[i])
		easterEggs := strings.TrimSpace(easterEggsList[i])

		if err := a.store.UpdatePlayer(id, name, emoji, easterEggs); err != nil {
			slog.Error("could not update player", "error", err)
			http.Error(w, "could not update player", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/players", http.StatusSeeOther)
}
