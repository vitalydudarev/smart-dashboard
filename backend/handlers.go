package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type server struct {
	// The first provider that supports a pair serves it.
	providers []rateProvider
	// Serves /api/weather; nil leaves weather unavailable.
	weather *weatherService
}

func newServer(providers ...rateProvider) *server {
	return &server{providers: providers}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/currencies", s.handleCurrencies)
	mux.HandleFunc("GET /api/rates/latest", s.handleRate)
	mux.HandleFunc("GET /api/rates/history", s.handleRateHistory)
	mux.HandleFunc("GET /api/weather", s.handleWeather)
	mux.HandleFunc("GET /api/weather/stations", s.handleWeatherStations)
	mux.HandleFunc("GET /api/weather/stations/{id}", s.handleStationWeather)
	mux.HandleFunc("GET /api/weather/stations/{id}/forecast", s.handleStationForecast)
	mux.HandleFunc("GET /api/weather/map", s.handleWeatherMap)
	mux.HandleFunc("GET /api/weather/radar", s.handleWeatherImage((*weatherService).radarImage))
	mux.HandleFunc("GET /api/weather/satellite", s.handleWeatherImage((*weatherService).satelliteImage))
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

type weatherResponse struct {
	Location weatherLocation `json:"location"`
	// Degrees Celsius.
	Temperature float64 `json:"temperature"`
	// Relative humidity in percent; null when the station did not report it.
	Humidity *float64 `json:"humidity"`
	// When the station took the measurement, in RFC 3339.
	MeasuredAt time.Time `json:"measuredAt"`
	Source     Source    `json:"source"`
}

type weatherLocation struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Country    string  `json:"country"`
	DistanceKm float64 `json:"distanceKm"`
}

type weatherStationsResponse struct {
	// Every station, nearest to the requested coordinates first.
	Stations []weatherLocation `json:"stations"`
	Source   Source            `json:"source"`
}

type weatherMapResponse struct {
	Country string `json:"country"`
	// Every station in the country, by name.
	Stations []weatherMapStation `json:"stations"`
	Source   Source              `json:"source"`
}

// weatherMapStation is a station's position and latest observation; the observation fields are null
// when it reported none.
type weatherMapStation struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Lat         float64    `json:"lat"`
	Lng         float64    `json:"lng"`
	Temperature *float64   `json:"temperature"`
	Humidity    *float64   `json:"humidity"`
	MeasuredAt  *time.Time `json:"measuredAt"`
}

// The country whose stations the weather map shows when none is asked for.
const defaultMapCountry = "BY"

type forecastResponse struct {
	StationID int               `json:"stationId"`
	Days      []forecastDayJSON `json:"days"`
	Source    Source            `json:"source"`
}

type forecastDayJSON struct {
	// The day, as YYYY-MM-DD.
	Date string `json:"date"`
	// Degrees Celsius; null when the forecast has none.
	TempMin *float64 `json:"tempMin"`
	TempMax *float64 `json:"tempMax"`
	// The midday weather, e.g. "Облачно"; empty when unknown.
	Description string `json:"description"`
}

type weatherImageResponse struct {
	// The image itself, on pogoda.by.
	URL string `json:"url"`
	// When the image was taken, from its file name; null when the name has no time.
	TakenAt *time.Time `json:"takenAt"`
	Source  Source     `json:"source"`
}

// Used when the client does not send its location.
const defaultLat, defaultLng = 53.9045, 27.5615 // Minsk

func (s *server) handleCurrencies(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, currencies)
}

// GET /api/rates/latest?base=EUR&quote=USD (today, or yesterday when today has no rate yet)
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
	// Without a date, today's rate may not be published yet; fall back to the day before.
	if errors.Is(err, errNoRate) && query.Get("date") == "" && day.After(earliest) {
		day = day.AddDate(0, 0, -1)
		rate, err = provider.rate(r.Context(), base, quote, day)
	}
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

// GET /api/weather?lat=53.9&lng=27.56 (Minsk when lat/lng are left out)
func (s *server) handleWeather(w http.ResponseWriter, r *http.Request) {
	lat, lng, ok := s.weatherParams(w, r)
	if !ok {
		return
	}

	weather, err := s.weather.weather(r.Context(), lat, lng)
	if err != nil {
		s.writeWeatherError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.weatherResponse(weather))
}

// GET /api/weather/stations?lat=53.9&lng=27.56 (distances from Minsk when lat/lng are left out)
func (s *server) handleWeatherStations(w http.ResponseWriter, r *http.Request) {
	lat, lng, ok := s.weatherParams(w, r)
	if !ok {
		return
	}

	ranked, err := s.weather.stationsByDistance(r.Context(), lat, lng)
	if err != nil {
		s.writeWeatherError(w, err)
		return
	}

	stations := make([]weatherLocation, len(ranked))
	for i, candidate := range ranked {
		stations[i] = newWeatherLocation(candidate.station, candidate.distanceKm)
	}
	writeJSON(w, http.StatusOK, weatherStationsResponse{Stations: stations, Source: s.weather.source()})
}

