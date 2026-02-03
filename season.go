package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/martinohansen/hest/internal/db"
)

type seasonContext struct {
	Seasons        []db.Season
	SeasonID       int
	SeasonExplicit bool
}

func (a *App) loadSeasonContext(r *http.Request) (seasonContext, *db.Season, error) {
	seasons, err := a.store.ListSeasons()
	if err != nil {
		return seasonContext{}, nil, err
	}

	selected, explicit, err := a.selectedSeason(r)
	if err != nil {
		return seasonContext{}, nil, err
	}

	ctx := buildSeasonContext(seasons, selected, explicit)
	return ctx, selected, nil
}

func buildSeasonContext(seasons []db.Season, selected *db.Season, explicit bool) seasonContext {
	ctx := seasonContext{
		Seasons:        seasons,
		SeasonExplicit: explicit,
	}
	if selected != nil {
		ctx.SeasonID = selected.ID
	}
	if explicit {
		id := 0
		if selected != nil {
			id = selected.ID
		}
		ctx.SeasonID = id
	}
	return ctx
}

func (a *App) selectedSeason(r *http.Request) (*db.Season, bool, error) {
	raw, hasParam := seasonParamFromRequest(r)
	if hasParam {
		if strings.TrimSpace(raw) == "" {
			return nil, true, nil
		}
		id, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || id <= 0 {
			return nil, true, nil
		}
		season, ok, err := a.store.SeasonByID(id)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, nil
		}
		return &season, true, nil
	}

	today := time.Now().Format(dateLayout)
	season, ok, err := a.store.SeasonForDate(today)
	if err != nil {
		return nil, false, err
	}
	if ok {
		return &season, false, nil
	}

	season, ok, err = a.store.LatestSeason()
	if err != nil {
		return nil, false, err
	}
	if ok {
		return &season, false, nil
	}

	return nil, false, nil
}

func seasonParamFromRequest(r *http.Request) (string, bool) {
	if values, ok := r.URL.Query()["season"]; ok {
		if len(values) == 0 {
			return "", true
		}
		return strings.TrimSpace(values[0]), true
	}

	if r.Method != http.MethodPost {
		return "", false
	}

	if err := r.ParseForm(); err != nil {
		return "", false
	}
	if values, ok := r.Form["season"]; ok {
		if len(values) == 0 {
			return "", true
		}
		return strings.TrimSpace(values[0]), true
	}
	return "", false
}

func seasonFilter(selected *db.Season) *db.DateRange {
	if selected == nil {
		return nil
	}
	return &db.DateRange{
		StartDate: selected.StartDate,
		EndDate:   selected.EndDate,
	}
}

func (a *App) seasonIDForDate(playedAt time.Time) (int, error) {
	date := playedAt.Format(dateLayout)
	season, ok, err := a.store.SeasonForDate(date)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	return season.ID, nil
}
