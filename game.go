package main

import "time"

// Game wraps db.Game with presentation-layer player data.
type Game struct {
	ID           int
	PlayedAt     time.Time
	Winner       Player
	Second       Player
	Participants []Player
	CreatedBy    string
}

func reorderParticipants(g Game) []Player {
	var ordered []Player
	if g.Winner.ID != 0 {
		ordered = append(ordered, g.Winner)
	}
	if g.Second.ID != 0 && g.Second.ID != g.Winner.ID {
		ordered = append(ordered, g.Second)
	}

	for _, p := range g.Participants {
		if p.ID == g.Winner.ID || p.ID == g.Second.ID {
			continue
		}
		ordered = append(ordered, p)
	}
	return ordered
}
