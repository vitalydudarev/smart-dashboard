package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// Minsk and Barysaŭ are close to each other; Berlin is far from both. Uzda has no coordinates.
const fakeCities = `{
"BY":[
 {"country":"BY","id":26850,"name":"Минск","lat":53.93,"lng":27.63,"timezone":3,"icao":"UMMS"},
 {"country":"BY","id":26759,"name":"Борисов","lat":54.23,"lng":28.5,"timezone":3,"icao":null},
 {"country":"BY","id":1,"name":"Узда","lat":null,"lng":null,"timezone":3,"icao":null}
],
"DE":[{"country":"DE","id":10384,"name":"Берлин","lat":52.468,"lng":13.402,"timezone":2,"icao":"EDDB"}]
}`

type fakePogoda struct {
	citiesRequests atomic.Int32
	factRequests   atomic.Int32
	failCities     atomic.Bool
	// Station ids whose weather-fact request fails, or which report no temperature.
	failing, empty map[string]bool
}

func (f *fakePogoda) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/weather-fact/all-cities":
		f.citiesRequests.Add(1)
		if f.failCities.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, fakeCities)
	case "/weather-fact":
		f.factRequests.Add(1)
		station := r.URL.Query().Get("station")
		switch {
		case f.failing[station]:
			http.Error(w, "down", http.StatusInternalServerError)
		case f.empty[station]:
			fmt.Fprint(w, `{"d":"2026-11-07T13:00:00.000Z","t":null,"w":null}`)
		default:
			fmt.Fprintf(w, `{"d":"2026-11-07T13:00:00.000Z","t":3.8,"w":71,"pStation":998.8,"pSea":1026.5,"tend":2,"code":null,"speedWind":2,"speedWindMax":4.4,"vis_m":20000,"dirWind":275,"station":%s}`, station)
		}
	default:
		http.NotFound(w, r)
	}
}

func newWeatherTestServer(t *testing.T, fake *fakePogoda) http.Handler {
	t.Helper()
	upstream := httptest.NewServer(fake)
	t.Cleanup(upstream.Close)

	srv := newServer()
	srv.weather = newWeatherService(newPogodaClient(upstream.URL))
	return srv.routes()
}

func TestWeatherFromNearestStation(t *testing.T) {
	handler := newWeatherTestServer(t, &fakePogoda{})

	var got weatherResponse
	if code := get(t, handler, "/api/weather?lat=53.9&lng=27.56", &got); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got.Location.ID != 26850 || got.Location.Name != "Минск" || got.Location.Country != "BY" {
		t.Errorf("location = %+v, want Minsk", got.Location)
	}
	if got.Temperature != 3.8 || got.Humidity == nil || *got.Humidity != 71 {
		t.Errorf("temperature = %v, humidity = %v", got.Temperature, got.Humidity)
	}
	if want := time.Date(2026, time.November, 7, 13, 0, 0, 0, time.UTC); !got.MeasuredAt.Equal(want) {
		t.Errorf("measuredAt = %v, want %v", got.MeasuredAt, want)
	}
	if got.Source != pogodaSource {
		t.Errorf("source = %+v", got.Source)
	}
}

func TestWeatherDefaultsToMinsk(t *testing.T) {
	var got weatherResponse
	if code := get(t, newWeatherTestServer(t, &fakePogoda{}), "/api/weather", &got); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got.Location.ID != 26850 {
		t.Errorf("location = %+v, want Minsk", got.Location)
	}
}

func TestWeatherFallsBackToNextNearestStation(t *testing.T) {
	handler := newWeatherTestServer(t, &fakePogoda{
		failing: map[string]bool{"26850": true},
		empty:   map[string]bool{"26759": true},
	})

	var got weatherResponse
	if code := get(t, handler, "/api/weather?lat=53.9&lng=27.56", &got); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got.Location.ID != 10384 {
		t.Errorf("location = %+v, want Berlin after Minsk failed and Barysaŭ had no data", got.Location)
	}
}

