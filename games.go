package main

import (
	"net/http"
	"time"
)

type gameDisplay struct {
	PlayedAt time.Time
	Winner   Player
	Second   Player
	Others   []Player
}

type gamesView struct {
	Path  string
	Games []gameDisplay
}

func newGameView() gamesView {
	return gamesView{
		Path: "/games",
	}
}

func (g gamesView) withGames(games []Game) gamesView {
	displays := make([]gameDisplay, len(games))
	for i, game := range games {
		display := gameDisplay{
			PlayedAt: game.PlayedAt,
			Winner:   Player(game.Winner),
			Second:   Player(game.Second),
		}
		// Add others (participants who are not winner or second)
		for _, p := range game.Participants {
			if p.ID != game.Winner.ID && p.ID != game.Second.ID {
				display.Others = append(display.Others, Player(p))
			}
		}
		displays[i] = display
	}
	g.Games = displays
	return g
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

	page := newGameView().withGames(games)
	renderTemplate(w, "layout", page, "templates/layout.html", "templates/games.html")
}
