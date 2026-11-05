package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type ExchangeRate struct {
	Pair string  `json:"pair"`
	Rate float64 `json:"rate"`
	// The day the rate was loaded for, as YYYY-MM-DD.
	Date string `json:"date"`
}

// Placeholder until the backend fetches real rates.
var eurUsdRate = ExchangeRate{
	Pair: "EUR/USD",
	Rate: 1.0842,
	Date: "2026-11-02",
}

func handleEurUsdRate(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(eurUsdRate); err != nil {
		log.Printf("encode rate: %v", err)
	}
}

func main() {
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/rates/eur-usd", handleEurUsdRate)

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
