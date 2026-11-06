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
	// The first provider that supports a pair serves it.
	providers []rateProvider
}

func newServer(providers ...rateProvider) *server {
	return &server{providers: providers}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/currencies", s.handleCurrencies)
	mux.HandleFunc("GET /api/rates/latest", s.handleRate)
	mux.HandleFunc("GET /api/rates/history", s.handleRateHistory)
	return mux
}

type latestRateResponse struct {
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
	// The day the rate is for, as YYYY-MM-DD.
	Date   string `json:"date"`
	Source Source `json:"source"`
}

type rateHistoryResponse struct {
	Base   string      `json:"base"`
	Quote  string      `json:"quote"`
	From   string      `json:"from"`
	To     string      `json:"to"`
	Points []RatePoint `json:"points"`
	Source Source      `json:"source"`
}

func (s *server) handleCurrencies(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, currencies)
}

// GET /api/rates/latest?base=EUR&quote=USD
// GET /api/rates/latest?base=USD&quote=BYN&date=2025-10-10
func (s *server) handleRate(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	base, quote, provider, err := s.pairParams(query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	earliest, latest := provider.bounds()
	day := latest
	if raw := query.Get("date"); raw != "" {
		day, err = time.Parse(dateLayout, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "date must be a date as YYYY-MM-DD")
			return
		}
		if day.Before(earliest) || day.After(latest) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("rates are available from %s to %s",
				earliest.Format(dateLayout), latest.Format(dateLayout)))
			return
		}
	}

	rate, err := provider.rate(r.Context(), base, quote, day)
	if err != nil {
		writeProviderError(w, provider, err)
		return
	}

	writeJSON(w, http.StatusOK, latestRateResponse{
		Base:   base,
		Quote:  quote,
		Rate:   rate,
		Date:   day.Format(dateLayout),
		Source: provider.source(),
	})
}

// GET /api/rates/history?base=EUR&quote=USD&range=30d
// GET /api/rates/history?base=EUR&quote=USD&from=2026-01-01&to=2026-03-31
func (s *server) handleRateHistory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	base, quote, provider, err := s.pairParams(query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	from, to, err := period(query, provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	points, err := provider.history(r.Context(), base, quote, from, to)
	if err != nil {
		writeProviderError(w, provider, err)
		return
	}

	writeJSON(w, http.StatusOK, rateHistoryResponse{
		Base:   base,
		Quote:  quote,
		From:   from.Format(dateLayout),
		To:     to.Format(dateLayout),
		Points: points,
		Source: provider.source(),
	})
}

func (s *server) pairParams(query url.Values) (base, quote string, provider rateProvider, err error) {
	base, quote = query.Get("base"), query.Get("quote")

	for _, code := range []string{base, quote} {
		if !isKnownCurrency(code) {
			return "", "", nil, fmt.Errorf("unknown currency %q", code)
		}
	}
	if base == quote {
		return "", "", nil, errors.New("base and quote must differ")
	}

	for _, provider := range s.providers {
		if provider.supports(base, quote) {
			return base, quote, provider, nil
		}
	}
	return "", "", nil, fmt.Errorf("no rates are available for %s/%s", base, quote)
}

// period resolves either a preset range, counted back from the provider's latest day, or an
// explicit from/to pair, which is clamped to the days the provider has rates for.
func period(query url.Values, provider rateProvider) (from, to time.Time, err error) {
	earliest, latest := provider.bounds()

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
		return maxTime(from, earliest), latest, nil
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

	from, to = maxTime(from, earliest), minTime(to, latest)
	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("rates are available from %s to %s",
			earliest.Format(dateLayout), latest.Format(dateLayout))
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

// writeProviderError reports a provider failure: 404 when it has no rate, 502 when it is unreachable.
func writeProviderError(w http.ResponseWriter, provider rateProvider, err error) {
	if errors.Is(err, errNoRate) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("%s has no rate for that day", provider.source().Name))
		return
	}
	log.Printf("%s: %v", provider.source().ID, err)
	writeError(w, http.StatusBadGateway, fmt.Sprintf("%s is unavailable", provider.source().Name))
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
