package main

import "github.com/martinohansen/hest/internal/db"

// toGame converts a db.Game to a presentation Game using stored emojis.
func toGame(g db.Game) Game {
	result := Game{
		ID:           g.ID,
		PlayedAt:     g.PlayedAt,
		Winner:       toPlayer(g.Winner),
		Second:       toPlayer(g.Second),
		Participants: toPlayers(g.Participants),
		CreatedBy:    g.CreatedBy,
	}
	result.Participants = reorderParticipants(result)
	return result
}

// toGames converts db.Games to presentation Games.
func toGames(games []db.Game) []Game {
	out := make([]Game, len(games))
	for i, g := range games {
		out[i] = toGame(g)
	}
	return out
}
