package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	pogodaBaseURL = "https://pogoda.by/api/v2"
	pogodaTimeout = 5 * time.Second
	// The station list changes rarely.
	pogodaCitiesTTL = 24 * time.Hour
	// After the station list fails to load, wait this long before asking again.
	pogodaRetryAfter = 30 * time.Second
	// How many of the nearest stations to try before giving up on the weather.
	pogodaMaxStations = 5
	// Stations report hourly at most; a short cache spares the API when a page asks for many.
	pogodaFactTTL = 5 * time.Minute
)

var pogodaSource = Source{ID: "pogoda", Name: "Belhydromet"}

var (
	// errNoNearbyWeather means none of the stations nearest a location reported the weather.
	errNoNearbyWeather = errors.New("no weather available near the location")
	// errNoWeather means the station asked for reported no temperature.
	errNoWeather = errors.New("the station reported no temperature")
	// errUnknownStation means the station id is not in the station list.
	errUnknownStation = errors.New("unknown station")
)

// pogodaCity is one station from the all-cities endpoint. Its id is the station argument of the
// weather-fact endpoint.
type pogodaCity struct {
	Country  string   `json:"country"`
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Lat      *float64 `json:"lat"`
	Lng      *float64 `json:"lng"`
	Timezone *float64 `json:"timezone"`
	ICAO     *string  `json:"icao"`
}

// pogodaFact is the latest observation at a station. Fields the station did not report are null.
type pogodaFact struct {
	Date         pogodaTime `json:"d"`
	Temperature  *float64   `json:"t"`
	Humidity     *float64   `json:"w"`
	PStation     *float64   `json:"pStation"`
	PSea         *float64   `json:"pSea"`
	Tendency     *float64   `json:"tend"`
	SpeedWind    *float64   `json:"speedWind"`
	SpeedWindMax *float64   `json:"speedWindMax"`
	Visibility   *float64   `json:"vis_m"`
	DirWind      *float64   `json:"dirWind"`
}

// pogodaTime parses the API's timestamps ("2026-11-07T13:00:00.000Z"); ones without a zone are UTC.
type pogodaTime struct{ time.Time }

func (d *pogodaTime) UnmarshalJSON(data []byte) error {
	var s *string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == nil || *s == "" {
		d.Time = time.Time{}
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, *s); err == nil {
			d.Time = t.UTC()
			return nil
		}
	}
	return fmt.Errorf("unrecognised time %q", *s)
}

// pogodaClient is a thin wrapper over the Belhydromet (pogoda.by) weather API.
type pogodaClient struct {
	baseURL string
	http    *http.Client
}

func newPogodaClient(baseURL string) *pogodaClient {
	return &pogodaClient{baseURL: baseURL, http: &http.Client{Timeout: pogodaTimeout}}
}

// cities returns every station, keyed by country code.
func (c *pogodaClient) cities(ctx context.Context) (map[string][]pogodaCity, error) {
	var byCountry map[string][]pogodaCity
	return byCountry, c.get(ctx, "/weather-fact/all-cities", nil, &byCountry)
}

// fact returns the latest observation at a station.
func (c *pogodaClient) fact(ctx context.Context, station int) (pogodaFact, error) {
	var fact pogodaFact
	return fact, c.get(ctx, "/weather-fact", url.Values{"station": {strconv.Itoa(station)}}, &fact)
}

func (c *pogodaClient) get(ctx context.Context, path string, query url.Values, out any) error {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("pogoda %s: %w", target, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pogoda %s: status %d", target, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("pogoda %s: decode: %w", target, err)
	}
	return nil
}

// weatherStation is a station with known coordinates.
type weatherStation struct {
	ID       int
	Name     string
	Country  string
	Lat, Lng float64
}

// Weather is the latest observation at the station nearest a location that reported one.
type Weather struct {
	Station     weatherStation
	DistanceKm  float64
	MeasuredAt  time.Time
	Temperature float64
	// Nil when the station did not report it.
	Humidity *float64
}

// weatherService finds the nearest working station and reports its latest observation.
type weatherService struct {
	client *pogodaClient
	now    func() time.Time

	// listMu is held while the station list loads, so concurrent requests share one fetch.
	listMu       sync.Mutex
	list         []weatherStation
	listLoadedAt time.Time
	listFailedAt time.Time

	factsMu sync.Mutex
	facts   map[int]cachedFact
}

type cachedFact struct {
	fact      pogodaFact
	fetchedAt time.Time
}

func newWeatherService(client *pogodaClient) *weatherService {
	return &weatherService{client: client, now: time.Now, facts: map[int]cachedFact{}}
}

func (s *weatherService) source() Source { return pogodaSource }

