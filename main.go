package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

var (
	addrFlag   = flag.String("addr", ":8080", "listen address")
	titleFlag  = flag.String("title", "LAN Quiz", "game title")
	secretFlag = flag.String("secret", "", "optional host secret")
	openFlag   = flag.Bool("open-browser", true, "open browser on startup")
	dataFile   = flag.String("data", "data/state.json", "path to state json")
)

func main() {
	flag.Parse()

	initGame(*titleFlag, *secretFlag, *dataFile)
	loadState()

	go pruneOfflineLoop()
	go autocloseLoop()
	go autosaveLoop()

	http.HandleFunc("/", playerPage)
	http.HandleFunc("/host", hostPage)
	http.HandleFunc("/screen", screenPage)

	http.HandleFunc("/events", eventsHandler)
	http.HandleFunc("/api/state", stateHandler)

	http.HandleFunc("/api/join", joinHandler)
	http.HandleFunc("/api/ping", pingHandler)
	http.HandleFunc("/api/answer", answerHandler)

	http.HandleFunc("/api/host/open", hostOnly(openRoundHandler))
	http.HandleFunc("/api/host/close", hostOnly(closeRoundHandler))
	http.HandleFunc("/api/host/reset", hostOnly(resetHandler))
	http.HandleFunc("/api/host/reveal", hostOnly(revealHandler))
	http.HandleFunc("/api/host/screen-qr", hostOnly(setScreenQRHandler))
	http.HandleFunc("/api/host/team/remove", hostOnly(removeTeamHandler))

	http.HandleFunc("/manifest.webmanifest", manifestHandler)
	http.HandleFunc("/sw.js", serviceWorkerHandler)

	http.HandleFunc("/qr.png", qrHandler)

	ipHints := localIPs()
	fmt.Println("======================================")
	fmt.Printf("Local:   http://localhost%s\n", *addrFlag)
	for _, ip := range ipHints {
		fmt.Printf("Players: http://%s%s/\n", ip, *addrFlag)
		fmt.Printf("Host:    http://%s%s/host\n", ip, *addrFlag)
		fmt.Printf("Screen:  http://%s%s/screen\n", ip, *addrFlag)
	}
	if game.Secret != "" {
		fmt.Println("Host secret enabled.")
	}
	fmt.Println("======================================")

	if *openFlag {
		go func() {
			time.Sleep(700 * time.Millisecond)
			openBrowser("http://localhost" + *addrFlag + "/host")
		}()
	}

	log.Fatal(http.ListenAndServe(*addrFlag, nil))
}
