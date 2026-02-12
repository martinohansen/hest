package main

import (
	"time"

	"github.com/martinohansen/hest/internal/db"
)

// calculateRankChanges compares current leaderboard with leaderboard from 2
// games ago and returns a map of player ID to rank change (positive = moved up,
// negative = moved down)
func (a *App) calculateRankChanges(filter *db.DateRange) (map[int]int, error) {
	// Get all games in the date range
	games, err := a.store.ListGames(filter)
	if err != nil {
		return nil, err
	}

	// If we have less than 2 games, no changes to show
	if len(games) < 2 {
		return make(map[int]int), nil
	}

	// Get the played_at time of the 2nd most recent game
	// games are ordered DESC, so games[1] is the 2nd most recent
	cutoffTime := games[1].PlayedAt

	// Create a filter that excludes the last 2 games
	var oldFilter *db.DateRange
	if filter != nil && filter.Valid() {
		oldFilter = &db.DateRange{
			StartDate: filter.StartDate,
			EndDate:   cutoffTime.Add(-1 * time.Second).Format(dateLayout),
		}
	} else {
		// No filter, so get everything before the cutoff
		oldFilter = &db.DateRange{
			StartDate: "1970-01-01",
			EndDate:   cutoffTime.Add(-1 * time.Second).Format(dateLayout),
		}
	}

	// Get old leaderboard (before last 2 games)
	oldLeaderboard, err := a.store.ListPlayersByPoints(oldFilter)
	if err != nil {
		return nil, err
	}

	// Get current leaderboard
	currentLeaderboard, err := a.store.ListPlayersByPoints(filter)
	if err != nil {
		return nil, err
	}

	// Build a map of player ID to old rank
	oldRanks := make(map[int]int)
	for i, p := range oldLeaderboard {
		oldRanks[p.ID] = i + 1
	}

	// Calculate rank changes
	rankChanges := make(map[int]int)
	for i, p := range currentLeaderboard {
		currentRank := i + 1
		oldRank, existed := oldRanks[p.ID]
		if !existed {
			// Player is new (didn't exist in old leaderboard)
			rankChanges[p.ID] = 0
		} else {
			// Positive change means moved up (lower rank number)
			rankChanges[p.ID] = oldRank - currentRank
		}
	}

	return rankChanges, nil
}
