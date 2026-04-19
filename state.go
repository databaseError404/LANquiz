package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
)

type Team struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Online   bool      `json:"online"`
	LastSeen time.Time `json:"lastSeen"`
	JoinedAt time.Time `json:"joinedAt"`
}

type Answer struct {
	TeamID   string    `json:"teamId"`
	TeamName string    `json:"teamName"`
	Choice   string    `json:"choice"`
	SentAt   time.Time `json:"sentAt"`
}

type RoundState struct {
	Number       int        `json:"number"`
	Open         bool       `json:"open"`
	AcceptLate   bool       `json:"acceptLate"`
	OpenedAt     *time.Time `json:"openedAt,omitempty"`
	ClosesAt     *time.Time `json:"closesAt,omitempty"`
	AllowChange  bool       `json:"allowChange"`
	HideAnswers  bool       `json:"hideAnswers"`
	ShowScreenQR bool       `json:"showScreenQR"`
	LanIP        string     `json:"lanIP,omitempty"`
	Correct      string     `json:"correct,omitempty"`
	Revealed     bool       `json:"revealed"`
}

type HistoryRow struct {
	Round    int       `json:"round"`
	TeamID   string    `json:"teamId"`
	TeamName string    `json:"teamName"`
	Choice   string    `json:"choice"`
	SentAt   time.Time `json:"sentAt"`
	Correct  string    `json:"correct"`
	IsRight  bool      `json:"isRight"`
}

type PublicTeam struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Online     bool   `json:"online"`
	Answered   bool   `json:"answered"`
	Choice     string `json:"choice,omitempty"`
	AnsweredAt string `json:"answeredAt,omitempty"`
}

type PublicState struct {
	Title          string       `json:"title"`
	ServerTimeUnix int64        `json:"serverTimeUnix"`
	Round          RoundState   `json:"round"`
	Teams          []PublicTeam `json:"teams"`
	OnlineCount    int          `json:"onlineCount"`
	AnsweredCount  int          `json:"answeredCount"`
	StatsRounds    []int        `json:"statsRounds,omitempty"`
	TeamStats      []TeamStats  `json:"teamStats,omitempty"`
	IPHints        []string     `json:"ipHints,omitempty"`
}

type TeamStats struct {
	TeamID       string   `json:"teamId"`
	TeamName     string   `json:"teamName"`
	TotalScore   int      `json:"totalScore"`
	RoundResults []string `json:"roundResults"`
}

type PersistedState struct {
	Title   string             `json:"title"`
	Teams   map[string]*Team   `json:"teams"`
	Answers map[string]*Answer `json:"answers"`
	Round   RoundState         `json:"round"`
	History []HistoryRow       `json:"history"`
}

type Game struct {
	Title    string
	Secret   string
	DataPath string

	Teams   map[string]*Team
	Answers map[string]*Answer
	Round   RoundState
	History []HistoryRow

	Events map[chan []byte]eventSubscriber
	Dirty  bool
}

type eventSubscriber struct {
	isHost bool
}

var (
	mu   sync.RWMutex
	game *Game
)

func initGame(title, secret, dataPath string) {
	game = &Game{
		Title:    title,
		Secret:   secret,
		DataPath: dataPath,
		Teams:    map[string]*Team{},
		Answers:  map[string]*Answer{},
		Events:   map[chan []byte]eventSubscriber{},
		Round: RoundState{
			Number:       1,
			Open:         false,
			AllowChange:  true,
			HideAnswers:  true,
			ShowScreenQR: false,
		},
	}
}

func publicState(isHost bool) PublicState {
	mu.RLock()
	defer mu.RUnlock()
	return publicStateLocked(isHost)
}

func publicStateLocked(isHost bool) PublicState {
	teams := make([]PublicTeam, 0, len(game.Teams))
	onlineCount := 0
	answeredCount := 0
	totalTeams := len(game.Teams)
	statsRounds := []int(nil)
	teamStats := []TeamStats(nil)

	for _, t := range game.Teams {
		if t.Online {
			onlineCount++
		}
		pt := PublicTeam{
			ID:     t.ID,
			Name:   t.Name,
			Online: t.Online,
		}
		if a, ok := game.Answers[t.ID]; ok {
			pt.Answered = true
			answeredCount++
			if game.Round.OpenedAt != nil {
				elapsed := int(a.SentAt.Sub(*game.Round.OpenedAt) / time.Second)
				pt.AnsweredAt = formatMMSS(elapsed)
			}
			if isHost || !game.Round.HideAnswers || (!game.Round.Open && !game.Round.AcceptLate) {
				pt.Choice = a.Choice
			}
		}
		teams = append(teams, pt)
	}

	sort.Slice(teams, func(i, j int) bool {
		if teams[i].Online != teams[j].Online {
			return teams[i].Online
		}
		return strings.ToLower(teams[i].Name) < strings.ToLower(teams[j].Name)
	})

	round := game.Round
	allAnswered := totalTeams == 0 || answeredCount >= totalTeams
	if !isHost && !allAnswered {
		round.Correct = ""
		round.Revealed = false
	}
	statsRounds, teamStats = buildTeamStatsLocked()

	return PublicState{
		Title:          game.Title,
		ServerTimeUnix: time.Now().Unix(),
		Round:          round,
		Teams:          teams,
		OnlineCount:    onlineCount,
		AnsweredCount:  answeredCount,
		StatsRounds:    statsRounds,
		TeamStats:      teamStats,
		IPHints:        localIPs(),
	}
}

