package main

import "github.com/martinohansen/hest/internal/db"

// Player wraps db.Player with presentation-layer fields.
type Player struct {
	db.Player
}

// toPlayer converts a db.Player to a presentation Player with emoji.
func toPlayer(p db.Player) Player {
	return Player{Player: p}
}

// toPlayers converts db.Players to presentation Players with emojis.
func toPlayers(players []db.Player) []Player {
	out := make([]Player, len(players))
	for i, p := range players {
		out[i] = toPlayer(p)
	}
	return out
}