// GET /api/weather/map?country=BY: every station in a country with its position and latest
// observation, for a map. The country defaults to Belarus.
func (s *server) handleWeatherMap(w http.ResponseWriter, r *http.Request) {
	country := strings.ToUpper(cmp.Or(r.URL.Query().Get("country"), defaultMapCountry))
	if len(country) != 2 {
		writeError(w, http.StatusBadRequest, "country must be a two-letter code")
		return
	}
	if s.weather == nil {
		writeError(w, http.StatusServiceUnavailable, "weather is not configured")
		return
	}

	readings, err := s.weather.countryWeather(r.Context(), country)
	if err != nil {
		s.writeWeatherError(w, err)
		return
	}
	stations := make([]weatherMapStation, len(readings))
	for i, reading := range readings {
		station := weatherMapStation{ID: reading.Station.ID, Name: reading.Station.Name, Lat: reading.Station.Lat, Lng: reading.Station.Lng}
		if weather := reading.Weather; weather != nil {
			station.Temperature = &weather.Temperature
			station.Humidity = weather.Humidity
			station.MeasuredAt = &weather.MeasuredAt
		}
		stations[i] = station
	}
	writeJSON(w, http.StatusOK, weatherMapResponse{Country: country, Stations: stations, Source: s.weather.source()})
}

// GET /api/weather/stations/26850?lat=53.9&lng=27.56 (distance from Minsk when lat/lng are left out)
func (s *server) handleStationWeather(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "station id must be a number")
		return
	}
	lat, lng, ok := s.weatherParams(w, r)
	if !ok {
		return
	}

	weather, err := s.weather.stationWeather(r.Context(), id, lat, lng)
	if err != nil {
		s.writeWeatherError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.weatherResponse(weather))
}

// GET /api/weather/stations/26850/forecast
func (s *server) handleStationForecast(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "station id must be a number")
		return
	}
	if _, _, ok := s.weatherParams(w, r); !ok {
		return
	}

	days, err := s.weather.forecast(r.Context(), id)
	if err != nil {
		s.writeWeatherError(w, err)
		return
	}

	body := forecastResponse{StationID: id, Days: make([]forecastDayJSON, len(days)), Source: s.weather.source()}
	for i, day := range days {
		body.Days[i] = forecastDayJSON{
			Date:        day.Date.Format(dateLayout),
			TempMin:     day.TempMin,
			TempMax:     day.TempMax,
			Description: day.Description,
		}
	}
	// Forecasts are revised during the day; always ask again.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, body)
}

// GET /api/weather/radar and GET /api/weather/satellite: the URL of the latest radar map or
// satellite image, and when it was taken.
func (s *server) handleWeatherImage(latest func(*weatherService, context.Context) (weatherImage, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.weather == nil {
			writeError(w, http.StatusServiceUnavailable, "weather is not configured")
			return
		}
		image, err := latest(s.weather, r.Context())
		if err != nil {
			s.writeWeatherError(w, err)
			return
		}
		// The latest image changes every few minutes; always ask again.
		w.Header().Set("Cache-Control", "no-store")
		body := weatherImageResponse{URL: image.URL, Source: s.weather.source()}
		if !image.TakenAt.IsZero() {
			body.TakenAt = &image.TakenAt
		}
		writeJSON(w, http.StatusOK, body)
	}
}

// weatherParams reads the optional lat/lng, defaulting to Minsk. It writes the error response and
// returns false when they are invalid or weather is not configured.
func (s *server) weatherParams(w http.ResponseWriter, r *http.Request) (lat, lng float64, ok bool) {
	query := r.URL.Query()

	lat, lng = defaultLat, defaultLng
	if query.Has("lat") || query.Has("lng") {
		var errLat, errLng error
		lat, errLat = strconv.ParseFloat(query.Get("lat"), 64)
		lng, errLng = strconv.ParseFloat(query.Get("lng"), 64)
		if errLat != nil || errLng != nil || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			writeError(w, http.StatusBadRequest, "lat and lng must be coordinates in degrees")
			return 0, 0, false
		}
	}

	if s.weather == nil {
		writeError(w, http.StatusServiceUnavailable, "weather is not configured")
		return 0, 0, false
	}
	return lat, lng, true
}

// writeWeatherError reports a weather failure: 404 for an unknown station or one with no current
// observation, 502 when the source is unreachable.
func (s *server) writeWeatherError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errUnknownStation):
		writeError(w, http.StatusNotFound, "no such weather station")
	case errors.Is(err, errNoWeather):
		writeError(w, http.StatusNotFound, "the station has no current observation")
	case errors.Is(err, errNoForecast):
		writeError(w, http.StatusNotFound, "the station has no forecast")
	case errors.Is(err, errNoImage):
		log.Printf("%s: %v", s.weather.source().ID, err)
		writeError(w, http.StatusNotFound, "the image is not available")
	default:
		log.Printf("%s: %v", s.weather.source().ID, err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("%s is unavailable", s.weather.source().Name))
	}
}

func (s *server) weatherResponse(weather Weather) weatherResponse {
	return weatherResponse{
		Location:    newWeatherLocation(weather.Station, weather.DistanceKm),
		Temperature: weather.Temperature,
		Humidity:    weather.Humidity,
		MeasuredAt:  weather.MeasuredAt,
		Source:      s.weather.source(),
	}
}

func newWeatherLocation(station weatherStation, distance float64) weatherLocation {
	return weatherLocation{
		ID:         station.ID,
		Name:       station.Name,
		Country:    station.Country,
		DistanceKm: math.Round(distance*10) / 10,
	}
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
