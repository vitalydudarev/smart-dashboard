package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Out of kick-off order: Arsenal (featured) at home in the Premier League, a La Liga match without a
// featured club, and a Bundesliga match, which is not shown.
const fakeFixtures = `{"errors":[],"response":[
 {"fixture":{"id":3,"date":"2026-11-24T20:00:00+00:00","status":{"long":"Not Started","short":"NS","elapsed":null,"extra":null},"venue":{"name":"Emirates Stadium","city":"London"}},
  "league":{"id":39,"round":"Regular Season - 13"},
  "teams":{"home":{"id":42,"name":"Arsenal","logo":"a.png"},"away":{"id":49,"name":"Chelsea","logo":"c.png"}},
  "goals":{"home":null,"away":null}},
 {"fixture":{"id":2,"date":"2026-11-24T18:00:00+00:00","status":{"long":"Second Half","short":"2H","elapsed":61,"extra":null},"venue":{"name":null,"city":null}},
  "league":{"id":140,"round":"Regular Season - 14"},
  "teams":{"home":{"id":530,"name":"Atletico Madrid","logo":"am.png"},"away":{"id":536,"name":"Sevilla","logo":"s.png"}},
  "goals":{"home":1,"away":0}},
 {"fixture":{"id":1,"date":"2026-11-24T17:30:00+00:00","status":{"long":"Not Started","short":"NS","elapsed":null,"extra":null},"venue":{"name":"Allianz Arena","city":"München"}},
  "league":{"id":78,"round":"Regular Season - 12"},
  "teams":{"home":{"id":157,"name":"Bayern München","logo":"b.png"},"away":{"id":165,"name":"Dortmund","logo":"d.png"}},
  "goals":{"home":null,"away":null}}
]}`

type fakeFootball struct {
	requests atomic.Int32
	fail     atomic.Bool
	// Answered instead of fakeFixtures when set.
	body string
}

func (f *fakeFootball) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests.Add(1)
	if r.Header.Get("x-apisports-key") != "test-key" {
		http.Error(w, "no key", http.StatusUnauthorized)
		return
	}
	if f.fail.Load() {
		http.Error(w, "down", http.StatusInternalServerError)
		return
	}
	// Only today's request has matches.
	if r.URL.Query().Get("date") != "2026-11-24" {
		w.Write([]byte(`{"errors":[],"response":[]}`))
		return
	}
	if f.body != "" {
		w.Write([]byte(f.body))
		return
	}
	w.Write([]byte(fakeFixtures))
}

func newTestFootball(t *testing.T, fake *fakeFootball, cachePath string) (*footballService, *time.Time) {
	t.Helper()
	upstream := httptest.NewServer(fake)
	t.Cleanup(upstream.Close)
	service := newFootballService(newFootballClient(upstream.URL, "test-key"), cachePath)
	now := time.Date(2026, time.November, 24, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	return service, &now
}

func TestFootballMatchesKeepsShownLeaguesInKickoffOrder(t *testing.T) {
	fake := &fakeFootball{}
	service, _ := newTestFootball(t, fake, "")

	srv := newServer()
	srv.football = service
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/football/matches", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	var body footballMatchesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.From != "2026-11-24" || body.To != "2026-11-25" {
		t.Errorf("days %s to %s, want 2026-11-24 to 2026-11-25", body.From, body.To)
	}
	if len(body.Matches) != 2 || body.Matches[0].ID != 2 || body.Matches[1].ID != 3 {
		t.Fatalf("matches %+v, want ids 2 then 3", body.Matches)
	}
	if body.Matches[0].Featured || !body.Matches[1].Featured {
		t.Errorf("featured %v, %v; want only Arsenal's match", body.Matches[0].Featured, body.Matches[1].Featured)
	}
	if got := body.Matches[1].Venue; got != "Emirates Stadium, London" {
		t.Errorf("venue %q", got)
	}
	if got := body.Matches[0].Status.Elapsed; got == nil || *got != 61 {
		t.Errorf("elapsed %v, want 61", got)
	}
	if got := fake.requests.Load(); got != footballDays {
		t.Errorf("%d upstream requests, want %d", got, footballDays)
	}
}

