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
	if err := store.ensurePlayerEmojis(); err != nil {
		database.Close()
		return nil, err
	}

	return store, nil
}

func ensureSchema(db *sql.DB) error {
	if err := createTables(db); err != nil {
		return err
	}
	return ensurePlayerEmojiColumn(db)
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

func ensurePlayerEmojiColumn(db *sql.DB) error {
	const query = `PRAGMA table_info(players);`
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			dfltValue  any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &primaryKey); err != nil {
			return err
		}
		if strings.EqualFold(name, "emoji") {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(`ALTER TABLE players ADD COLUMN emoji TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *Store) ensurePlayerEmojis() error {
	rows, err := s.db.Query(`SELECT id, name, COALESCE(emoji, '') FROM players ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type playerRow struct {
		id    int
		name  string
		emoji string
	}

	var rowsData []playerRow
	used := make(map[string]struct{})

	for rows.Next() {
		var p playerRow
		if err := rows.Scan(&p.id, &p.name, &p.emoji); err != nil {
			return err
		}
		p.emoji = strings.TrimSpace(p.emoji)
		if p.emoji != "" {
			used[p.emoji] = struct{}{}
		}
		rowsData = append(rowsData, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(rowsData) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, p := range rowsData {
		if p.emoji != "" {
			continue
		}
		emoji := chooseEmoji(p.name, p.id, used)
		used[emoji] = struct{}{}
		if _, err := tx.Exec(`UPDATE players SET emoji = ? WHERE id = ?`, emoji, p.id); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) usedEmojis() (map[string]struct{}, error) {
	rows, err := s.db.Query(`SELECT emoji FROM players WHERE TRIM(COALESCE(emoji, '')) != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	used := make(map[string]struct{})
	for rows.Next() {
		var emoji string
		if err := rows.Scan(&emoji); err != nil {
			return nil, err
		}
		emoji = strings.TrimSpace(emoji)
		if emoji == "" {
			continue
		}
		used[emoji] = struct{}{}
	}
	return used, rows.Err()
}

func (s *Store) nextEmoji(name string) (string, error) {
	used, err := s.usedEmojis()
	if err != nil {
		return "", err
	}
	emoji := chooseEmoji(name, 0, used)
	if emoji == "" {
		return "", fmt.Errorf("no emojis available")
	}
	return emoji, nil
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
	emoji, err := s.nextEmoji(name)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO players (name, emoji) VALUES (?, ?)`, name, emoji)
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

func (s *Store) ListPlayers() ([]Player, error) {
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
