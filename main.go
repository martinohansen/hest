package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

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

	port := "8080"
	if portEnv := os.Getenv("HEST_PORT"); portEnv != "" {
		port = portEnv
	}

	slog.Info("listening on http://localhost:"+port,
		"version", versioninfo.Short(),
	)
	if err := http.ListenAndServe(":"+port, app.routes()); err != nil {
		log.Fatal(err)
	}
}
