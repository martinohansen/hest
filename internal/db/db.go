package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type Player struct {
	ID      int
	Name    string
	Emoji   string
	Games   int
	Wins    int
	Seconds int
	Points  int
	PPG     float64
}

type Game struct {
	ID           int
	PlayedAt     time.Time
	Winner       Player
	Second       Player
	Participants []Player
	CreatedBy    string
}

type PlayerGameHistoryEntry struct {
	PlayedAt     time.Time
	PointsEarned int
	TotalPoints  int
	GamesPlayed  int
	PPG          float64
}

type PlayerRankHistoryEntry struct {
	PlayedAt time.Time
	Rank     int
}

type H2HStats struct {
	Player1       Player
	Player2       Player
	SharedGames   int
	Player1Stats  Player
	Player2Stats  Player
	SharedGamesList []Game
}

func Open(path string) (*Store, error) {
	database, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		return nil, err
	}

	if err := ensureSchema(database); err != nil {
		database.Close()
		return nil, err
	}

	store := &Store{db: database}

	return store, nil
}

func ensureSchema(db *sql.DB) error {
	if err := createTables(db); err != nil {
		return err
	}
	return nil
}

func createTables(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS players (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	emoji TEXT NOT NULL DEFAULT '',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS games (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	played_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	winner_id INTEGER NOT NULL REFERENCES players(id),
	second_id INTEGER NOT NULL REFERENCES players(id),
	created_by TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	CHECK (winner_id != second_id)
);

CREATE TABLE IF NOT EXISTS game_players (
	game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
	player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
	PRIMARY KEY (game_id, player_id)
);`
	_, err := db.Exec(schema)
	return err
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// buildPlaceholders creates SQL placeholder strings and argument slices for IN queries.
func buildPlaceholders(ids []int) (string, []any) {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	return strings.Join(placeholders, ","), args
}

// scanPlayer scans a player row from the playerTotalsQuery result.
func scanPlayer(scanner interface{ Scan(...any) error }) (Player, error) {
	var p Player
	err := scanner.Scan(&p.ID, &p.Name, &p.Emoji, &p.Games, &p.Wins, &p.Seconds, &p.Points, &p.PPG)
	return p, err
}

func (s *Store) AddPlayer(name string) error {
	_, err := s.db.Exec(`INSERT INTO players (name, emoji) VALUES (?, ?)`, name, emoji(name))
	return err
}

func (s *Store) ListGames() ([]Game, error) {
	rows, err := s.db.Query(`
SELECT g.id, g.played_at,
	g.winner_id, winner.name, winner.emoji,
	g.second_id, second.name, second.emoji,
	COALESCE(g.created_by, '')
FROM games g
JOIN players winner ON winner.id = g.winner_id
JOIN players second ON second.id = g.second_id
ORDER BY g.played_at DESC, g.id DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []Game
	var gameIDs []int
	for rows.Next() {
		var (
			g        Game
			winnerID int
			secondID int
			wEmoji   string
			sEmoji   string
		)
		if err := rows.Scan(&g.ID, &g.PlayedAt, &winnerID, &g.Winner.Name, &wEmoji, &secondID, &g.Second.Name, &sEmoji, &g.CreatedBy); err != nil {
			return nil, err
		}
		g.Winner.ID = winnerID
		g.Winner.Emoji = wEmoji
		g.Second.ID = secondID
		g.Second.Emoji = sEmoji
		games = append(games, g)
		gameIDs = append(gameIDs, g.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(games) == 0 {
		return games, nil
	}

	participantMap, err := s.loadGameParticipants(gameIDs)
	if err != nil {
		return nil, err
	}

	for i, g := range games {
		g.Participants = participantMap[g.ID]
		games[i] = g
	}

	return games, nil
}

// loadGameParticipants fetches all participants for the given game IDs.
func (s *Store) loadGameParticipants(gameIDs []int) (map[int][]Player, error) {
	placeholders, args := buildPlaceholders(gameIDs)
	rows, err := s.db.Query(fmt.Sprintf(`
SELECT gp.game_id, p.id, p.name, p.emoji
FROM game_players gp
JOIN players p ON p.id = gp.player_id
WHERE gp.game_id IN (%s)
ORDER BY gp.game_id, p.name
`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	participantMap := make(map[int][]Player, len(gameIDs))
	for rows.Next() {
		var (
			gameID int
			p      Player
		)
		if err := rows.Scan(&gameID, &p.ID, &p.Name, &p.Emoji); err != nil {
			return nil, err
		}
		participantMap[gameID] = append(participantMap[gameID], p)
	}
	return participantMap, rows.Err()
}

func (s *Store) PlayerGameHistory(playerID int) ([]PlayerGameHistoryEntry, error) {
	rows, err := s.db.Query(`
WITH player_games AS (
	SELECT
		g.id,
		g.played_at,
		CASE
			WHEN g.winner_id = ? THEN 3
			WHEN g.second_id = ? THEN 1
			ELSE 0
		END as points_earned
	FROM games g
	JOIN game_players gp ON g.id = gp.game_id
	WHERE gp.player_id = ?
	ORDER BY g.played_at ASC, g.id ASC
)
SELECT
	played_at,
	points_earned,
	SUM(points_earned) OVER (ORDER BY played_at, id) as total_points,
	ROW_NUMBER() OVER (ORDER BY played_at, id) as games_played,
	CAST(SUM(points_earned) OVER (ORDER BY played_at, id) AS REAL) /
		ROW_NUMBER() OVER (ORDER BY played_at, id) as ppg
FROM player_games
ORDER BY played_at ASC, id ASC
`, playerID, playerID, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []PlayerGameHistoryEntry
	for rows.Next() {
		var entry PlayerGameHistoryEntry
		if err := rows.Scan(&entry.PlayedAt, &entry.PointsEarned, &entry.TotalPoints, &entry.GamesPlayed, &entry.PPG); err != nil {
			return nil, err
		}
		history = append(history, entry)
	}
	return history, rows.Err()
}

func (s *Store) PlayerRankHistory(playerID int) ([]PlayerRankHistoryEntry, error) {
	rows, err := s.db.Query(`
WITH player_games AS (
	-- Get all games where target player participated
	SELECT DISTINCT g.id, g.played_at
	FROM games g
	JOIN game_players gp ON g.id = gp.game_id
	WHERE gp.player_id = ?
	ORDER BY g.played_at ASC, g.id ASC
),
leaderboard_snapshots AS (
	-- For each game, calculate ALL players' stats up to that point
	SELECT
		pg.played_at,
		pg.id as game_id,
		p.id as player_id,
		COUNT(DISTINCT g_hist.id) as games,
		COUNT(DISTINCT CASE WHEN g_hist.winner_id = p.id THEN g_hist.id END) as wins,
		COUNT(DISTINCT CASE WHEN g_hist.second_id = p.id THEN g_hist.id END) as seconds,
		(COUNT(DISTINCT CASE WHEN g_hist.winner_id = p.id THEN g_hist.id END) * 3 +
		 COUNT(DISTINCT CASE WHEN g_hist.second_id = p.id THEN g_hist.id END)) as points
	FROM player_games pg
	CROSS JOIN players p
	LEFT JOIN game_players gp_hist ON gp_hist.player_id = p.id
	LEFT JOIN games g_hist ON g_hist.id = gp_hist.game_id
		AND (g_hist.played_at < pg.played_at
		     OR (g_hist.played_at = pg.played_at AND g_hist.id <= pg.id))
	GROUP BY pg.played_at, pg.id, p.id
),
ranked_leaderboard AS (
	-- Apply ranking with proper tiebreakers
	SELECT
		played_at,
		player_id,
		ROW_NUMBER() OVER (
			PARTITION BY played_at, game_id
			ORDER BY points DESC, wins DESC, seconds DESC, games DESC, player_id ASC
		) as rank
	FROM leaderboard_snapshots
)
SELECT played_at, rank
FROM ranked_leaderboard
WHERE player_id = ?
ORDER BY played_at ASC
`, playerID, playerID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []PlayerRankHistoryEntry
	for rows.Next() {
		var entry PlayerRankHistoryEntry
		if err := rows.Scan(&entry.PlayedAt, &entry.Rank); err != nil {
			return nil, err
		}
		history = append(history, entry)
	}
	return history, rows.Err()
}

func (s *Store) PlayerGames(playerID int) ([]Game, error) {
	rows, err := s.db.Query(`
SELECT g.id, g.played_at,
	g.winner_id, winner.name, winner.emoji,
	g.second_id, second.name, second.emoji,
	COALESCE(g.created_by, '')
FROM games g
JOIN players winner ON winner.id = g.winner_id
JOIN players second ON second.id = g.second_id
JOIN game_players gp ON g.id = gp.game_id
WHERE gp.player_id = ?
ORDER BY g.played_at DESC, g.id DESC
`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []Game
	var gameIDs []int
	for rows.Next() {
		var (
			g        Game
			winnerID int
			secondID int
			wEmoji   string
			sEmoji   string
		)
		if err := rows.Scan(&g.ID, &g.PlayedAt, &winnerID, &g.Winner.Name, &wEmoji, &secondID, &g.Second.Name, &sEmoji, &g.CreatedBy); err != nil {
			return nil, err
		}
		g.Winner.ID = winnerID
		g.Winner.Emoji = wEmoji
		g.Second.ID = secondID
		g.Second.Emoji = sEmoji
		games = append(games, g)
		gameIDs = append(gameIDs, g.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(games) > 0 {
		participantMap, err := s.loadGameParticipants(gameIDs)
		if err != nil {
			return nil, err
		}

		for i, g := range games {
			g.Participants = participantMap[g.ID]
			games[i] = g
		}
	}

	return games, nil
}

func (s *Store) ListPlayersByName() ([]Player, error) {
	rows, err := s.db.Query(playerTotalsQuery("", `ORDER BY name ASC`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []Player
	for rows.Next() {
		p, err := scanPlayer(rows)
		if err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

// ListPlayersByPoints returns all players ordered by their points with
// tiebreakers, in order of wins, seconds, games played, and lastly name.
func (s *Store) ListPlayersByPoints() ([]Player, error) {
	rows, err := s.db.Query(playerTotalsQuery("", `
ORDER BY points DESC, wins DESC, seconds DESC, games DESC, name ASC
`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []Player
	for rows.Next() {
		p, err := scanPlayer(rows)
		if err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

func (s *Store) PlayersByIDs(ids []int) ([]Player, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders, args := buildPlaceholders(ids)
	query := playerTotalsQuery(fmt.Sprintf("WHERE p.id IN (%s)", placeholders), "")

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[int]Player)
	for rows.Next() {
		p, err := scanPlayer(rows)
		if err != nil {
			return nil, err
		}
		byID[p.ID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ordered := make([]Player, 0, len(ids))
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			ordered = append(ordered, p)
		}
	}
	return ordered, nil
}

// Dedupe returns a new slice with duplicate values removed, preserving order.
func Dedupe(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

// validateGameParticipants checks that the game has valid participants and placements.
func validateGameParticipants(participantIDs []int, winnerID, secondID int) error {
	if len(participantIDs) == 0 {
		return fmt.Errorf("no participants")
	}

	participantSet := make(map[int]struct{}, len(participantIDs))
	for _, id := range participantIDs {
		participantSet[id] = struct{}{}
	}

	if _, ok := participantSet[winnerID]; !ok {
		return fmt.Errorf("winner must be a participant")
	}
	if _, ok := participantSet[secondID]; !ok {
		return fmt.Errorf("second place must be a participant")
	}
	return nil
}

func (s *Store) AddGame(playedAt time.Time, participantIDs []int, winnerID, secondID int, createdBy string) error {
	uniqueIDs := Dedupe(participantIDs)
	if err := validateGameParticipants(uniqueIDs, winnerID, secondID); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	res, err := tx.Exec(`INSERT INTO games (played_at, winner_id, second_id, created_by) VALUES (?, ?, ?, ?)`, playedAt, winnerID, secondID, createdBy)
	if err != nil {
		return err
	}

	gameID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO game_players (game_id, player_id) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, pid := range uniqueIDs {
		if _, err = stmt.Exec(gameID, pid); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

// GetH2HStats returns head-to-head statistics for two players, including only games where both participated.
func (s *Store) GetH2HStats(player1ID, player2ID int) (H2HStats, error) {
	var stats H2HStats

	// Get base player info
	players, err := s.PlayersByIDs([]int{player1ID, player2ID})
	if err != nil {
		return stats, err
	}
	if len(players) != 2 {
		return stats, fmt.Errorf("players not found")
	}
	stats.Player1 = players[0]
	stats.Player2 = players[1]

	// Get games where both players participated
	rows, err := s.db.Query(`
SELECT g.id, g.played_at,
	g.winner_id, winner.name, winner.emoji,
	g.second_id, second.name, second.emoji,
	COALESCE(g.created_by, '')
FROM games g
JOIN players winner ON winner.id = g.winner_id
JOIN players second ON second.id = g.second_id
JOIN game_players gp1 ON g.id = gp1.game_id AND gp1.player_id = ?
JOIN game_players gp2 ON g.id = gp2.game_id AND gp2.player_id = ?
ORDER BY g.played_at DESC, g.id DESC
`, player1ID, player2ID)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	var games []Game
	var gameIDs []int
	for rows.Next() {
		var (
			g        Game
			winnerID int
			secondID int
			wEmoji   string
			sEmoji   string
		)
		if err := rows.Scan(&g.ID, &g.PlayedAt, &winnerID, &g.Winner.Name, &wEmoji, &secondID, &g.Second.Name, &sEmoji, &g.CreatedBy); err != nil {
			return stats, err
		}
		g.Winner.ID = winnerID
		g.Winner.Emoji = wEmoji
		g.Second.ID = secondID
		g.Second.Emoji = sEmoji
		games = append(games, g)
		gameIDs = append(gameIDs, g.ID)
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	stats.SharedGames = len(games)

	if len(games) > 0 {
		participantMap, err := s.loadGameParticipants(gameIDs)
		if err != nil {
			return stats, err
		}

		for i, g := range games {
			g.Participants = participantMap[g.ID]
			games[i] = g
		}
	}

	stats.SharedGamesList = games

	// Calculate stats for player1 in shared games
	var p1Games, p1Wins, p1Seconds int
	for _, g := range games {
		p1Games++
		if g.Winner.ID == player1ID {
			p1Wins++
		} else if g.Second.ID == player1ID {
			p1Seconds++
		}
	}
	p1Points := p1Wins*3 + p1Seconds
	var p1PPG float64
	if p1Games > 0 {
		p1PPG = float64(p1Points) / float64(p1Games)
	}
	stats.Player1Stats = Player{
		ID:      player1ID,
		Name:    stats.Player1.Name,
		Emoji:   stats.Player1.Emoji,
		Games:   p1Games,
		Wins:    p1Wins,
		Seconds: p1Seconds,
		Points:  p1Points,
		PPG:     p1PPG,
	}

	// Calculate stats for player2 in shared games
	var p2Games, p2Wins, p2Seconds int
	for _, g := range games {
		p2Games++
		if g.Winner.ID == player2ID {
			p2Wins++
		} else if g.Second.ID == player2ID {
			p2Seconds++
		}
	}
	p2Points := p2Wins*3 + p2Seconds
	var p2PPG float64
	if p2Games > 0 {
		p2PPG = float64(p2Points) / float64(p2Games)
	}
	stats.Player2Stats = Player{
		ID:      player2ID,
		Name:    stats.Player2.Name,
		Emoji:   stats.Player2.Emoji,
		Games:   p2Games,
		Wins:    p2Wins,
		Seconds: p2Seconds,
		Points:  p2Points,
		PPG:     p2PPG,
	}

	return stats, nil
}

// playerTotalsQuery builds a query to fetch player statistics.
// The where parameter should include the WHERE keyword if needed (e.g., "WHERE p.id IN (...)").
// The order parameter should include the ORDER BY keyword if needed.
// Scoring: 3 points per win, 1 point per second place.
func playerTotalsQuery(where, order string) string {
	var b strings.Builder
	b.WriteString(`
WITH games_count AS (
    SELECT player_id, COUNT(*) AS games
    FROM game_players
    GROUP BY player_id
),
wins_count AS (
    SELECT winner_id AS player_id, COUNT(*) AS wins
    FROM games
    GROUP BY winner_id
),
seconds_count AS (
    SELECT second_id AS player_id, COUNT(*) AS seconds
    FROM games
    GROUP BY second_id
)
SELECT p.id, p.name,
	COALESCE(p.emoji, '') AS emoji,
	COALESCE(gc.games, 0) AS games,
	COALESCE(w.wins, 0) AS wins,
	COALESCE(s.seconds, 0) AS seconds,
	(COALESCE(w.wins, 0) * 3 + COALESCE(s.seconds, 0)) AS points,
	CASE WHEN COALESCE(gc.games, 0) = 0
		THEN 0
		ELSE CAST((COALESCE(w.wins, 0) * 3 + COALESCE(s.seconds, 0)) AS REAL) / gc.games
	END AS ppg
FROM players p
LEFT JOIN games_count gc ON gc.player_id = p.id
LEFT JOIN wins_count w ON w.player_id = p.id
LEFT JOIN seconds_count s ON s.player_id = p.id`)

	if where != "" {
		b.WriteString("\n")
		b.WriteString(where)
	}
	if order != "" {
		b.WriteString("\n")
		b.WriteString(order)
	}
	b.WriteString(";")

	return b.String()
}
