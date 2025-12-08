package main

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/martinohansen/hest/internal/db"
)

type App struct {
	store *db.Store
}

func newApp(store *db.Store) *App {
	return &App{store: store}
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	staticServer := http.FileServer(http.FS(staticContent))
	mux.Handle("/static/", http.StripPrefix("/static/", staticServer))
	mux.HandleFunc("/", a.handleLeaderboard)
	mux.HandleFunc("/games", a.handleGames)
	mux.HandleFunc("/games/save", a.handleSaveGame)
	mux.HandleFunc("/games/save-and-new", a.handleSaveAndNewGame)
	mux.HandleFunc("/new", a.handleNewGame)
	mux.HandleFunc("/new/score", a.handleScoreGame)
	mux.HandleFunc("/players", a.handleAddPlayer)
	return mux
}

func (a *App) renderTemplate(w http.ResponseWriter, tplName string, data any, files ...string) {
	if err := render(tplName, w, data, files...); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func render(tplName string, w http.ResponseWriter, data any, files ...string) error {
	for i, f := range files {
		files[i] = filepath.Clean(f)
	}
	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}
	tpl, err := template.New(filepath.Base(files[0])).Funcs(funcs).ParseFS(templateFS, files...)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tpl.ExecuteTemplate(w, tplName, data)
}

func (a *App) requireAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, pass, ok := r.BasicAuth()
	if !ok || pass != requiredPassword() || user == "" {
		a.unauthorized(w)
		return "", false
	}
	return strings.TrimSpace(user), true
}

func (a *App) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Hest"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("authorization required"))
}

func requiredPassword() string {
	if p := strings.TrimSpace(os.Getenv("HEST_PASSWORD")); p != "" {
		return p
	}
	return "hest"
}

func (a *App) listPlayers() ([]Player, error) {
	players, err := a.store.ListPlayers()
	if err != nil {
		return nil, err
	}
	return toPlayers(players), nil
}

func (a *App) listGames() ([]Game, error) {
	games, err := a.store.ListGames()
	if err != nil {
		return nil, err
	}
	return toGames(games), nil
}

func (a *App) playersByIDs(ids []int) ([]Player, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	players, err := a.store.PlayersByIDs(ids)
	if err != nil {
		return nil, err
	}
	if len(players) == 0 {
		return nil, nil
	}
	return toPlayers(players), nil
}
