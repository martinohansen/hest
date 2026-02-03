package db

import (
	"database/sql"
	"errors"
	"strings"
)

var (
	ErrSeasonOverlap  = errors.New("season overlaps existing season")
	ErrSeasonNotFound = errors.New("season not found")
)

type Season struct {
	ID        int
	Name      string
	StartDate string
	EndDate   string
}

type DateRange struct {
	StartDate string
	EndDate   string
}

func (r DateRange) Valid() bool {
	return strings.TrimSpace(r.StartDate) != ""
}

func (s *Store) ListSeasons() ([]Season, error) {
	rows, err := s.db.Query(`
SELECT id, name, start_date, end_date
FROM seasons
ORDER BY start_date DESC, id DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []Season
	for rows.Next() {
		var season Season
		var endDate sql.NullString
		if err := rows.Scan(&season.ID, &season.Name, &season.StartDate, &endDate); err != nil {
			return nil, err
		}
		if endDate.Valid {
			season.EndDate = endDate.String
		}
		seasons = append(seasons, season)
	}
	return seasons, rows.Err()
}

func (s *Store) SeasonByID(id int) (Season, bool, error) {
	var season Season
	var endDate sql.NullString
	err := s.db.QueryRow(`
SELECT id, name, start_date, end_date
FROM seasons
WHERE id = ?
`, id).Scan(&season.ID, &season.Name, &season.StartDate, &endDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Season{}, false, nil
		}
		return Season{}, false, err
	}
	if endDate.Valid {
		season.EndDate = endDate.String
	}
	return season, true, nil
}

func (s *Store) SeasonForDate(date string) (Season, bool, error) {
	var season Season
	var endDate sql.NullString
	err := s.db.QueryRow(`
SELECT id, name, start_date, end_date
FROM seasons
WHERE start_date <= ? AND (end_date IS NULL OR end_date >= ?)
ORDER BY start_date DESC, id DESC
LIMIT 1
`, date, date).Scan(&season.ID, &season.Name, &season.StartDate, &endDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Season{}, false, nil
		}
		return Season{}, false, err
	}
	if endDate.Valid {
		season.EndDate = endDate.String
	}
	return season, true, nil
}

func (s *Store) LatestSeason() (Season, bool, error) {
	var season Season
	var endDate sql.NullString
	err := s.db.QueryRow(`
SELECT id, name, start_date, end_date
FROM seasons
ORDER BY start_date DESC, id DESC
LIMIT 1
`).Scan(&season.ID, &season.Name, &season.StartDate, &endDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Season{}, false, nil
		}
		return Season{}, false, err
	}
	if endDate.Valid {
		season.EndDate = endDate.String
	}
	return season, true, nil
}

func (s *Store) AddSeason(name, startDate, endDate string) error {
	name = strings.TrimSpace(name)
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)

	endValue, _ := normalizeEndDate(endDate)
	_, err := s.db.Exec(`
INSERT INTO seasons (name, start_date, end_date)
VALUES (?, ?, ?)
`, name, startDate, endValue)
	if err != nil {
		if isSeasonOverlapError(err) {
			return ErrSeasonOverlap
		}
		return err
	}
	return nil
}

func (s *Store) UpdateSeason(id int, name, startDate, endDate string) error {
	name = strings.TrimSpace(name)
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)

	endValue, _ := normalizeEndDate(endDate)
	res, err := s.db.Exec(`
UPDATE seasons
SET name = ?, start_date = ?, end_date = ?
WHERE id = ?
`, name, startDate, endValue, id)
	if err != nil {
		if isSeasonOverlapError(err) {
			return ErrSeasonOverlap
		}
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrSeasonNotFound
	}

	return nil
}

func normalizeEndDate(value string) (any, any) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	return trimmed, trimmed
}

func isSeasonOverlapError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "season overlaps existing season")
}
