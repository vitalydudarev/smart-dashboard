package main

import (
	"cmp"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
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

// Two days, listed out of order, the second under the first day's key so grouping must use DATES.
const fakeForecast = `{
"Mon Nov 09 2026 00:00:00 GMT+0000 (Coordinated Universal Time)":{
 "12":{"DATES":"2026-11-09T00:00:00.000Z","ADVANCE_TIME":12,"TMP_MIN":-1,"TMP_MAX":2,"TypeWeather":"Ясно"},
 "0":{"DATES":"2026-11-09T00:00:00.000Z","ADVANCE_TIME":0,"TMP_MIN":-3.5,"TMP_MAX":0,"TypeWeather":"Туман"}
},
"Sun Nov 08 2026 00:00:00 GMT+0000 (Coordinated Universal Time)":{
 "0":{"DATES":"2026-11-08T00:00:00.000Z","ADVANCE_TIME":0,"TMP_MIN":2.7,"TMP_MAX":3.3,"TypeWeather":"Облачно"},
 "12":{"DATES":"2026-11-08T00:00:00.000Z","ADVANCE_TIME":12,"TMP_MIN":3.1,"TMP_MAX":5.6,"TypeWeather":"Переменная облачность"}
}
}`

type fakePogoda struct {
	citiesRequests   atomic.Int32
	factRequests     atomic.Int32
	forecastRequests atomic.Int32
	failCities       atomic.Bool
	// Replace the radar and satellite lists when set.
	radars, satellites string
	// Station ids whose weather-fact request fails, or which report no temperature.
	failing, empty map[string]bool
}

func (f *fakePogoda) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if station, ok := strings.CutPrefix(r.URL.Path, "/numeric-weather/12/"); ok {
		f.forecastRequests.Add(1)
		switch {
		case f.failing[station]:
			http.Error(w, "down", http.StatusInternalServerError)
		case f.empty[station]:
			fmt.Fprint(w, `{}`)
		default:
			fmt.Fprint(w, fakeForecast)
		}
		return
	}

	switch r.URL.Path {
	case "/radar/last":
		fmt.Fprint(w, cmp.Or(f.radars, `{"UMII":"Radar_26666_MAP_PHEN_20261108_1120.png","UMMN":"Radar_26850_MAP_PHEN_20261108_1120.png"}`))
	case "/satellite":
		fmt.Fprint(w, cmp.Or(f.satellites, `{"009":["009-202611081100.jpg","small_009-20261108.gif",4.36],"012":["012-202611081100.jpg","small_012-20261108.gif",2.69]}`))
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

func TestStationForecast(t *testing.T) {
	fake := &fakePogoda{
		failing: map[string]bool{"26759": true},
		empty:   map[string]bool{"10384": true},
	}
	handler := newWeatherTestServer(t, fake)

	var got forecastResponse
	for range 2 {
		if code := get(t, handler, "/api/weather/stations/26850/forecast", &got); code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
	}
	if n := fake.forecastRequests.Load(); n != 2 {
		t.Errorf("forecast requested %d times, want 2 (no caching)", n)
	}

	f := func(v float64) *float64 { return &v }
	want := []forecastDayJSON{
		{Date: "2026-11-08", TempMin: f(2.7), TempMax: f(5.6), Description: "Переменная облачность"},
		{Date: "2026-11-09", TempMin: f(-3.5), TempMax: f(2), Description: "Ясно"},
	}
	if len(got.Days) != len(want) {
		t.Fatalf("got %d days, want %d: %+v", len(got.Days), len(want), got.Days)
	}
	for i, day := range got.Days {
		w := want[i]
		if day.Date != w.Date || *day.TempMin != *w.TempMin || *day.TempMax != *w.TempMax || day.Description != w.Description {
			t.Errorf("day %d = %+v (min %v, max %v), want %+v (min %v, max %v)",
				i, day, *day.TempMin, *day.TempMax, w, *w.TempMin, *w.TempMax)
		}
	}

	for target, want := range map[string]int{
		"/api/weather/stations/10384/forecast": http.StatusNotFound,   // empty forecast
		"/api/weather/stations/99/forecast":    http.StatusNotFound,   // does not exist
		"/api/weather/stations/abc/forecast":   http.StatusBadRequest, // not an id
		"/api/weather/stations/26759/forecast": http.StatusBadGateway, // upstream fails
	} {
		if code := get(t, handler, target, nil); code != want {
			t.Errorf("%s: status = %d, want %d", target, code, want)
		}
	}
}

func TestWeatherImages(t *testing.T) {
	handler := newWeatherTestServer(t, &fakePogoda{})
	for target, want := range map[string]string{
		"/api/weather/radar":     "https://pogoda.by/files/radars/static/26850/Radar_26850_MAP_PHEN_20261108_1120.png",
		"/api/weather/satellite": "https://pogoda.by/files/satellite/012-202611081100.jpg",
	} {
		var got weatherImageResponse
		if code := get(t, handler, target, &got); code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", target, code)
		}
		if got.URL != want || got.Source != pogodaSource {
			t.Errorf("%s: got %+v, want url %s", target, got, want)
		}
	}
}

func TestWeatherImagesMissing(t *testing.T) {
	handler := newWeatherTestServer(t, &fakePogoda{
		radars:     `{"UMII":"Radar_26666_MAP_PHEN_20261108_1120.png","UMMN":"../../etc/passwd"}`,
		satellites: `{"009":["009-202611081100.jpg"],"012":["small_012-20261108.gif",2.69]}`,
	})
	for _, target := range []string{"/api/weather/radar", "/api/weather/satellite"} {
		if code := get(t, handler, target, nil); code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", target, code)
		}
	}
}

