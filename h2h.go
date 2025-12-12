package main

import (
	"net/http"
	"strconv"

	"github.com/martinohansen/hest/internal/db"
)

type h2hView struct {
	Path        string
	Title       string
	Players     []db.Player
	Player1ID   int
	Player2ID   int
	Stats       *db.H2HStats
	ShowResults bool
}

func newH2HView() *h2hView {
	return &h2hView{
		Path:  "/h2h",
		Title: "H2H",
	}
}

func (h h2hView) withPlayers(players []db.Player) h2hView {
	h.Players = players
	return h
}

func (h h2hView) withSelection(player1ID, player2ID int) h2hView {
	h.Player1ID = player1ID
	h.Player2ID = player2ID
	return h
}

func (h h2hView) withStats(stats db.H2HStats) h2hView {
	h.Stats = &stats
	h.ShowResults = true
	return h
}

func (a *App) handleH2H(w http.ResponseWriter, r *http.Request) {
	players, err := a.store.ListPlayersByName()
	if err != nil {
		http.Error(w, "loading players", http.StatusInternalServerError)
		return
	}

	view := newH2HView().withPlayers(players)

	player1IDStr := r.URL.Query().Get("player1")
	player2IDStr := r.URL.Query().Get("player2")

	if player1IDStr != "" && player2IDStr != "" {
		player1ID, err1 := strconv.Atoi(player1IDStr)
		player2ID, err2 := strconv.Atoi(player2IDStr)

		if err1 == nil && err2 == nil && player1ID != player2ID {
			stats, err := a.store.GetH2HStats(player1ID, player2ID)
			if err == nil {
				view = view.withSelection(player1ID, player2ID).withStats(stats)
			}
		}
	}

	// If HTMX request, return only the content partial
	if r.Header.Get("HX-Request") == "true" {
		renderTemplate(w, "content", view, "templates/h2h.html")
		return
	}

	// Otherwise, render the full page
	renderTemplate(w, "layout", view, "templates/layout.html", "templates/h2h.html")
}
