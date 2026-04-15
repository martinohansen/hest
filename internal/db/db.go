package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type Player struct {
	ID         int
	Name       string
	Emoji      string
	EasterEggs string
	Games      int
	Wins       int
	Seconds    int
	Points     int
	PPG        float64
}

type Game struct {
	ID           int
	PlayedAt     time.Time
	Winner       Player
	Second       Player
	Participants []Player
	CreatedBy    string
	CreatedAt    time.Time
	Weather      string
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
	Player1         Player
	Player2         Player
	SharedGames     int
	Player1Stats    Player
	Player2Stats    Player
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
	if err := migrate(db); err != nil {
		return err
	}
	return nil
}

// migrate applies incremental schema changes.
func migrate(db *sql.DB) error {
	// Add weather column to games table if it doesn't exist.
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('games') WHERE name='weather'`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		if _, err := db.Exec(`ALTER TABLE games ADD COLUMN weather TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}

	// Add easter_eggs column to players table if it doesn't exist.
	var easterEggsCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('players') WHERE name='easter_eggs'`).Scan(&easterEggsCount)
	if err != nil {
		return err
	}
	if easterEggsCount == 0 {
		if _, err := db.Exec(`ALTER TABLE players ADD COLUMN easter_eggs TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
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

CREATE TABLE IF NOT EXISTS seasons (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	start_date TEXT NOT NULL,
	end_date TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	CHECK (end_date IS NULL OR start_date <= end_date)
);

CREATE INDEX IF NOT EXISTS seasons_date_idx
	ON seasons (start_date, end_date);

CREATE TRIGGER IF NOT EXISTS seasons_no_overlap_insert
BEFORE INSERT ON seasons
BEGIN
	SELECT
		CASE
			WHEN EXISTS (
				SELECT 1
				FROM seasons s
				WHERE s.start_date <= COALESCE(NEW.end_date, '9999-12-31')
					AND (s.end_date IS NULL OR s.end_date >= NEW.start_date)
			)
			THEN RAISE(ABORT, 'season overlaps existing season')
		END;
END;

CREATE TRIGGER IF NOT EXISTS seasons_no_overlap_update
BEFORE UPDATE ON seasons
BEGIN
	SELECT
		CASE
			WHEN EXISTS (
				SELECT 1
				FROM seasons s
				WHERE s.id != NEW.id
					AND s.start_date <= COALESCE(NEW.end_date, '9999-12-31')
					AND (s.end_date IS NULL OR s.end_date >= NEW.start_date)
			)
			THEN RAISE(ABORT, 'season overlaps existing season')
		END;
END;

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

func dateRangeWhere(column string, filter *DateRange) (string, []any) {
	start, end, ok := normalizedDateRange(filter)
	if !ok {
		return "", nil
	}
	return fmt.Sprintf("WHERE date(%s) BETWEEN ? AND ?", column), []any{start, end}
}

func dateRangeAnd(column string, filter *DateRange) (string, []any) {
	start, end, ok := normalizedDateRange(filter)
	if !ok {
		return "", nil
	}
	return fmt.Sprintf(" AND date(%s) BETWEEN ? AND ?", column), []any{start, end}
}

func normalizedDateRange(filter *DateRange) (string, string, bool) {
	if filter == nil || !filter.Valid() {
		return "", "", false
	}
	start := strings.TrimSpace(filter.StartDate)
	end := strings.TrimSpace(filter.EndDate)
	if end == "" {
		end = "9999-12-31"
	}
	return start, end, true
}

// scanPlayer scans a player row from the playerTotalsQuery result.
func scanPlayer(scanner interface{ Scan(...any) error }) (Player, error) {
	var p Player
	err := scanner.Scan(&p.ID, &p.Name, &p.Emoji, &p.EasterEggs, &p.Games, &p.Wins, &p.Seconds, &p.Points, &p.PPG)
	return p, err
}

func (s *Store) AddPlayer(name string) error {
	_, err := s.db.Exec(`INSERT INTO players (name, emoji) VALUES (?, ?)`, name, emoji(name))
	return err
}

