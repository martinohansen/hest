package main

import (
	"net/http"
	"sort"
)

type PlayerWithRank struct {
	Player
	OriginalRank int
}

type leaderboardForm struct {
	Path    string
	Players []PlayerWithRank
	SortBy  string
	SortDir string
}

func newLeaderboardForm() *leaderboardForm {
	return &leaderboardForm{
		Path: "/",
	}
}

// withPlayers adds players to the leaderboard first player being number 1 and
// so on.
func (l leaderboardForm) withPlayers(leaderboard []Player) leaderboardForm {
	rankedPlayers := make([]PlayerWithRank, len(leaderboard))
	for i, p := range leaderboard {
		rankedPlayers[i] = PlayerWithRank{
			Player:       p,
			OriginalRank: i + 1,
		}
	}
	l.Players = rankedPlayers
	return l
}

func (l leaderboardForm) withSort(sortBy, sortDir string) leaderboardForm {
	l.SortBy = sortBy
	l.SortDir = sortDir

	if sortDir == "" {
		sortDir = "desc"
	}

	// If no sort or default sort (points desc), return as-is
	if sortBy == "" || (sortBy == "points" && sortDir == "desc") {
		return l
	}

	ascending := sortDir == "asc"
	sort.Slice(l.Players, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "games":
			less = l.Players[i].Games < l.Players[j].Games
		case "wins":
			less = l.Players[i].Wins < l.Players[j].Wins
		case "seconds":
			less = l.Players[i].Seconds < l.Players[j].Seconds
		case "points":
			less = l.Players[i].Points < l.Players[j].Points
		case "ppg":
			less = l.Players[i].PPG < l.Players[j].PPG
		default:
			less = l.Players[i].Points < l.Players[j].Points
		}

		if ascending {
			return less
		}
		return !less
	})

	return l
}

func (a *App) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	sortBy := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")

	players, err := a.Leaderboard()
	if err != nil {
		http.Error(w, "loading leaderboard", http.StatusInternalServerError)
		return
	}

	form := newLeaderboardForm().withPlayers(players).withSort(sortBy, sortDir)

	// If HTMX request, return only the table partial
	if r.Header.Get("HX-Request") == "true" {
		renderTemplate(w, "content", form, "templates/leaderboard.html")
		return
	}

	// Otherwise, render the full page
	renderTemplate(w, "layout", form, "templates/layout.html", "templates/leaderboard.html")
}
