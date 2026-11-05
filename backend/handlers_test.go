package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var testLatest = time.Date(2026, time.November, 5, 0, 0, 0, 0, time.UTC)

func newTestServer() http.Handler {
	return (&server{store: newRateStore(historyStart, testLatest)}).routes()
}

func get(t *testing.T, handler http.Handler, target string, body any) int {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	if body != nil && rec.Code == http.StatusOK {
		if err := json.NewDecoder(rec.Body).Decode(body); err != nil {
			t.Fatalf("decode %s: %v", target, err)
		}
	}
	return rec.Code
}

func TestLatestRate(t *testing.T) {
	var got latestRateResponse
	if code := get(t, newTestServer(), "/api/rates/latest?base=EUR&quote=USD", &got); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got.Base != "EUR" || got.Quote != "USD" || got.Date != "2026-11-05" {
		t.Errorf("got %+v", got)
	}
	if got.Rate < 0.8 || got.Rate > 1.4 {
		t.Errorf("EUR/USD rate %v is implausible", got.Rate)
	}
}

func TestHistoryPresetRanges(t *testing.T) {
	handler := newTestServer()
	tests := []struct {
		preset   string
		wantFrom string
	}{
		{"7d", "2026-10-30"},
		{"30d", "2026-10-07"},
		{"6m", "2026-05-05"},
		{"1y", "2025-11-05"},
		{"5y", "2021-11-05"},
	}

	for _, tt := range tests {
		var got rateHistoryResponse
		if code := get(t, handler, "/api/rates/history?base=GBP&quote=JPY&range="+tt.preset, &got); code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", tt.preset, code)
		}
		first, last := got.Points[0], got.Points[len(got.Points)-1]
		if got.From != tt.wantFrom || first.Date != tt.wantFrom || last.Date != "2026-11-05" {
			t.Errorf("%s: from %s (first point %s), last point %s", tt.preset, got.From, first.Date, last.Date)
		}
	}
}

func TestHistoryCustomRangeIsClamped(t *testing.T) {
	var got rateHistoryResponse
	code := get(t, newTestServer(), "/api/rates/history?base=USD&quote=EUR&from=2015-06-01&to=2030-01-01", &got)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got.From != "2016-01-01" || got.To != "2026-11-05" {
		t.Errorf("clamped to %s..%s", got.From, got.To)
	}
}

func TestCrossRatesAreConsistent(t *testing.T) {
	handler := newTestServer()
	var eurUsd, usdEur latestRateResponse
	get(t, handler, "/api/rates/latest?base=EUR&quote=USD", &eurUsd)
	get(t, handler, "/api/rates/latest?base=USD&quote=EUR", &usdEur)

	if product := eurUsd.Rate * usdEur.Rate; product < 0.9999 || product > 1.0001 {
		t.Errorf("EUR/USD × USD/EUR = %v, want 1", product)
	}
}

func TestBadRequests(t *testing.T) {
	handler := newTestServer()
	for _, target := range []string{
		"/api/rates/latest?base=EUR&quote=XXX",
		"/api/rates/latest?base=EUR&quote=EUR",
		"/api/rates/history?base=EUR&quote=USD&range=2w",
		"/api/rates/history?base=EUR&quote=USD&from=2026-02-01&to=2026-01-01",
		"/api/rates/history?base=EUR&quote=USD&from=2010-01-01&to=2010-12-31",
		"/api/rates/history?base=EUR&quote=USD",
	} {
		if code := get(t, handler, target, nil); code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, code)
		}
	}
}
