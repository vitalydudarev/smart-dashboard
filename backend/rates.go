package main

import (
	"hash/fnv"
	"math"
	"math/rand/v2"
	"time"
)

const dateLayout = "2006-01-02"

type Currency struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

var currencies = []Currency{
	{Code: "USD", Name: "US Dollar"},
	{Code: "EUR", Name: "Euro"},
	{Code: "GBP", Name: "British Pound"},
	{Code: "JPY", Name: "Japanese Yen"},
	{Code: "CHF", Name: "Swiss Franc"},
	{Code: "CAD", Name: "Canadian Dollar"},
	{Code: "AUD", Name: "Australian Dollar"},
	{Code: "CNY", Name: "Chinese Yuan"},
	{Code: "SEK", Name: "Swedish Krona"},
	{Code: "PLN", Name: "Polish Zloty"},
	// Served by the National Bank of the Republic of Belarus rather than the mock store.
	{Code: "BYN", Name: "Belarusian Ruble"},
}

func isKnownCurrency(code string) bool {
	for _, currency := range currencies {
		if currency.Code == code {
			return true
		}
	}
	return false
}

// Typical units of each currency per US dollar. The mock series wander around these values.
var usdAnchors = map[string]float64{
	"USD": 1,
	"EUR": 0.92,
	"GBP": 0.79,
	"JPY": 150,
	"CHF": 0.88,
	"CAD": 1.37,
	"AUD": 1.52,
	"CNY": 7.2,
	"SEK": 10.6,
	"PLN": 4.0,
}

type RatePoint struct {
	Date string  `json:"date"`
	Rate float64 `json:"rate"`
}

// rateStore holds a mocked daily series of how many units of each currency one US dollar buys.
// Cross rates are derived from those, so every pair stays consistent with every other.
type rateStore struct {
	start  time.Time
	days   int
	perUSD map[string][]float64
}

// newRateStore generates one value per day from start to end inclusive. The series are seeded by
// currency code, so a given day always has the same rate regardless of when the server started.
func newRateStore(start, end time.Time) *rateStore {
	days := daysBetween(start, end) + 1
	store := &rateStore{start: start, days: days, perUSD: make(map[string][]float64, len(usdAnchors))}

	for code, anchor := range usdAnchors {
		store.perUSD[code] = randomWalk(code, anchor, days)
	}

	return store
}

// randomWalk is a mean-reverting walk in log space: daily moves of about 0.5%, pulled slowly back
// towards the anchor so the series stays plausible over a decade.
func randomWalk(code string, anchor float64, days int) []float64 {
	const (
		volatility = 0.005
		reversion  = 0.002
	)

	values := make([]float64, days)
	if code == "USD" {
		for i := range values {
			values[i] = 1
		}
		return values
	}

	hash := fnv.New64a()
	hash.Write([]byte(code))
	rng := rand.New(rand.NewPCG(hash.Sum64(), 0))

	target := math.Log(anchor)
	level := target
	for i := range values {
		level += reversion*(target-level) + volatility*rng.NormFloat64()
		values[i] = math.Exp(level)
	}

	return values
}

func (s *rateStore) earliest() time.Time { return s.start }

func (s *rateStore) latest() time.Time { return s.start.AddDate(0, 0, s.days-1) }

func (s *rateStore) isKnown(code string) bool {
	_, ok := s.perUSD[code]
	return ok
}

// rate returns how many units of quote one unit of base buys on the given day.
func (s *rateStore) rate(base, quote string, day time.Time) float64 {
	i := daysBetween(s.start, day)
	return roundSignificant(s.perUSD[quote][i]/s.perUSD[base][i], 6)
}

// history returns one point per day from from to to inclusive; both must lie within the store.
func (s *rateStore) history(base, quote string, from, to time.Time) []RatePoint {
	points := make([]RatePoint, 0, daysBetween(from, to)+1)
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		points = append(points, RatePoint{Date: day.Format(dateLayout), Rate: s.rate(base, quote, day)})
	}
	return points
}

func daysBetween(from, to time.Time) int {
	return int(to.Sub(from).Hours() / 24)
}

func roundSignificant(value float64, digits int) float64 {
	if value == 0 {
		return 0
	}
	scale := math.Pow(10, float64(digits)-math.Ceil(math.Log10(math.Abs(value))))
	return math.Round(value*scale) / scale
}

func today() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}
