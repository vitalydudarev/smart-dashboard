package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// The first day the mocked rate history covers.
var historyStart = time.Date(2016, time.January, 1, 0, 0, 0, 0, time.UTC)

func main() {
	loadDotEnv(".env")

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	// The mocked history ends on the day the server starts; a restart picks up any days since.
	srv := newServer(
		newNBRBProvider(newNBRBClient(nbrbBaseURL)),
		&mockProvider{store: newRateStore(historyStart, today())},
	)
	srv.weather = newWeatherService(newPogodaClient(pogodaBaseURL))
	if key := os.Getenv("API_FOOTBALL_KEY"); key != "" {
		srv.football = newFootballService(newFootballClient(footballBaseURL, key), defaultFootballCachePath())
	} else {
		log.Print("API_FOOTBALL_KEY is not set; football matches are unavailable")
	}

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, srv.routes()))
}

// loadDotEnv sets the KEY=VALUE pairs of a .env file that are not already in the environment. A
// missing file is fine: the variables can come from the environment instead.
func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.Trim(strings.TrimSpace(value), `"'`)
		if _, set := os.LookupEnv(key); !set {
			os.Setenv(key, value)
		}
	}
}
