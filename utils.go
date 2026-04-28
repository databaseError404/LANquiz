package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeHTML(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	_, _ = w.Write([]byte(html))
}

func hostOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkHostSecret(r) {
			http.Error(w, "forbidden", 403)
			return
		}
		next(w, r)
	}
}

func checkHostSecret(r *http.Request) bool {
	if game.Secret == "" {
		return true
	}
	return r.Header.Get("X-Host-Secret") == game.Secret || r.URL.Query().Get("secret") == game.Secret
}

func pruneOfflineLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		changed := false
		now := time.Now()

		mu.Lock()
		for _, t := range game.Teams {
			online := now.Sub(t.LastSeen) < 12*time.Second
			if t.Online != online {
				t.Online = online
				changed = true
			}
		}
		if changed {
			broadcastLocked()
		}
		mu.Unlock()
	}
}

func autocloseLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		mu.Lock()
		if game.Round.Open && game.Round.ClosesAt != nil && time.Now().After(*game.Round.ClosesAt) {
			answeredAll := len(game.Teams) == 0 || len(game.Answers) >= len(game.Teams)
			if answeredAll {
				applyFinishRoundLocked()
			} else {
				applyStopRoundLocked()
			}
		}
		mu.Unlock()
	}
}

func localIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			ip = ip.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			s := ip.String()
			if strings.HasPrefix(s, "169.254.") {
				continue
			}
			ips = append(ips, s)
		}
	}
	sort.Strings(ips)
	return uniq(ips)
}

func uniq(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := []string{in[0]}
	for i := 1; i < len(in); i++ {
		if in[i] != in[i-1] {
			out = append(out, in[i])
		}
	}
	return out
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