func (s *Store) ListGames(filter *DateRange) ([]Game, error) {
	whereClause, args := dateRangeWhere("g.played_at", filter)
	query := `
SELECT g.id, g.played_at,
	g.winner_id, winner.name, winner.emoji,
	g.second_id, second.name, second.emoji,
	COALESCE(g.created_by, ''),
	g.created_at,
	COALESCE(g.weather, '')
FROM games g
JOIN players winner ON winner.id = g.winner_id
JOIN players second ON second.id = g.second_id
`
	if whereClause != "" {
		query += whereClause + "\n"
	}
	query += "ORDER BY g.played_at DESC, g.id DESC\n"

	rows, err := s.db.Query(query, args...)
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
		if err := rows.Scan(&g.ID, &g.PlayedAt, &winnerID, &g.Winner.Name, &wEmoji, &secondID, &g.Second.Name, &sEmoji, &g.CreatedBy, &g.CreatedAt, &g.Weather); err != nil {
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
SELECT gp.game_id, p.id, p.name, p.emoji, COALESCE(p.easter_eggs, '')
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
		if err := rows.Scan(&gameID, &p.ID, &p.Name, &p.Emoji, &p.EasterEggs); err != nil {
			return nil, err
		}
		participantMap[gameID] = append(participantMap[gameID], p)
	}
	return participantMap, rows.Err()
}