func TestFootballMatchesAreCachedForAnHour(t *testing.T) {
	fake := &fakeFootball{}
	service, now := newTestFootball(t, fake, "")
	ctx := context.Background()

	for range 3 {
		if _, err := service.matches(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := fake.requests.Load(); got != footballDays {
		t.Fatalf("%d upstream requests, want %d", got, footballDays)
	}

	*now = now.Add(footballTTL)
	if _, err := service.matches(ctx); err != nil {
		t.Fatal(err)
	}
	if got := fake.requests.Load(); got != 2*footballDays {
		t.Errorf("%d upstream requests after an hour, want %d", got, 2*footballDays)
	}
}

func TestFootballMatchesServeStaleOnFailureAndBackOff(t *testing.T) {
	fake := &fakeFootball{}
	service, now := newTestFootball(t, fake, "")
	ctx := context.Background()

	if _, err := service.matches(ctx); err != nil {
		t.Fatal(err)
	}
	fake.fail.Store(true)
	*now = now.Add(footballTTL)

	snapshot, err := service.matches(ctx)
	if err != nil || len(snapshot.Matches) != 2 {
		t.Fatalf("got %d matches, %v; want the cached 2", len(snapshot.Matches), err)
	}
	before := fake.requests.Load()
	*now = now.Add(footballRetryAfter / 2)
	if _, err := service.matches(ctx); err != nil {
		t.Fatal(err)
	}
	if got := fake.requests.Load(); got != before {
		t.Errorf("asked again %d times while backing off", got-before)
	}
}

func TestFootballMatchesReportAPIErrors(t *testing.T) {
	fake := &fakeFootball{body: `{"errors":{"requests":"You have reached the request limit for the day"},"response":[]}`}
	service, _ := newTestFootball(t, fake, "")

	if _, err := service.matches(context.Background()); err == nil {
		t.Fatal("want an error when the API reports one")
	}
}

func TestFootballMatchesSurviveARestart(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "football.json")
	fake := &fakeFootball{}
	service, _ := newTestFootball(t, fake, cachePath)
	if _, err := service.matches(context.Background()); err != nil {
		t.Fatal(err)
	}

	restarted, _ := newTestFootball(t, fake, cachePath)
	snapshot, err := restarted.matches(context.Background())
	if err != nil || len(snapshot.Matches) != 2 {
		t.Fatalf("got %d matches, %v; want the saved 2", len(snapshot.Matches), err)
	}
	if got := fake.requests.Load(); got != footballDays {
		t.Errorf("%d upstream requests, want only the first service's %d", got, footballDays)
	}
}

func TestFootballMatchesUnconfigured(t *testing.T) {
	rec := httptest.NewRecorder()
	newServer().routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/football/matches", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503", rec.Code)
	}
}

func TestFootballRefreshIgnoresACancelledRequest(t *testing.T) {
	fake := &fakeFootball{}
	service, _ := newTestFootball(t, fake, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	snapshot, err := service.matches(ctx)
	if err != nil || len(snapshot.Matches) != 2 {
		t.Fatalf("got %d matches, %v; want 2", len(snapshot.Matches), err)
	}
}

func TestFootballMatchesRefetchWhenTheLeaguesChange(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "football.json")
	fake := &fakeFootball{}
	service, _ := newTestFootball(t, fake, cachePath)
	if _, err := service.matches(context.Background()); err != nil {
		t.Fatal(err)
	}
	// As saved before the Champions League was added.
	service.snapshot.LeagueIDs = []int{39, 140, 135}
	service.save(*service.snapshot)

	restarted, _ := newTestFootball(t, fake, cachePath)
	if _, err := restarted.matches(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := fake.requests.Load(); got != 2*footballDays {
		t.Errorf("%d upstream requests, want %d: the saved snapshot covers other leagues", got, 2*footballDays)
	}
}