func TestDistanceKm(t *testing.T) {
	// Minsk to Berlin is about 950 km.
	if d := distanceKm(53.9, 27.56, 52.52, 13.405); d < 940 || d > 960 {
		t.Errorf("distance = %v km", d)
	}
}

func TestForecastDays(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	s := func(v string) *string { return &v }
	step := func(dates string, advance int, lo, hi float64, weather string) pogodaForecastStep {
		var d pogodaTime
		if err := d.UnmarshalJSON([]byte(`"` + dates + `"`)); err != nil {
			t.Fatal(err)
		}
		return pogodaForecastStep{Date: d, AdvanceTime: advance, TempMin: f(lo), TempMax: f(hi), TypeWeather: s(weather)}
	}
	type day struct {
		date        string
		lo, hi      float64
		description string
	}
	check := func(t *testing.T, got []ForecastDay, want []day) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("got %d days, want %d: %+v", len(got), len(want), got)
		}
		for i, w := range want {
			g := got[i]
			if g.Date.Format(dateLayout) != w.date || *g.TempMin != w.lo || *g.TempMax != w.hi || g.Description != w.description {
				t.Errorf("day %d = %s %v..%v %q, want %+v", i, g.Date.Format(dateLayout), *g.TempMin, *g.TempMax, g.Description, w)
			}
		}
	}

	t.Run("one issue time, hours ahead", func(t *testing.T) {
		const issued = "2026-11-08T00:00:00.000Z"
		got := forecastDays(map[string]map[string]pogodaForecastStep{
			"Sun Nov 08 2026 00:00:00 GMT+0000 (Coordinated Universal Time)": {
				"0":  step(issued, 0, 1, 3, "Облачно"),
				"12": step(issued, 12, 2, 5, "Дождь"),
				"24": step(issued, 24, -2, 0, "Туман"),
				"36": step(issued, 36, -1, 4, "Ясно"),
				"48": step(issued, 48, -4, -1, "Снег"),
			},
		})
		check(t, got, []day{
			{"2026-11-08", 1, 5, "Дождь"},
			{"2026-11-09", -2, 4, "Ясно"},
			{"2026-11-10", -4, -1, ""},
		})
	})

	t.Run("one issue time, day keys", func(t *testing.T) {
		const issued = "2026-11-08T00:00:00.000Z"
		got := forecastDays(map[string]map[string]pogodaForecastStep{
			"Sun Nov 08 2026 00:00:00 GMT+0000 (Coordinated Universal Time)": {
				"0":  step(issued, 0, 1, 3, "Облачно"),
				"12": step(issued, 12, 2, 5, "Дождь"),
			},
			"Mon Nov 09 2026 00:00:00 GMT+0000 (Coordinated Universal Time)": {
				"0":  step(issued, 0, -2, 0, "Туман"),
				"12": step(issued, 12, -1, 4, "Ясно"),
			},
			"2026-11-10": {
				"12": step(issued, 12, -4, -1, "Снег"),
			},
		})
		check(t, got, []day{
			{"2026-11-08", 1, 5, "Дождь"},
			{"2026-11-09", -2, 4, "Ясно"},
			{"2026-11-10", -4, -1, "Снег"},
		})
	})
}
