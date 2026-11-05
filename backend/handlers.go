package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

type server struct {
	store *rateStore
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/currencies", s.handleCurrencies)
	mux.HandleFunc("GET /api/rates/latest", s.handleLatestRate)
	mux.HandleFunc("GET /api/rates/history", s.handleRateHistory)
	return mux
}

type latestRateResponse struct {
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
	// The day the rate was loaded for, as YYYY-MM-DD.
	Date string `json:"date"`
}

type rateHistoryResponse struct {
	Base   string      `json:"base"`
	Quote  string      `json:"quote"`
	From   string      `json:"from"`
	To     string      `json:"to"`
	Points []RatePoint `json:"points"`
}

func (s *server) handleCurrencies(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, currencies)
}

// GET /api/rates/latest?base=EUR&quote=USD
func (s *server) handleLatestRate(w http.ResponseWriter, r *http.Request) {
	base, quote, err := s.pairParams(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	day := s.store.latest()
	writeJSON(w, http.StatusOK, latestRateResponse{
		Base:  base,
		Quote: quote,
		Rate:  s.store.rate(base, quote, day),
		Date:  day.Format(dateLayout),
	})
}

// GET /api/rates/history?base=EUR&quote=USD&range=30d
// GET /api/rates/history?base=EUR&quote=USD&from=2026-01-01&to=2026-03-31
func (s *server) handleRateHistory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	base, quote, err := s.pairParams(query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	from, to, err := s.period(query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, rateHistoryResponse{
		Base:   base,
		Quote:  quote,
		From:   from.Format(dateLayout),
		To:     to.Format(dateLayout),
		Points: s.store.history(base, quote, from, to),
	})
}

func (s *server) pairParams(query url.Values) (base, quote string, err error) {
	base, quote = query.Get("base"), query.Get("quote")

	for _, code := range []string{base, quote} {
		if !s.store.isKnown(code) {
			return "", "", fmt.Errorf("unknown currency %q", code)
		}
	}
	if base == quote {
		return "", "", errors.New("base and quote must differ")
	}

	return base, quote, nil
}

// period resolves either a preset range, counted back from the latest day, or an explicit
// from/to pair, which is clamped to the days the store has data for.
func (s *server) period(query url.Values) (from, to time.Time, err error) {
	latest := s.store.latest()

	if preset := query.Get("range"); preset != "" {
		switch preset {
		case "7d":
			from = latest.AddDate(0, 0, -6)
		case "30d":
			from = latest.AddDate(0, 0, -29)
		case "6m":
			from = latest.AddDate(0, -6, 0)
		case "1y":
			from = latest.AddDate(-1, 0, 0)
		case "5y":
			from = latest.AddDate(-5, 0, 0)
		default:
			return time.Time{}, time.Time{}, fmt.Errorf("unknown range %q", preset)
		}
		return maxTime(from, s.store.earliest()), latest, nil
	}

	from, err = time.Parse(dateLayout, query.Get("from"))
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("from must be a date as YYYY-MM-DD")
	}
	to, err = time.Parse(dateLayout, query.Get("to"))
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("to must be a date as YYYY-MM-DD")
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, errors.New("from must not be after to")
	}

	from, to = maxTime(from, s.store.earliest()), minTime(to, latest)
	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("rates are available from %s to %s",
			s.store.earliest().Format(dateLayout), latest.Format(dateLayout))
	}

	return from, to, nil
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