func TestWeatherUnavailable(t *testing.T) {
	all := map[string]bool{"26850": true, "26759": true, "10384": true}
	if code := get(t, newWeatherTestServer(t, &fakePogoda{failing: all}), "/api/weather", nil); code != http.StatusBadGateway {
		t.Errorf("all stations failing: status = %d, want 502", code)
	}

	fake := &fakePogoda{}
	fake.failCities.Store(true)
	handler := newWeatherTestServer(t, fake)
	for range 2 {
		if code := get(t, handler, "/api/weather", nil); code != http.StatusBadGateway {
			t.Errorf("cities failing: status = %d, want 502", code)
		}
	}
	if n := fake.citiesRequests.Load(); n != 1 {
		t.Errorf("cities requested %d times, want 1 while backing off", n)
	}
}

func TestWeatherCachesStations(t *testing.T) {
	fake := &fakePogoda{}
	handler := newWeatherTestServer(t, fake)
	for range 3 {
		get(t, handler, "/api/weather", nil)
	}
	if n := fake.citiesRequests.Load(); n != 1 {
		t.Errorf("cities requested %d times, want 1", n)
	}
}

func TestWeatherBadCoordinates(t *testing.T) {
	handler := newWeatherTestServer(t, &fakePogoda{})
	for _, target := range []string{"/api/weather?lat=91&lng=0", "/api/weather?lat=abc&lng=1", "/api/weather?lat=50"} {
		if code := get(t, handler, target, nil); code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, code)
		}
	}
}

func TestWeatherStationsByDistance(t *testing.T) {
	handler := newWeatherTestServer(t, &fakePogoda{})

	// From Berlin, Berlin comes first and Barysaŭ, east of Minsk, last; Uzda has no coordinates.
	var got weatherStationsResponse
	if code := get(t, handler, "/api/weather/stations?lat=52.52&lng=13.405", &got); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	var ids []int
	for _, station := range got.Stations {
		ids = append(ids, station.ID)
	}
	if want := []int{10384, 26850, 26759}; !slices.Equal(ids, want) {
		t.Errorf("station ids = %v, want %v", ids, want)
	}
	if got.Stations[0].DistanceKm > 10 || got.Stations[1].DistanceKm < 900 {
		t.Errorf("distances = %+v", got.Stations)
	}
	if got.Source != pogodaSource {
		t.Errorf("source = %+v", got.Source)
	}

	if code := get(t, handler, "/api/weather/stations?lat=abc&lng=1", nil); code != http.StatusBadRequest {
		t.Errorf("bad coordinates: status = %d, want 400", code)
	}
}

func TestStationWeather(t *testing.T) {
	fake := &fakePogoda{
		failing: map[string]bool{"26759": true},
		empty:   map[string]bool{"10384": true},
	}
	handler := newWeatherTestServer(t, fake)

	var got weatherResponse
	if code := get(t, handler, "/api/weather/stations/26850?lat=52.52&lng=13.405", &got); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got.Location.ID != 26850 || got.Temperature != 3.8 || got.Location.DistanceKm < 900 {
		t.Errorf("got %+v, want Minsk about 950 km from Berlin", got)
	}

	for target, want := range map[string]int{
		"/api/weather/stations/10384": http.StatusNotFound,   // reports no temperature
		"/api/weather/stations/1":     http.StatusNotFound,   // has no coordinates
		"/api/weather/stations/99":    http.StatusNotFound,   // does not exist
		"/api/weather/stations/abc":   http.StatusBadRequest, // not an id
		"/api/weather/stations/26759": http.StatusBadGateway, // upstream fails
	} {
		if code := get(t, handler, target, nil); code != want {
			t.Errorf("%s: status = %d, want %d", target, code, want)
		}
	}
}

func TestStationWeatherCachesObservations(t *testing.T) {
	fake := &fakePogoda{}
	handler := newWeatherTestServer(t, fake)
	for range 3 {
		get(t, handler, "/api/weather/stations/26850", nil)
	}
	get(t, handler, "/api/weather", nil)
	if n := fake.factRequests.Load(); n != 1 {
		t.Errorf("weather fact requested %d times, want 1", n)
	}
}

func TestDistanceKm(t *testing.T) {
	// Minsk to Berlin is about 950 km.
	if d := distanceKm(53.9, 27.56, 52.52, 13.405); d < 940 || d > 960 {
		t.Errorf("distance = %v km", d)
	}
}
