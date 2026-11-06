package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The currency records the fake API knows: USD changed Cur_ID in July 2021, JPY is quoted per 100.
const fakeCurrencies = `[
{"Cur_ID":145,"Cur_ParentID":145,"Cur_Code":"840","Cur_Abbreviation":"USD","Cur_Name_Eng":"US Dollar","Cur_Scale":1,"Cur_Periodicity":0,"Cur_DateStart":"1991-01-01T00:00:00","Cur_DateEnd":"2021-07-08T00:00:00"},
{"Cur_ID":431,"Cur_ParentID":145,"Cur_Code":"840","Cur_Abbreviation":"USD","Cur_Name_Eng":"US Dollar","Cur_Scale":1,"Cur_Periodicity":0,"Cur_DateStart":"2021-07-09T00:00:00","Cur_DateEnd":"2050-01-01T00:00:00"},
{"Cur_ID":508,"Cur_ParentID":508,"Cur_Code":"392","Cur_Abbreviation":"JPY","Cur_Name_Eng":"Japanese Yen","Cur_Scale":100,"Cur_Periodicity":0,"Cur_DateStart":"2021-07-09T00:00:00","Cur_DateEnd":"2050-01-01T00:00:00"},
{"Cur_ID":1,"Cur_ParentID":1,"Cur_Code":"008","Cur_Abbreviation":"ALL","Cur_Name_Eng":"Albanian Lek","Cur_Scale":1,"Cur_Periodicity":1,"Cur_DateStart":"1991-01-01T00:00:00","Cur_DateEnd":"2007-11-30T00:00:00"}
]`

// fakeRate is the official rate the fake API reports for a Cur_ID on a day.
func fakeRate(id int, day time.Time) float64 {
	return float64(id) + float64(day.YearDay())/1000
}

type fakeNBRB struct {
	requests       atomic.Int32
	failCurrencies atomic.Bool
	down           atomic.Bool
}

