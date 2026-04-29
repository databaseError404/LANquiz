package main

import (
	"bufio"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

type rfReceiver struct {
	mu     sync.Mutex
	port   serial.Port
	stopCh chan struct{}
}

var rfLineRe = regexp.MustCompile(`^(=>|==|<=)\s+([0-9A-Fa-f]+)\s*\(`)

var rfButtonMap = map[string]string{
	"8": "А",
	"4": "Б",
	"2": "В",
	"1": "Г",
}

func listCOMPorts() []string {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		if p == nil || p.Name == "" {
			continue
		}
		name := strings.ToUpper(strings.TrimSpace(p.Name))
		if strings.HasPrefix(name, "COM") {
			out = append(out, p.Name)
		}
	}
	sort.Strings(out)
	return uniq(out)
}

func (r *rfReceiver) connect(portName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.port != nil {
		_ = r.port.Close()
		r.port = nil
	}
	if r.stopCh != nil {
		close(r.stopCh)
		r.stopCh = nil
	}

	mode := &serial.Mode{BaudRate: 9600}
	p, err := serial.Open(portName, mode)
	if err != nil {
		return err
	}

	r.port = p
	r.stopCh = make(chan struct{})
	go r.readLoop(p, r.stopCh)
	return nil
}

func (r *rfReceiver) disconnect() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopCh != nil {
		close(r.stopCh)
		r.stopCh = nil
	}
	if r.port != nil {
		_ = r.port.Close()
		r.port = nil
	}
}

func (r *rfReceiver) readLoop(p serial.Port, stopCh chan struct{}) {
	scanner := bufio.NewScanner(p)
	for scanner.Scan() {
		select {
		case <-stopCh:
			return
		default:
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		handleRFLine(line)
	}
	if err := scanner.Err(); err != nil {
		mu.Lock()
		game.RFStatus.Connected = false
		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "port has been closed") {
			game.RFStatus.Error = "COM порт закрыт"
		} else {
			game.RFStatus.Error = "Ошибка чтения COM: " + msg
		}
		broadcastLocked()
		mu.Unlock()
	}
}

var rf = &rfReceiver{}

var rfPairingTimer *time.Timer
var rfHostPairingTimer *time.Timer

func setRFPairingTimeoutLocked(enabled bool) {
	if rfPairingTimer != nil {
		rfPairingTimer.Stop()
		rfPairingTimer = nil
	}
	if !enabled {
		return
	}
	rfPairingTimer = time.AfterFunc(30*time.Second, func() {
		mu.Lock()
		defer mu.Unlock()
		if game.RFPairing {
			game.RFPairing = false
			game.RFStatus.Error = "Режим привязки выключен по таймауту 30 сек"
			broadcastLocked()
		}
	})
}

func setRFHostPairingTimeoutLocked(enabled bool) {
	if rfHostPairingTimer != nil {
		rfHostPairingTimer.Stop()
		rfHostPairingTimer = nil
	}
	if !enabled {
		return
	}
	rfHostPairingTimer = time.AfterFunc(30*time.Second, func() {
		mu.Lock()
		defer mu.Unlock()
		if game.RFHostPairing {
			game.RFHostPairing = false
			game.RFStatus.Error = "Режим привязки пульта ведущего выключен по таймауту 30 сек"
			broadcastLocked()
		}
	})
}

func applyRFHostChoiceLocked(choice string) error {
	if choice != "А" && choice != "Б" && choice != "В" && choice != "Г" {
		return fmt.Errorf("unsupported choice")
	}
	canGoNextRound := !game.Round.Open && !game.Round.AcceptLate && game.Round.Revealed && game.Round.Correct != ""
	if canGoNextRound {
		game.Answers = map[string]*Answer{}
		game.Round.Open = false
		game.Round.AcceptLate = false
		game.Round.OpenedAt = nil
		game.Round.ClosesAt = nil
		game.Round.Correct = ""
		game.Round.Revealed = false
		game.Round.Number++
	}
	if !game.Round.Open && game.Round.Revealed {
		return fmt.Errorf("round already revealed; use replay round")
	}
	if !game.Round.Open {
		now := time.Now()
		game.Answers = map[string]*Answer{}
		game.Round.Open = true
		game.Round.AcceptLate = false
		game.Round.OpenedAt = &now
		game.Round.HideAnswers = true
		game.Round.Correct = ""
		game.Round.Revealed = false
		if game.HostDurationSec > 0 {
			closesAt := now.Add(time.Duration(game.HostDurationSec) * time.Second)
			game.Round.ClosesAt = &closesAt
		} else {
			game.Round.ClosesAt = nil
		}
	}

	game.Round.Correct = choice
	game.Round.Revealed = false
	for i := range game.History {
		if game.History[i].Round == game.Round.Number {
			game.History[i].Correct = choice
			game.History[i].IsRight = choice != "" && game.History[i].Choice == choice
		}
	}
	return nil
}

