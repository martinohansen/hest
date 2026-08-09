package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/martinohansen/hest/internal/db"
)

type seasonsView struct {
	Path    string
	Title   string
	Seasons []db.Season
	Error   string
	Success string
	seasonContext
}

func newSeasonsView(seasons []db.Season) seasonsView {
	return seasonsView{
		Path:    "/seasons",
		Title:   "Sæsoner",
		Seasons: seasons,
	}
}

func (a *App) handleSeasons(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := requireAuth(w, r); !ok {
			return
		}
		ctx, _, err := a.loadSeasonContext(r)
		if err != nil {
			http.Error(w, "loading seasons", http.StatusInternalServerError)
			return
		}
		view := newSeasonsView(ctx.Seasons)
		view.seasonContext = ctx
		renderTemplate(w, "layout", view, "templates/layout.html", "templates/seasons.html")
		return
	case http.MethodPost:
		_, ok := a.ensureAuthAndForm(w, r)
		if !ok {
			return
		}
		if err := a.saveSeasons(r); err != nil {
			a.renderSeasonError(w, r, err.Error())
			return
		}
		a.renderSeasonSuccess(w, r, "Sæson gemt.")
		return
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (a *App) saveSeasons(r *http.Request) error {
	ids := r.Form["id"]
	for _, idRaw := range ids {
		idRaw = strings.TrimSpace(idRaw)
		if idRaw == "" {
			continue
		}
		id, err := strconv.Atoi(idRaw)
		if err != nil || id <= 0 {
			return errors.New("invalid season id")
		}

		name := strings.TrimSpace(r.FormValue("name_" + idRaw))
		start := strings.TrimSpace(r.FormValue("start_date_" + idRaw))
		end := strings.TrimSpace(r.FormValue("end_date_" + idRaw))

		if err := validateSeasonInput(name, start, end); err != nil {
			return err
		}

		if err := a.store.UpdateSeason(id, name, start, end); err != nil {
			if errors.Is(err, db.ErrSeasonOverlap) {
				return errors.New("season overlaps existing season")
			}
			if errors.Is(err, db.ErrSeasonNotFound) {
				return errors.New("season not found")
			}
			return err
		}
	}

	newName := strings.TrimSpace(r.FormValue("name_new"))
	newStart := strings.TrimSpace(r.FormValue("start_date_new"))
	newEnd := strings.TrimSpace(r.FormValue("end_date_new"))
	if newName != "" || newStart != "" || newEnd != "" {
		if err := validateSeasonInput(newName, newStart, newEnd); err != nil {
			return err
		}
		if err := a.store.AddSeason(newName, newStart, newEnd); err != nil {
			if errors.Is(err, db.ErrSeasonOverlap) {
				return errors.New("season overlaps existing season")
			}
			return err
		}
	}
	return nil
}

func validateSeasonInput(name, start, end string) error {
	if name == "" {
		return errors.New("name required")
	}
	if start == "" {
		return errors.New("start date required")
	}

	startDate, err := time.Parse(dateLayout, start)
	if err != nil {
		return errors.New("invalid start date")
	}
	if end != "" {
		endDate, err := time.Parse(dateLayout, end)
		if err != nil {
			return errors.New("invalid end date")
		}
		if startDate.After(endDate) {
			return errors.New("start date must be before end date")
		}
	}
	return nil
}

func (a *App) renderSeasonError(w http.ResponseWriter, r *http.Request, msg string) {
	ctx, _, err := a.loadSeasonContext(r)
	if err != nil {
		http.Error(w, "loading seasons", http.StatusInternalServerError)
		return
	}
	view := newSeasonsView(ctx.Seasons)
	view.seasonContext = ctx
	view.Error = msg
	renderTemplate(w, "layout", view, "templates/layout.html", "templates/seasons.html")
}

func (a *App) renderSeasonSuccess(w http.ResponseWriter, r *http.Request, msg string) {
	ctx, _, err := a.loadSeasonContext(r)
	if err != nil {
		http.Error(w, "loading seasons", http.StatusInternalServerError)
		return
	}
	view := newSeasonsView(ctx.Seasons)
	view.seasonContext = ctx
	view.Success = msg
	renderTemplate(w, "layout", view, "templates/layout.html", "templates/seasons.html")
}