func (f *fakeNBRB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests.Add(1)
	if f.down.Load() {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return
	}
	query := r.URL.Query()

	switch {
	case r.URL.Path == "/currencies":
		if f.failCurrencies.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, fakeCurrencies)

	case strings.HasPrefix(r.URL.Path, "/rates/dynamics/"):
		var id int
		fmt.Sscan(strings.TrimPrefix(r.URL.Path, "/rates/dynamics/"), &id)
		from, _ := time.Parse(dateLayout, query.Get("startdate"))
		to, _ := time.Parse(dateLayout, query.Get("enddate"))
		if to.Sub(from) >= nbrbMaxDynamicsDays*24*time.Hour {
			http.Error(w, "too long", http.StatusBadRequest)
			return
		}
		var rows []string
		for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
			rows = append(rows, fmt.Sprintf(`{"Cur_ID":%d,"Date":"%sT00:00:00","Cur_OfficialRate":%v}`,
				id, day.Format(dateLayout), fakeRate(id, day)))
		}
		fmt.Fprintf(w, "[%s]", strings.Join(rows, ","))

	case strings.HasPrefix(r.URL.Path, "/rates/"):
		id := strings.TrimPrefix(r.URL.Path, "/rates/")
		day, _ := time.Parse(dateLayout, query.Get("ondate"))
		curID, scale := 0, 1
		switch {
		case id == "431" || (query.Get("parammode") == "2" && id == "USD"):
			curID = 431
		case id == "508":
			curID, scale = 508, 100
		default:
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"Cur_ID":%d,"Date":"%sT00:00:00","Cur_Scale":%d,"Cur_OfficialRate":%v}`,
			curID, day.Format(dateLayout), scale, fakeRate(curID, day))

	default:
		http.NotFound(w, r)
	}
}

func newNBRBTestServer(t *testing.T) (http.Handler, *fakeNBRB) {
	t.Helper()
	fake := &fakeNBRB{}
	upstream := httptest.NewServer(fake)
	t.Cleanup(upstream.Close)

	nbrb := newNBRBProvider(newNBRBClient(upstream.URL))
	// 2026-11-05 22:30 UTC is already the 6th in Minsk.
	nbrb.now = func() time.Time { return time.Date(2026, time.November, 5, 22, 30, 0, 0, time.UTC) }

	return newServer(nbrb, &mockProvider{store: newRateStore(historyStart, testLatest)}).routes(), fake
}

func TestNBRBRateOnDate(t *testing.T) {
	handler, _ := newNBRBTestServer(t)
	day := time.Date(2025, time.October, 10, 0, 0, 0, 0, time.UTC)

	var got latestRateResponse
	if code := get(t, handler, "/api/rates/latest?base=USD&quote=BYN&date=2025-10-10", &got); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got.Rate != roundSignificant(fakeRate(431, day), 6) || got.Date != "2025-10-10" || got.Source != nbrbSource {
		t.Errorf("got %+v", got)
	}

	// BYN as the base inverts the quotation, and JPY is quoted per 100 yen.
	get(t, handler, "/api/rates/latest?base=BYN&quote=JPY&date=2025-10-10", &got)
	if want := roundSignificant(100/fakeRate(508, day), 6); got.Rate != want {
		t.Errorf("BYN/JPY = %v, want %v", got.Rate, want)
	}
}

func TestNBRBLatestIsTodayInMinsk(t *testing.T) {
	handler, _ := newNBRBTestServer(t)
	var got latestRateResponse
	get(t, handler, "/api/rates/latest?base=USD&quote=BYN", &got)
	if got.Date != "2026-11-06" {
		t.Errorf("date = %s, want 2026-11-06", got.Date)
	}
}

func TestNBRBHistorySpansCurrencyRecordsAndChunks(t *testing.T) {
	handler, _ := newNBRBTestServer(t)

	var got rateHistoryResponse
	code := get(t, handler, "/api/rates/history?base=USD&quote=BYN&from=2020-01-01&to=2022-12-31", &got)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(got.Points) != 1096 {
		t.Fatalf("got %d points, want 1096", len(got.Points))
	}
	// The last day of the old record, then the first of the new one.
	for i, want := range map[int]float64{
		554: fakeRate(145, time.Date(2021, time.July, 8, 0, 0, 0, 0, time.UTC)),
		555: fakeRate(431, time.Date(2021, time.July, 9, 0, 0, 0, 0, time.UTC)),
	} {
		if got.Points[i].Rate != roundSignificant(want, 6) {
			t.Errorf("point %d (%s) = %v, want %v", i, got.Points[i].Date, got.Points[i].Rate, want)
		}
	}
}

func TestNBRBHistoryIsCachedAndReusedForRates(t *testing.T) {
	handler, fake := newNBRBTestServer(t)

	get(t, handler, "/api/rates/history?base=USD&quote=BYN&range=30d", nil)
	afterFirst := fake.requests.Load() // the currency list and one dynamics call
	if afterFirst != 2 {
		t.Errorf("first history made %d requests, want 2", afterFirst)
	}

	var history rateHistoryResponse
	get(t, handler, "/api/rates/history?base=BYN&quote=USD&range=7d", &history)
	var rate latestRateResponse
	get(t, handler, "/api/rates/latest?base=USD&quote=BYN&date=2026-11-01", &rate)
	if n := fake.requests.Load(); n != afterFirst {
		t.Errorf("cached requests reached the API %d more times", n-afterFirst)
	}
	if want := roundSignificant(fakeRate(431, time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)), 6); rate.Rate != want {
		t.Errorf("cached rate = %v, want %v", rate.Rate, want)
	}
}

func TestNBRBRateFallsBackToISOCode(t *testing.T) {
	handler, fake := newNBRBTestServer(t)
	fake.failCurrencies.Store(true)

	var got latestRateResponse
	if code := get(t, handler, "/api/rates/latest?base=USD&quote=BYN", &got); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if code := get(t, handler, "/api/rates/history?base=USD&quote=BYN&range=7d", nil); code != http.StatusBadGateway {
		t.Errorf("history without currencies: status = %d, want 502", code)
	}
}

func TestNBRBUnavailable(t *testing.T) {
	handler, fake := newNBRBTestServer(t)
	fake.down.Store(true)

	for _, target := range []string{
		"/api/rates/latest?base=USD&quote=BYN",
		"/api/rates/history?base=BYN&quote=USD&range=30d",
	} {
		if code := get(t, handler, target, nil); code != http.StatusBadGateway {
			t.Errorf("%s: status = %d, want 502", target, code)
		}
	}
	// Pairs without BYN keep using the mock.
	if code := get(t, handler, "/api/rates/latest?base=EUR&quote=USD", nil); code != http.StatusOK {
		t.Errorf("EUR/USD: status = %d, want 200", code)
	}
}

func TestNBRBTimeout(t *testing.T) {
	hang := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-hang }))
	t.Cleanup(func() { close(hang); upstream.Close() })

	client := newNBRBClient(upstream.URL)
	client.http.Timeout = 50 * time.Millisecond
	handler := newServer(newNBRBProvider(client)).routes()

	start := time.Now()
	if code := get(t, handler, "/api/rates/latest?base=USD&quote=BYN", nil); code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", code)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("took %v", elapsed)
	}
}

func TestNBRBRejectsDaysBeforeRedenomination(t *testing.T) {
	handler, _ := newNBRBTestServer(t)
	if code := get(t, handler, "/api/rates/latest?base=USD&quote=BYN&date=2016-06-30", nil); code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", code)
	}
}
