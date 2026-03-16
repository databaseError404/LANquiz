package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func saveState() error {
	mu.RLock()
	ps := PersistedState{
		Title:   game.Title,
		Teams:   game.Teams,
		Answers: game.Answers,
		Round:   game.Round,
		History: game.History,
	}
	mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(game.DataPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(game.DataPath)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(ps)
}

func loadState() {
	f, err := os.Open(game.DataPath)
	if err != nil {
		return
	}
	defer f.Close()

	var ps PersistedState
	if err := json.NewDecoder(f).Decode(&ps); err != nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if ps.Title != "" {
		game.Title = ps.Title
	}
	if ps.Teams != nil {
		game.Teams = ps.Teams
	}
	if ps.Answers != nil {
		game.Answers = ps.Answers
	}
	game.Round = ps.Round
	game.History = ps.History
}

func autosaveLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mu.RLock()
		dirty := game.Dirty
		mu.RUnlock()

		if dirty {
			_ = saveState()
			mu.Lock()
			game.Dirty = false
			mu.Unlock()
		}
	}
}