func handleRFLine(line string) {
	m := rfLineRe.FindStringSubmatch(line)
	if len(m) < 3 {
		return
	}
	mark := m[1]
	hexCode := strings.ToUpper(strings.TrimSpace(m[2]))
	if len(hexCode) < 1 {
		return
	}
	base := ""
	if len(hexCode) > 1 {
		base = hexCode[:len(hexCode)-1]
	}
	btnCode := hexCode[len(hexCode)-1:]
	choice, ok := rfButtonMap[btnCode]

	mu.Lock()
	defer mu.Unlock()

	game.RFStatus.LastRaw = line
	if ok {
		game.RFStatus.LastButton = choice
	}

	if mark != "=>" {
		broadcastLocked()
		return
	}

	if game.RFPairing && choice == "А" {
		teamID := strings.TrimSpace(game.RFBindings[base])
		created := false
		teamName := ""
		if teamID == "" {
			now := time.Now()
			t := &Team{
				ID:       randomID(),
				Name:     "RF " + base,
				Online:   false,
				LastSeen: now,
				JoinedAt: now,
			}
			game.Teams[t.ID] = t
			game.RFBindings[base] = t.ID
			created = true
			teamName = t.Name
		} else if _, exists := game.Teams[teamID]; !exists {
			now := time.Now()
			t := &Team{
				ID:       teamID,
				Name:     "RF " + base,
				Online:   false,
				LastSeen: now,
				JoinedAt: now,
			}
			game.Teams[t.ID] = t
			created = true
			teamName = t.Name
		} else if t, ok := game.Teams[teamID]; ok && t != nil {
			teamName = t.Name
			game.RFStatus.Hint = fmt.Sprintf("Пульт %s уже привязан к команде \"%s\"", base, teamName)
		}
		if created {
			game.RFStatus.Hint = fmt.Sprintf("Пульт %s привязан к команде \"%s\"", base, teamName)
			game.RFPairing = false
			setRFPairingTimeoutLocked(false)
		}
		broadcastLocked()
		return
	}

	if game.RFHostPairing && choice == "А" {
		if strings.TrimSpace(base) == "" {
			game.RFStatus.Error = "Не удалось определить базовый ID пульта ведущего"
			broadcastLocked()
			return
		}
		if teamID := strings.TrimSpace(game.RFBindings[base]); teamID != "" {
			teamName := teamID
			if t, ok := game.Teams[teamID]; ok && t != nil && strings.TrimSpace(t.Name) != "" {
				teamName = t.Name
			}
			game.RFStatus.Error = fmt.Sprintf("Пульт %s уже привязан к команде \"%s\"; привязка к ведущему отклонена", base, teamName)
			broadcastLocked()
			return
		}
		game.RFHostBinding = base
		game.RFHostPairing = false
		setRFHostPairingTimeoutLocked(false)
		game.RFStatus.Hint = fmt.Sprintf("Пульт ведущего %s успешно привязан", base)
		broadcastLocked()
		return
	}

	if !ok {
		broadcastLocked()
		return
	}

	if bound := strings.TrimSpace(game.RFHostBinding); bound != "" && bound == base {
		if err := applyRFHostChoiceLocked(choice); err != nil {
			game.RFStatus.Error = fmt.Sprintf("RF команда ведущего отклонена: %v", err)
		}
		broadcastLocked()
		return
	}

	teamID := strings.TrimSpace(game.RFBindings[base])
	if teamID == "" {
		broadcastLocked()
		return
	}
	if err := submitAnswerLocked(teamID, choice, false); err != nil {
		game.RFStatus.Error = fmt.Sprintf("RF ответ отклонён: %v", err)
	}
	broadcastLocked()
}

func connectRFSelectedPortLocked() error {
	port := strings.TrimSpace(game.RFSelectedPort)
	if port == "" {
		return fmt.Errorf("порт не выбран")
	}
	if err := rf.connect(port); err != nil {
		game.RFStatus.Connected = false
		game.RFStatus.Error = err.Error()
		return err
	}
	game.RFStatus.Connected = true
	game.RFStatus.Error = ""
	return nil
}

func disconnectRFLocked() {
	rf.disconnect()
	game.RFStatus.Connected = false
}

func initRFAfterLoad() {
	mu.Lock()
	game.RFStatus.Ports = listCOMPorts()
	selected := strings.TrimSpace(game.RFSelectedPort)
	shouldConnect := selected != ""
	mu.Unlock()

	if shouldConnect {
		mu.Lock()
		err := connectRFSelectedPortLocked()
		if err != nil {
			log.Printf("RF auto-connect failed: %v", err)
		}
		broadcastLocked()
		mu.Unlock()
	}
}