// weather tries the stations nearest lat/lng in order and returns the first one that reports a
// temperature. errNoNearbyWeather means none of them did.
func (s *weatherService) weather(ctx context.Context, lat, lng float64) (Weather, error) {
	stations, err := s.stations(ctx)
	if err != nil {
		return Weather{}, err
	}

	for _, candidate := range nearestStations(stations, lat, lng, pogodaMaxStations) {
		weather, err := s.observation(ctx, candidate.station)
		if err != nil {
			if ctx.Err() != nil {
				return Weather{}, ctx.Err()
			}
			log.Printf("pogoda station %d (%s): %v", candidate.station.ID, candidate.station.Name, err)
			continue
		}
		weather.DistanceKm = candidate.distanceKm
		return weather, nil
	}
	return Weather{}, errNoNearbyWeather
}

// stationsByDistance returns every station, nearest to lat/lng first.
func (s *weatherService) stationsByDistance(ctx context.Context, lat, lng float64) ([]stationDistance, error) {
	stations, err := s.stations(ctx)
	if err != nil {
		return nil, err
	}
	return nearestStations(stations, lat, lng, len(stations)), nil
}

// stationWeather returns the latest observation at one station, with its distance from lat/lng.
// errUnknownStation means there is no such station; errNoWeather that it reported no temperature.
func (s *weatherService) stationWeather(ctx context.Context, id int, lat, lng float64) (Weather, error) {
	stations, err := s.stations(ctx)
	if err != nil {
		return Weather{}, err
	}
	i := slices.IndexFunc(stations, func(station weatherStation) bool { return station.ID == id })
	if i < 0 {
		return Weather{}, errUnknownStation
	}
	weather, err := s.observation(ctx, stations[i])
	if err != nil {
		return Weather{}, err
	}
	weather.DistanceKm = distanceKm(lat, lng, stations[i].Lat, stations[i].Lng)
	return weather, nil
}

// observation returns a station's latest observation, without its distance. Observations are
// cached for a few minutes; errNoWeather means the station reported no temperature.
func (s *weatherService) observation(ctx context.Context, station weatherStation) (Weather, error) {
	s.factsMu.Lock()
	cached, ok := s.facts[station.ID]
	s.factsMu.Unlock()

	fact := cached.fact
	if !ok || s.now().Sub(cached.fetchedAt) >= pogodaFactTTL {
		var err error
		fact, err = s.client.fact(ctx, station.ID)
		if err != nil {
			return Weather{}, err
		}
		s.factsMu.Lock()
		s.facts[station.ID] = cachedFact{fact: fact, fetchedAt: s.now()}
		s.factsMu.Unlock()
	}

	if fact.Temperature == nil || fact.Date.IsZero() {
		return Weather{}, errNoWeather
	}
	return Weather{
		Station:     station,
		MeasuredAt:  fact.Date.Time,
		Temperature: *fact.Temperature,
		Humidity:    fact.Humidity,
	}, nil
}

// stations returns the cached station list, reloading it once a day. A stale list is served when a
// reload fails, and after a failed load the API is left alone for a while.
func (s *weatherService) stations(ctx context.Context) ([]weatherStation, error) {
	s.listMu.Lock()
	defer s.listMu.Unlock()

	now := s.now()
	if s.list != nil && now.Sub(s.listLoadedAt) < pogodaCitiesTTL {
		return s.list, nil
	}
	if now.Sub(s.listFailedAt) < pogodaRetryAfter {
		if s.list != nil {
			return s.list, nil
		}
		return nil, fmt.Errorf("pogoda cities: failed %s ago", now.Sub(s.listFailedAt).Round(time.Second))
	}

	// Load the list even if the request that asked for it goes away: other requests are waiting on it.
	loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), pogodaTimeout)
	defer cancel()
	byCountry, err := s.client.cities(loadCtx)
	if err == nil && len(byCountry) == 0 {
		err = errors.New("pogoda cities: empty list")
	}
	if err != nil {
		s.listFailedAt = now
		if s.list != nil {
			log.Printf("reload pogoda cities, serving the cached list: %v", err)
			return s.list, nil
		}
		return nil, err
	}

	var list []weatherStation
	for country, cities := range byCountry {
		for _, city := range cities {
			if city.Lat == nil || city.Lng == nil {
				continue
			}
			if city.Country != "" {
				country = city.Country
			}
			list = append(list, weatherStation{ID: city.ID, Name: city.Name, Country: country, Lat: *city.Lat, Lng: *city.Lng})
		}
	}
	if len(list) == 0 {
		s.listFailedAt = now
		return nil, errors.New("pogoda cities: no station has coordinates")
	}

	s.list, s.listLoadedAt = list, now
	return list, nil
}

type stationDistance struct {
	station    weatherStation
	distanceKm float64
}

// nearestStations returns up to n stations closest to lat/lng, nearest first.
func nearestStations(stations []weatherStation, lat, lng float64, n int) []stationDistance {
	ranked := make([]stationDistance, len(stations))
	for i, station := range stations {
		ranked[i] = stationDistance{station, distanceKm(lat, lng, station.Lat, station.Lng)}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].distanceKm < ranked[j].distanceKm })
	return ranked[:min(n, len(ranked))]
}

// distanceKm is the great-circle distance between two points, by the haversine formula.
func distanceKm(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371.0
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	dLat, dLng := rad(lat2-lat1), rad(lng2-lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(a)))
}
