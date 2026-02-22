package db

import (
	"testing"
	"time"
)

func TestAddGameWithWeather(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer store.Close()

	// Add players
	if err := store.AddPlayer("Alice"); err != nil {
		t.Fatalf("adding player: %v", err)
	}
	if err := store.AddPlayer("Bob"); err != nil {
		t.Fatalf("adding player: %v", err)
	}

	// Add a game with weather
	playedAt := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	weather := "☀️ 22°"
	if _, err := store.AddGame(playedAt, []int{1, 2}, 1, 2, "test", weather); err != nil {
		t.Fatalf("adding game: %v", err)
	}

	// List games and verify weather
	games, err := store.ListGames(nil)
	if err != nil {
		t.Fatalf("listing games: %v", err)
	}

	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}

	if games[0].Weather != weather {
		t.Errorf("expected weather %q, got %q", weather, games[0].Weather)
	}
}

func TestAddGameWithoutWeather(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer store.Close()

	if err := store.AddPlayer("Alice"); err != nil {
		t.Fatalf("adding player: %v", err)
	}
	if err := store.AddPlayer("Bob"); err != nil {
		t.Fatalf("adding player: %v", err)
	}

	playedAt := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	if _, err := store.AddGame(playedAt, []int{1, 2}, 1, 2, "test", ""); err != nil {
		t.Fatalf("adding game: %v", err)
	}

	games, err := store.ListGames(nil)
	if err != nil {
		t.Fatalf("listing games: %v", err)
	}

	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}

	if games[0].Weather != "" {
		t.Errorf("expected empty weather, got %q", games[0].Weather)
	}
}

func TestPlayerGamesWithWeather(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer store.Close()

	if err := store.AddPlayer("Alice"); err != nil {
		t.Fatalf("adding player: %v", err)
	}
	if err := store.AddPlayer("Bob"); err != nil {
		t.Fatalf("adding player: %v", err)
	}

	playedAt := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	weather := "🌧️ 10°"
	if _, err := store.AddGame(playedAt, []int{1, 2}, 1, 2, "test", weather); err != nil {
		t.Fatalf("adding game: %v", err)
	}

	games, err := store.PlayerGames(1, nil)
	if err != nil {
		t.Fatalf("listing player games: %v", err)
	}

	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}

	if games[0].Weather != weather {
		t.Errorf("expected weather %q, got %q", weather, games[0].Weather)
	}
}

func TestGamesWithoutWeather(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer store.Close()

	if err := store.AddPlayer("Alice"); err != nil {
		t.Fatalf("adding player: %v", err)
	}
	if err := store.AddPlayer("Bob"); err != nil {
		t.Fatalf("adding player: %v", err)
	}

	playedAt := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	// Game without weather
	if _, err := store.AddGame(playedAt, []int{1, 2}, 1, 2, "test", ""); err != nil {
		t.Fatalf("adding game: %v", err)
	}
	// Game with weather
	if _, err := store.AddGame(playedAt, []int{1, 2}, 2, 1, "test", "☀️ 22°"); err != nil {
		t.Fatalf("adding game: %v", err)
	}

	games, err := store.GamesWithoutWeather()
	if err != nil {
		t.Fatalf("listing games without weather: %v", err)
	}

	if len(games) != 1 {
		t.Fatalf("expected 1 game without weather, got %d", len(games))
	}

	if games[0].ID != 1 {
		t.Errorf("expected game ID 1, got %d", games[0].ID)
	}
}

func TestUpdateGameWeather(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer store.Close()

	if err := store.AddPlayer("Alice"); err != nil {
		t.Fatalf("adding player: %v", err)
	}
	if err := store.AddPlayer("Bob"); err != nil {
		t.Fatalf("adding player: %v", err)
	}

	playedAt := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	if _, err := store.AddGame(playedAt, []int{1, 2}, 1, 2, "test", ""); err != nil {
		t.Fatalf("adding game: %v", err)
	}

	if err := store.UpdateGameWeather(1, "🌧️ 10°"); err != nil {
		t.Fatalf("updating weather: %v", err)
	}

	games, err := store.ListGames(nil)
	if err != nil {
		t.Fatalf("listing games: %v", err)
	}

	if games[0].Weather != "🌧️ 10°" {
		t.Errorf("expected weather %q, got %q", "🌧️ 10°", games[0].Weather)
	}

	// No more games without weather
	noWeather, err := store.GamesWithoutWeather()
	if err != nil {
		t.Fatalf("listing games without weather: %v", err)
	}
	if len(noWeather) != 0 {
		t.Errorf("expected 0 games without weather, got %d", len(noWeather))
	}
}

func TestGetGameByID(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer store.Close()

	if err := store.AddPlayer("Alice"); err != nil {
		t.Fatalf("adding player: %v", err)
	}
	if err := store.AddPlayer("Bob"); err != nil {
		t.Fatalf("adding player: %v", err)
	}

	playedAt := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	weather := "☀️ 22°"
	if _, err := store.AddGame(playedAt, []int{1, 2}, 1, 2, "test", weather); err != nil {
		t.Fatalf("adding game: %v", err)
	}

	game, ok, err := store.GetGameByID(1)
	if err != nil {
		t.Fatalf("getting game: %v", err)
	}
	if !ok {
		t.Fatal("expected game to be found")
	}

	if game.ID != 1 {
		t.Errorf("expected game ID 1, got %d", game.ID)
	}
	if game.Weather != weather {
		t.Errorf("expected weather %q, got %q", weather, game.Weather)
	}
	if game.Winner.Name != "Alice" {
		t.Errorf("expected winner Alice, got %q", game.Winner.Name)
	}
	if game.Second.Name != "Bob" {
		t.Errorf("expected second Bob, got %q", game.Second.Name)
	}
	if len(game.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(game.Participants))
	}

	// Not found
	_, ok, err = store.GetGameByID(999)
	if err != nil {
		t.Fatalf("getting non-existent game: %v", err)
	}
	if ok {
		t.Error("expected game not to be found")
	}
}
