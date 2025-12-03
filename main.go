package main

import (
	"log"
	"log/slog"
	"net/http"

	"github.com/carlmjohnson/versioninfo"
	"github.com/martinohansen/hest/internal/db"
)

func main() {
	store, err := db.Open("hest.db")
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer store.Close()

	app := newApp(store)

	slog.Info("listening on http://localhost:8080",
		"version", versioninfo.Short(),
	)
	if err := http.ListenAndServe(":8080", app.routes()); err != nil {
		log.Fatal(err)
	}
}
