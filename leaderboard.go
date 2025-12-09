package main

import "net/http"

type leaderboardForm struct {
	Path    string
	Players []Player
}

func newLeaderboardForm() *leaderboardForm {
	return &leaderboardForm{
		Path: "/",
	}
}

func (l leaderboardForm) withPlayers(players []Player) leaderboardForm {
	l.Players = players
	return l
}

func (a *App) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	players, err := a.Leaderboard()
	if err != nil {
		http.Error(w, "loading leaderboard", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "layout", newLeaderboardForm().withPlayers(players), "templates/layout.html", "templates/leaderboard.html")
}
