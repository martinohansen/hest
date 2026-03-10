package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/carlmjohnson/versioninfo"
	"github.com/martinohansen/hest/internal/db"
	"github.com/martinohansen/hest/internal/weather"
)

func main() {
	store, err := db.Open("hest.db")
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer store.Close()

	backfillWeather(store)

	app := newApp(store)

	port := "8080"
	if portEnv := os.Getenv("HEST_PORT"); portEnv != "" {
		port = portEnv
	}

	slog.Info("listening on http://localhost:"+port,
		"version", versioninfo.Revision,
	)
	if err := http.ListenAndServe(":"+port, app.routes()); err != nil {
		log.Fatal(err)
	}
}

// backfillWeather fetches and stores weather data for all games that don't
// have it yet, using the Open-Meteo historic API. Runs once at startup.
func backfillWeather(store *db.Store) {
	games, err := store.GamesWithoutWeather()
	if err != nil {
		slog.Error("backfill: listing games", "error", err)
		return
	}
	if len(games) == 0 {
		return
	}

	slog.Info("backfilling weather", "games", len(games))
	for _, g := range games {
		w, err := weather.Fetch(g.PlayedAt, nil)
		if err != nil {
			slog.Warn("backfill: fetch weather", "game_id", g.ID, "error", err)
			continue
		}
		if err := store.UpdateGameWeather(g.ID, w); err != nil {
			slog.Warn("backfill: update game", "game_id", g.ID, "error", err)
			continue
		}
		slog.Info("backfill: updated", "game_id", g.ID, "weather", w)
	}
}