func buildTeamStatsLocked() ([]int, []TeamStats) {
	roundCount := game.Round.Number - 1
	maxRound := roundCount
	for _, h := range game.History {
		if h.Round > maxRound {
			maxRound = h.Round
		}
	}

	if maxRound < 0 {
		maxRound = 0
	}

	rounds := make([]int, 0, maxRound)
	for i := 1; i <= maxRound; i++ {
		rounds = append(rounds, i)
	}

	teamNames := map[string]string{}
	for id, t := range game.Teams {
		teamNames[id] = t.Name
	}
	for _, h := range game.History {
		if _, ok := teamNames[h.TeamID]; !ok {
			teamNames[h.TeamID] = h.TeamName
		}
	}

	if len(teamNames) == 0 {
		return rounds, nil
	}

	type teamRoundResult struct {
		sentAt time.Time
		status string
	}

	results := map[string]map[int]teamRoundResult{}
	scores := map[string]int{}

	for _, h := range game.History {
		status := "wrong"
		if h.Correct == "" {
			status = "pending"
		} else if h.IsRight {
			status = "right"
		}

		byRound, ok := results[h.TeamID]
		if !ok {
			byRound = map[int]teamRoundResult{}
			results[h.TeamID] = byRound
		}

		prev, exists := byRound[h.Round]
		if !exists || h.SentAt.After(prev.sentAt) {
			byRound[h.Round] = teamRoundResult{
				sentAt: h.SentAt,
				status: status,
			}
		}

		if h.IsRight {
			scores[h.TeamID]++
		}
	}

	stats := make([]TeamStats, 0, len(teamNames))
	for teamID, teamName := range teamNames {
		row := make([]string, len(rounds))
		for i := range row {
			row[i] = "noanswer"
		}

		for i, roundNo := range rounds {
			if byRound, ok := results[teamID]; ok {
				if rr, ok := byRound[roundNo]; ok {
					row[i] = rr.status
				}
			}
		}

		stats = append(stats, TeamStats{
			TeamID:       teamID,
			TeamName:     teamName,
			TotalScore:   scores[teamID],
			RoundResults: row,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return strings.ToLower(stats[i].TeamName) < strings.ToLower(stats[j].TeamName)
	})

	return rounds, stats
}

func formatMMSS(totalSec int) string {
	if totalSec < 0 {
		totalSec = 0
	}
	return time.Unix(int64(totalSec), 0).UTC().Format("04:05")
}

func broadcastLocked() {
	hostPayload, _ := json.Marshal(publicStateLocked(true))
	publicPayload, _ := json.Marshal(publicStateLocked(false))
	for ch, sub := range game.Events {
		payload := publicPayload
		if sub.isHost {
			payload = hostPayload
		}
		select {
		case ch <- payload:
		default:
		}
	}
	game.Dirty = true
}

func closeRoundLocked() {
	if !game.Round.Open && !game.Round.AcceptLate {
		return
	}

	game.Round.Open = false
	game.Round.AcceptLate = false
	game.Round.ClosesAt = nil
	game.Round.OpenedAt = nil
	rebuildRoundHistoryLocked()
	broadcastLocked()
}

func rebuildRoundHistoryLocked() {
	filtered := make([]HistoryRow, 0, len(game.History))
	for _, h := range game.History {
		if h.Round != game.Round.Number {
			filtered = append(filtered, h)
		}
	}

	for _, a := range game.Answers {
		filtered = append(filtered, HistoryRow{
			Round:    game.Round.Number,
			TeamID:   a.TeamID,
			TeamName: a.TeamName,
			Choice:   a.Choice,
			SentAt:   a.SentAt,
			Correct:  game.Round.Correct,
			IsRight:  game.Round.Correct != "" && a.Choice == game.Round.Correct,
		})
	}
	game.History = filtered
}

func removeRoundHistoryLocked(roundNo int) {
	filtered := make([]HistoryRow, 0, len(game.History))
	for _, h := range game.History {
		if h.Round != roundNo {
			filtered = append(filtered, h)
		}
	}
	game.History = filtered
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
