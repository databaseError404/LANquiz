package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

	// После перезапуска сервера выбор правильного ответа текущего вопроса
	// не переносим: новый запуск начинается без выбранного варианта.
	game.Round.Correct = ""
	game.Round.Revealed = false
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

func exportStatsCSVLocked() (string, error) {
	if err := os.MkdirAll("data/stats", 0755); err != nil {
		return "", err
	}

	stamp := time.Now().Format("20060102-150405")
	filePath := filepath.Join("data", "stats", fmt.Sprintf("stats-%s.csv", stamp))

	f, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// BOM нужен для корректного открытия UTF-8 CSV в Excel на Windows.
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return "", err
	}

	w := csv.NewWriter(f)

	rounds, stats := buildTeamStatsLocked()
	header := []string{"Название команды", "Общий счёт"}
	for _, r := range rounds {
		header = append(header, fmt.Sprintf("Вопрос %d", r))
	}
	if err := w.Write(header); err != nil {
		return "", err
	}

	for _, ts := range stats {
		row := []string{ts.TeamName, strconv.Itoa(ts.TotalScore)}
		for _, rr := range ts.RoundResults {
			row = append(row, roundResultRU(rr))
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}

	return filePath, nil
}

func roundResultRU(status string) string {
	switch status {
	case "right":
		return "Верно"
	case "wrong":
		return "Неверно"
	case "pending":
		return "Без проверки"
	default:
		return "Нет ответа"
	}
}
