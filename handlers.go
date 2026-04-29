package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func playerPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeHTML(w, playerHTML)
}

func hostPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/host" {
		http.NotFound(w, r)
		return
	}
	writeHTML(w, hostHTML)
}

func screenPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/screen" {
		http.NotFound(w, r)
		return
	}
	writeHTML(w, screenHTML)
}

func stateHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, publicState(checkHostSecret(r)))
}

func joinHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		TeamName string `json:"teamName"`
		TeamID   string `json:"teamId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	req.TeamName = strings.TrimSpace(req.TeamName)
	if req.TeamName == "" {
		http.Error(w, "empty team name", 400)
		return
	}

	now := time.Now()

	mu.Lock()
	defer mu.Unlock()

	var t *Team
	if req.TeamID != "" {
		if existing, ok := game.Teams[req.TeamID]; ok {
			t = existing
			t.Name = req.TeamName
			t.Online = true
			t.LastSeen = now
		}
	}
	if t == nil {
		t = &Team{
			ID:       randomID(),
			Name:     req.TeamName,
			Online:   true,
			LastSeen: now,
			JoinedAt: now,
		}
		game.Teams[t.ID] = t
	}

	broadcastLocked()
	writeJSON(w, map[string]any{
		"ok":     true,
		"teamId": t.ID,
		"name":   t.Name,
	})
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		TeamID string `json:"teamId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	t, ok := game.Teams[req.TeamID]
	if !ok {
		http.Error(w, "unknown team", 404)
		return
	}
	t.LastSeen = time.Now()
	t.Online = true
	broadcastLocked()

	writeJSON(w, map[string]any{"ok": true})
}

func answerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		TeamID string `json:"teamId"`
		Choice string `json:"choice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	req.Choice = strings.ToUpper(strings.TrimSpace(req.Choice))
	if req.Choice != "А" && req.Choice != "Б" && req.Choice != "В" && req.Choice != "Г" {
		http.Error(w, "choice must be А/Б/В/Г", 400)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	t, ok := game.Teams[req.TeamID]
	if !ok {
		http.Error(w, "unknown team", 404)
		return
	}
	if !game.Round.Open && !game.Round.AcceptLate {
		http.Error(w, "round closed", 409)
		return
	}
	if existing, exists := game.Answers[req.TeamID]; exists {
		if game.Round.AcceptLate {
			http.Error(w, "late mode accepts only unanswered teams", 409)
			return
		}
		if !game.Round.AllowChange {
			http.Error(w, "answer already submitted", 409)
			return
		}

		// Повторный выбор уже выбранного варианта снимает ответ
		// и возвращает команду в состояние "нет ответа".
		if existing.Choice == req.Choice {
			delete(game.Answers, req.TeamID)
			if !game.Round.Open {
				rebuildRoundHistoryLocked()
			}
			broadcastLocked()
			writeJSON(w, map[string]any{"ok": true})
			return
		}
	}

	t.LastSeen = time.Now()
	t.Online = true
	now := time.Now()

	game.Answers[req.TeamID] = &Answer{
		TeamID:   t.ID,
		TeamName: t.Name,
		Choice:   req.Choice,
		SentAt:   now,
	}

	if !game.Round.Open {
		rebuildRoundHistoryLocked()
	}
	if game.Round.AcceptLate && len(game.Answers) >= len(game.Teams) {
		game.Round.AcceptLate = false
	}

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func openRoundHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		DurationSec  int  `json:"durationSec"`
		AllowChange  bool `json:"allowChange"`
		ShowScreenQR bool `json:"showScreenQR"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	mu.Lock()
	defer mu.Unlock()

	if game.Round.Open || game.Round.AcceptLate {
		http.Error(w, "round is not finished", 409)
		return
	}

	now := time.Now()
	game.Answers = map[string]*Answer{}
	game.Round.Open = true
	game.Round.AcceptLate = false
	game.Round.OpenedAt = &now
	game.Round.AllowChange = req.AllowChange
	game.Round.HideAnswers = true
	game.Round.ShowScreenQR = req.ShowScreenQR
	game.Round.Correct = ""
	game.Round.Revealed = false

	if req.DurationSec > 0 {
		closesAt := now.Add(time.Duration(req.DurationSec) * time.Second)
		game.Round.ClosesAt = &closesAt
	} else {
		game.Round.ClosesAt = nil
	}

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func acceptLateAnswersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Принимать оставшиеся ответы можно только после завершения вопроса
	// и только если ответили не все команды.
	if game.Round.Open {
		http.Error(w, "round is still open", 409)
		return
	}
	if len(game.Teams) == 0 || len(game.Answers) >= len(game.Teams) {
		http.Error(w, "all teams already answered", 409)
		return
	}

	game.Round.Open = false
	game.Round.ClosesAt = nil
	game.Round.AcceptLate = true

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func setScreenQRHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		Show  bool   `json:"show"`
		LanIP string `json:"lanIP"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	game.Round.ShowScreenQR = req.Show
	if req.LanIP != "" {
		game.Round.LanIP = strings.TrimSpace(req.LanIP)
	} else {
		game.Round.LanIP = ""
	}
	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func setNonBurnModeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	game.Round.NonBurnMode = req.Enabled
	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func addTeamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		TeamName string `json:"teamName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	req.TeamName = strings.TrimSpace(req.TeamName)
	if req.TeamName == "" {
		http.Error(w, "empty team name", 400)
		return
	}

	now := time.Now()

	mu.Lock()
	defer mu.Unlock()

	t := &Team{
		ID:       randomID(),
		Name:     req.TeamName,
		Online:   false,
		LastSeen: now,
		JoinedAt: now,
	}
	game.Teams[t.ID] = t

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true, "teamId": t.ID, "name": t.Name})
}

func setTeamAnswerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		TeamID string `json:"teamId"`
		Choice string `json:"choice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	req.TeamID = strings.TrimSpace(req.TeamID)
	req.Choice = strings.ToUpper(strings.TrimSpace(req.Choice))
	if req.Choice == "—" {
		req.Choice = ""
	}

	if req.TeamID == "" {
		http.Error(w, "empty team id", 400)
		return
	}
	if req.Choice != "" && req.Choice != "А" && req.Choice != "Б" && req.Choice != "В" && req.Choice != "Г" {
		http.Error(w, "choice must be —/А/Б/В/Г", 400)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	t, ok := game.Teams[req.TeamID]
	if !ok {
		http.Error(w, "unknown team", 404)
		return
	}

	if !game.Round.Open && game.Round.Revealed {
		http.Error(w, "round already revealed", 409)
		return
	}

	if req.Choice == "" {
		delete(game.Answers, req.TeamID)
	} else {
		now := time.Now()
		game.Answers[req.TeamID] = &Answer{
			TeamID:   t.ID,
			TeamName: t.Name,
			Choice:   req.Choice,
			SentAt:   now,
		}

		if game.Round.AcceptLate && len(game.Answers) >= len(game.Teams) {
			game.Round.AcceptLate = false
		}
	}

	if !game.Round.Open {
		rebuildRoundHistoryLocked()
	}

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true, "teamId": t.ID, "choice": req.Choice})
}

func setTeamSafeSumsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		TeamID   string `json:"teamId"`
		SafeSums []int  `json:"safeSums"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	req.TeamID = strings.TrimSpace(req.TeamID)
	if req.TeamID == "" {
		http.Error(w, "empty team id", 400)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	t, ok := game.Teams[req.TeamID]
	if !ok {
		http.Error(w, "unknown team", 404)
		return
	}

	t.SafeSums = normalizeSafeSums(req.SafeSums)
	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true, "teamId": t.ID, "safeSums": t.SafeSums})
}

func renameTeamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		TeamID   string `json:"teamId"`
		TeamName string `json:"teamName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	req.TeamID = strings.TrimSpace(req.TeamID)
	req.TeamName = strings.TrimSpace(req.TeamName)
	if req.TeamID == "" {
		http.Error(w, "empty team id", 400)
		return
	}
	if req.TeamName == "" {
		http.Error(w, "empty team name", 400)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	t, ok := game.Teams[req.TeamID]
	if !ok {
		http.Error(w, "unknown team", 404)
		return
	}

	t.Name = req.TeamName
	if ans, ok := game.Answers[req.TeamID]; ok {
		ans.TeamName = req.TeamName
	}
	for i := range game.History {
		if game.History[i].TeamID == req.TeamID {
			game.History[i].TeamName = req.TeamName
		}
	}

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true, "teamId": t.ID, "name": t.Name})
}

func removeTeamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		TeamID string `json:"teamId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	req.TeamID = strings.TrimSpace(req.TeamID)
	if req.TeamID == "" {
		http.Error(w, "empty team id", 400)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if _, ok := game.Teams[req.TeamID]; !ok {
		http.Error(w, "unknown team", 404)
		return
	}

	delete(game.Teams, req.TeamID)
	delete(game.Answers, req.TeamID)

	filtered := make([]HistoryRow, 0, len(game.History))
	for _, h := range game.History {
		if h.TeamID != req.TeamID {
			filtered = append(filtered, h)
		}
	}
	game.History = filtered

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func closeRoundHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	mu.Lock()
	defer mu.Unlock()
	applyFinishRoundLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func stopRoundHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// "Остановить" — только прекратить приём ответов, без показа правильного ответа.
	applyStopRoundLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func revealHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		Correct string `json:"correct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	req.Correct = strings.ToUpper(strings.TrimSpace(req.Correct))
	if req.Correct != "" && req.Correct != "А" && req.Correct != "Б" && req.Correct != "В" && req.Correct != "Г" {
		http.Error(w, "correct must be А/Б/В/Г", 400)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if !game.Round.Open && game.Round.Revealed {
		http.Error(w, "round already revealed; use replay round", 409)
		return
	}

	game.Round.Correct = req.Correct
	game.Round.Revealed = false

	for i := range game.History {
		if game.History[i].Round == game.Round.Number {
			game.History[i].Correct = req.Correct
			game.History[i].IsRight = req.Correct != "" && game.History[i].Choice == req.Correct
		}
	}

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func replayRoundHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	game.Answers = map[string]*Answer{}
	game.Round.Open = false
	game.Round.AcceptLate = false
	game.Round.OpenedAt = nil
	game.Round.ClosesAt = nil
	game.Round.Correct = ""
	game.Round.Revealed = false
	removeRoundHistoryLocked(game.Round.Number)

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func resetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		Full bool `json:"full"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	mu.Lock()
	defer mu.Unlock()

	game.Answers = map[string]*Answer{}
	game.Round.Open = false
	game.Round.AcceptLate = false
	game.Round.OpenedAt = nil
	game.Round.ClosesAt = nil
	game.Round.Correct = ""
	game.Round.Revealed = false

	if req.Full {
		csvPath, err := exportStatsCSVLocked()
		if err != nil {
			http.Error(w, "failed to export csv: "+err.Error(), 500)
			return
		}

		game.History = []HistoryRow{}
		game.Round.Number = 1
		broadcastLocked()
		writeJSON(w, map[string]any{"ok": true, "csvPath": csvPath})
		return
	} else {
		if game.Round.Open || game.Round.AcceptLate {
			http.Error(w, "round is not finished", 409)
			return
		}
		game.Round.Number++
	}

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func prevRoundHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	game.Answers = map[string]*Answer{}
	game.Round.Open = false
	game.Round.AcceptLate = false
	game.Round.OpenedAt = nil
	game.Round.ClosesAt = nil
	game.Round.Correct = ""
	game.Round.Revealed = false

	if game.Round.Number > 1 {
		game.Round.Number--
	}

	broadcastLocked()
	writeJSON(w, map[string]any{"ok": true})
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 8)

	mu.Lock()
	game.Events[ch] = eventSubscriber{isHost: checkHostSecret(r)}
	initial, _ := json.Marshal(publicStateLocked(checkHostSecret(r)))
	mu.Unlock()

	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(initial)
	_, _ = w.Write([]byte("\n\n"))
	flusher.Flush()

	ctx := r.Context()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			mu.Lock()
			delete(game.Events, ch)
			close(ch)
			mu.Unlock()
			return
		case msg := <-ch:
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(msg)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		case <-ticker.C:
			_, _ = w.Write([]byte("data: {\"type\":\"keepalive\"}\n\n"))
			flusher.Flush()
		}
	}
}