func (s *Store) PlayerGameHistory(playerID int, filter *DateRange) ([]PlayerGameHistoryEntry, error) {
	andClause, dateArgs := dateRangeAnd("g.played_at", filter)
	query := `
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
	WHERE gp.player_id = ?` + andClause + `
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
`
	args := []any{playerID, playerID, playerID}
	args = append(args, dateArgs...)

	rows, err := s.db.Query(query, args...)
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

func (s *Store) PlayerRankHistory(playerID int, filter *DateRange) ([]PlayerRankHistoryEntry, error) {
	if filter == nil || !filter.Valid() {
		rows, err := s.db.Query(`
WITH player_first_game AS (
	-- Find the player's first game
	SELECT MIN(g.played_at) as first_game_date
	FROM games g
	JOIN game_players gp ON g.id = gp.game_id
	WHERE gp.player_id = ?
),
player_games AS (
	-- Get ALL games from the player's first game onward
	SELECT g.id, g.played_at
	FROM games g
	CROSS JOIN player_first_game pfg
	WHERE g.played_at >= pfg.first_game_date
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
			OR (g_hist.played_at = pg.played_at AND g_hist.id <= pg.id)
		)
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

	start, end, ok := normalizedDateRange(filter)
	if !ok {
		return nil, nil
	}

	query := `
WITH player_first_game AS (
	-- Find the player's first game in season
	SELECT MIN(g.played_at) as first_game_date
	FROM games g
	JOIN game_players gp ON g.id = gp.game_id
	WHERE gp.player_id = ?
		AND date(g.played_at) BETWEEN ? AND ?
),
player_games AS (
	-- Get ALL games from the player's first game onward (season only)
	SELECT g.id, g.played_at
	FROM games g
	CROSS JOIN player_first_game pfg
	WHERE g.played_at >= pfg.first_game_date
		AND date(g.played_at) BETWEEN ? AND ?
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
			OR (g_hist.played_at = pg.played_at AND g_hist.id <= pg.id)
		)
		AND date(g_hist.played_at) BETWEEN ? AND ?
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
`
	args := []any{
		playerID,
		start,
		end,
		start,
		end,
		start,
		end,
		playerID,
	}

	rows, err := s.db.Query(query, args...)
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

func (s *Store) PlayerGames(playerID int, filter *DateRange) ([]Game, error) {
	andClause, dateArgs := dateRangeAnd("g.played_at", filter)
	query := `
SELECT g.id, g.played_at,
	g.winner_id, winner.name, winner.emoji,
	g.second_id, second.name, second.emoji,
	COALESCE(g.created_by, ''),
	g.created_at,
	COALESCE(g.weather, '')
FROM games g
JOIN players winner ON winner.id = g.winner_id
JOIN players second ON second.id = g.second_id
JOIN game_players gp ON g.id = gp.game_id
WHERE gp.player_id = ?` + andClause + `
ORDER BY g.played_at DESC, g.id DESC
`
	args := []any{playerID}
	args = append(args, dateArgs...)

	rows, err := s.db.Query(query, args...)
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
		if err := rows.Scan(&g.ID, &g.PlayedAt, &winnerID, &g.Winner.Name, &wEmoji, &secondID, &g.Second.Name, &sEmoji, &g.CreatedBy, &g.CreatedAt, &g.Weather); err != nil {
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
	query, args := playerTotalsQuery("", `ORDER BY name ASC`, nil)
	rows, err := s.db.Query(query, args...)
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
func (s *Store) ListPlayersByPoints(filter *DateRange) ([]Player, error) {
	query, args := playerTotalsQuery("", `
ORDER BY points DESC, wins DESC, seconds DESC, games DESC, name ASC
`, filter)
	rows, err := s.db.Query(query, args...)
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

func (s *Store) PlayerTotalsByID(id int, filter *DateRange) (Player, bool, error) {
	query, args := playerTotalsQuery("WHERE p.id = ?", "", filter)
	args = append(args, id)

	row := s.db.QueryRow(query, args...)
	player, err := scanPlayer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Player{}, false, nil
		}
		return Player{}, false, err
	}
	return player, true, nil
}

func (s *Store) PlayersByIDs(ids []int) ([]Player, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders, args := buildPlaceholders(ids)
	query, seasonArgs := playerTotalsQuery(fmt.Sprintf("WHERE p.id IN (%s)", placeholders), "", nil)

	args = append(seasonArgs, args...)
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

func (s *Store) AddGame(playedAt time.Time, participantIDs []int, winnerID, secondID int, createdBy string, weather string) (int64, error) {
	uniqueIDs := Dedupe(participantIDs)
	if err := validateGameParticipants(uniqueIDs, winnerID, secondID); err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	res, err := tx.Exec(`INSERT INTO games (played_at, winner_id, second_id, created_by, weather) VALUES (?, ?, ?, ?, ?)`, playedAt, winnerID, secondID, createdBy, weather)
	if err != nil {
		return 0, err
	}

	gameID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare(`INSERT INTO game_players (game_id, player_id) VALUES (?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, pid := range uniqueIDs {
		if _, err = stmt.Exec(gameID, pid); err != nil {
			return 0, err
		}
	}

	err = tx.Commit()
	return gameID, err
}

// UpdateGame updates game date, placements and participants while preserving existing weather.
func (s *Store) UpdateGame(gameID int, playedAt time.Time, participantIDs []int, winnerID, secondID int) error {
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

	res, err := tx.Exec(`UPDATE games SET played_at = ?, winner_id = ?, second_id = ? WHERE id = ?`, playedAt, winnerID, secondID, gameID)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if _, err = tx.Exec(`DELETE FROM game_players WHERE game_id = ?`, gameID); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO game_players (game_id, player_id) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, participantID := range uniqueIDs {
		if _, err = stmt.Exec(gameID, participantID); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

// DeleteGame removes a game and its participants via cascading deletes.
func (s *Store) DeleteGame(gameID int) error {
	res, err := s.db.Exec(`DELETE FROM games WHERE id = ?`, gameID)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GamesWithoutWeather returns all games that have no weather data.
func (s *Store) GamesWithoutWeather() ([]Game, error) {
	rows, err := s.db.Query(`
SELECT id, played_at FROM games WHERE weather = '' ORDER BY played_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []Game
	for rows.Next() {
		var g Game
		if err := rows.Scan(&g.ID, &g.PlayedAt); err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

// UpdatePlayer updates a player's name, emoji, and easter_eggs fields.
func (s *Store) UpdatePlayer(id int, name, emoji, easterEggs string) error {
	_, err := s.db.Exec(`UPDATE players SET name = ?, emoji = ?, easter_eggs = ? WHERE id = ?`, name, emoji, easterEggs, id)
	return err
}

// ListPlayersBase returns all players with only base fields (id, name, emoji, easter_eggs).
func (s *Store) ListPlayersBase() ([]Player, error) {
	rows, err := s.db.Query(`SELECT id, name, COALESCE(emoji, ''), COALESCE(easter_eggs, '') FROM players ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []Player
	for rows.Next() {
		var p Player
		if err := rows.Scan(&p.ID, &p.Name, &p.Emoji, &p.EasterEggs); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

// UpdateGameWeather sets the weather string for a specific game.
func (s *Store) UpdateGameWeather(gameID int, weather string) error {
	_, err := s.db.Exec(`UPDATE games SET weather = ? WHERE id = ?`, weather, gameID)
	return err
}

// GetMaxGameID returns the maximum game ID in the database.
func (s *Store) GetMaxGameID() (int, error) {
	var maxID int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM games`).Scan(&maxID)
	return maxID, err
}

// GetGameByID returns a single game by its ID, including participants.
func (s *Store) GetGameByID(id int) (Game, bool, error) {
	query := `
SELECT g.id, g.played_at,
	g.winner_id, winner.name, winner.emoji,
	g.second_id, second.name, second.emoji,
	COALESCE(g.created_by, ''),
	g.created_at,
	COALESCE(g.weather, '')
FROM games g
JOIN players winner ON winner.id = g.winner_id
JOIN players second ON second.id = g.second_id
WHERE g.id = ?
`
	var (
		g        Game
		winnerID int
		secondID int
		wEmoji   string
		sEmoji   string
	)
	err := s.db.QueryRow(query, id).Scan(&g.ID, &g.PlayedAt, &winnerID, &g.Winner.Name, &wEmoji, &secondID, &g.Second.Name, &sEmoji, &g.CreatedBy, &g.CreatedAt, &g.Weather)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Game{}, false, nil
		}
		return Game{}, false, err
	}
	g.Winner.ID = winnerID
	g.Winner.Emoji = wEmoji
	g.Second.ID = secondID
	g.Second.Emoji = sEmoji

	participantMap, err := s.loadGameParticipants([]int{g.ID})
	if err != nil {
		return Game{}, false, err
	}
	g.Participants = participantMap[g.ID]

	return g, true, nil
}

// GetH2HStats returns head-to-head statistics for two players, including only games where both participated.
func (s *Store) GetH2HStats(player1ID, player2ID int, filter *DateRange) (H2HStats, error) {
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
	whereClause, dateArgs := dateRangeWhere("g.played_at", filter)
	query := `
SELECT g.id, g.played_at,
	g.winner_id, winner.name, winner.emoji,
	g.second_id, second.name, second.emoji,
	COALESCE(g.created_by, ''),
	g.created_at,
	COALESCE(g.weather, '')
FROM games g
JOIN players winner ON winner.id = g.winner_id
JOIN players second ON second.id = g.second_id
JOIN game_players gp1 ON g.id = gp1.game_id AND gp1.player_id = ?
JOIN game_players gp2 ON g.id = gp2.game_id AND gp2.player_id = ?
`
	if whereClause != "" {
		query += whereClause + "\n"
	}
	query += `ORDER BY g.played_at DESC, g.id DESC
`
	args := []any{player1ID, player2ID}
	args = append(args, dateArgs...)

	rows, err := s.db.Query(query, args...)
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
		if err := rows.Scan(&g.ID, &g.PlayedAt, &winnerID, &g.Winner.Name, &wEmoji, &secondID, &g.Second.Name, &sEmoji, &g.CreatedBy, &g.CreatedAt, &g.Weather); err != nil {
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
func playerTotalsQuery(where, order string, filter *DateRange) (string, []any) {
	var b strings.Builder
	var args []any
	seasonClause := ""
	seasonArgs := []any{}
	if start, end, ok := normalizedDateRange(filter); ok {
		seasonClause = "WHERE date(g.played_at) BETWEEN ? AND ?"
		seasonArgs = []any{start, end}
	}

	b.WriteString(`
WITH games_count AS (
    SELECT player_id, COUNT(*) AS games
    FROM game_players
    JOIN games g ON g.id = game_players.game_id
`)
	if seasonClause != "" {
		b.WriteString("    ")
		b.WriteString(seasonClause)
		b.WriteString("\n")
		args = append(args, seasonArgs...)
	}
	b.WriteString(`    GROUP BY player_id
),
wins_count AS (
    SELECT winner_id AS player_id, COUNT(*) AS wins
    FROM games g
`)
	if seasonClause != "" {
		b.WriteString("    ")
		b.WriteString(seasonClause)
		b.WriteString("\n")
		args = append(args, seasonArgs...)
	}
	b.WriteString(`    GROUP BY winner_id
),
seconds_count AS (
    SELECT second_id AS player_id, COUNT(*) AS seconds
    FROM games g
`)
	if seasonClause != "" {
		b.WriteString("    ")
		b.WriteString(seasonClause)
		b.WriteString("\n")
		args = append(args, seasonArgs...)
	}
	b.WriteString(`    GROUP BY second_id
)
SELECT p.id, p.name,
	COALESCE(p.emoji, '') AS emoji,
	COALESCE(p.easter_eggs, '') AS easter_eggs,
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

	return b.String(), args
}
