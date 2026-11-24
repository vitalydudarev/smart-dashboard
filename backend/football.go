package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

const (
	footballBaseURL = "https://v3.football.api-sports.io"
	footballTimeout = 10 * time.Second
	// The free plan allows 100 requests a day; each refresh costs one per day fetched, so at most 48 a day.
	footballTTL = time.Hour
	// After a failed refresh, keep serving what we have and wait this long before asking again.
	footballRetryAfter = 10 * time.Minute
	// The free plan only answers for yesterday to tomorrow, so today and tomorrow are all the upcoming
	// matches it can give.
	footballDays = 2
)

var footballSource = Source{ID: "api-football", Name: "API-Football"}

// errFootballNotConfigured means the backend has no API-Football key.
var errFootballNotConfigured = errors.New("football is not configured")

// footballLeague is a league whose matches are shown, in display order.
type footballLeague struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Logo    string `json:"logo"`
}

var footballLeagues = []footballLeague{
	{ID: 39, Name: "Premier League", Country: "England", Logo: "https://media.api-sports.io/football/leagues/39.png"},
	{ID: 140, Name: "La Liga", Country: "Spain", Logo: "https://media.api-sports.io/football/leagues/140.png"},
	{ID: 135, Name: "Serie A", Country: "Italy", Logo: "https://media.api-sports.io/football/leagues/135.png"},
}

// The clubs whose matches are featured on the dashboard, by API-Football team id.
var featuredTeams = map[int]string{
	541: "Real Madrid",
	42:  "Arsenal",
	40:  "Liverpool",
	50:  "Manchester City",
	33:  "Manchester United",
	505: "Inter",
	489: "AC Milan",
}

func isShownLeague(id int) bool {
	return slices.ContainsFunc(footballLeagues, func(l footballLeague) bool { return l.ID == id })
}

// apiFixture is one entry of the fixtures endpoint's response, with only the fields we use.
type apiFixture struct {
	Fixture struct {
		ID     int       `json:"id"`
		Date   time.Time `json:"date"`
		Status struct {
			Long    string `json:"long"`
			Short   string `json:"short"`
			Elapsed *int   `json:"elapsed"`
			Extra   *int   `json:"extra"`
		} `json:"status"`
		Venue struct {
			Name *string `json:"name"`
			City *string `json:"city"`
		} `json:"venue"`
	} `json:"fixture"`
	League struct {
		ID    int    `json:"id"`
		Round string `json:"round"`
	} `json:"league"`
	Teams struct {
		Home apiTeam `json:"home"`
		Away apiTeam `json:"away"`
	} `json:"teams"`
	Goals struct {
		Home *int `json:"home"`
		Away *int `json:"away"`
	} `json:"goals"`
}

type apiTeam struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Logo string `json:"logo"`
}

// apiResponse is the envelope every API-Football endpoint answers with. Errors come back with status
// 200, as an object (or an empty array when there are none).
type apiResponse[T any] struct {
	Errors   json.RawMessage `json:"errors"`
	Response T               `json:"response"`
}

// footballClient is a thin wrapper over API-Football v3.
type footballClient struct {
	baseURL string
	key     string
	http    *http.Client
}

func newFootballClient(baseURL, key string) *footballClient {
	return &footballClient{baseURL: baseURL, key: key, http: &http.Client{Timeout: footballTimeout}}
}

// fixturesOn returns every fixture, of every league, on a UTC day.
func (c *footballClient) fixturesOn(ctx context.Context, day time.Time) ([]apiFixture, error) {
	target := c.baseURL + "/fixtures?" + url.Values{"date": {day.Format(dateLayout)}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-apisports-key", c.key)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api-football %s: %w", target, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api-football %s: status %d", target, resp.StatusCode)
	}
	var body apiResponse[[]apiFixture]
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("api-football %s: decode: %w", target, err)
	}
	if apiErrors := string(body.Errors); apiErrors != "" && apiErrors != "[]" && apiErrors != "{}" && apiErrors != "null" {
		return nil, fmt.Errorf("api-football %s: %s", target, apiErrors)
	}
	return body.Response, nil
}

// footballMatch is a match as the API returns it to the frontend.
type footballMatch struct {
	ID       int       `json:"id"`
	LeagueID int       `json:"leagueId"`
	Round    string    `json:"round"`
	Kickoff  time.Time `json:"kickoff"`
	Status   struct {
		// API-Football's short code, e.g. NS, 1H, HT, FT.
		Short string `json:"short"`
		Long  string `json:"long"`
		// Minutes played; null before kick-off.
		Elapsed *int `json:"elapsed"`
		Extra   *int `json:"extra"`
	} `json:"status"`
	Home footballSide `json:"home"`
	Away footballSide `json:"away"`
	// Empty when unknown.
	Venue string `json:"venue"`
	// Whether one of the featured clubs plays.
	Featured bool `json:"featured"`
}

type footballSide struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Logo string `json:"logo"`
	// Null before kick-off.
	Goals *int `json:"goals"`
}

func newFootballMatch(f apiFixture) footballMatch {
	m := footballMatch{
		ID:       f.Fixture.ID,
		LeagueID: f.League.ID,
		Round:    f.League.Round,
		Kickoff:  f.Fixture.Date.UTC(),
		Home:     footballSide{ID: f.Teams.Home.ID, Name: f.Teams.Home.Name, Logo: f.Teams.Home.Logo, Goals: f.Goals.Home},
		Away:     footballSide{ID: f.Teams.Away.ID, Name: f.Teams.Away.Name, Logo: f.Teams.Away.Logo, Goals: f.Goals.Away},
	}
	m.Status.Short = f.Fixture.Status.Short
	m.Status.Long = f.Fixture.Status.Long
	m.Status.Elapsed = f.Fixture.Status.Elapsed
	m.Status.Extra = f.Fixture.Status.Extra
	if f.Fixture.Venue.Name != nil {
		m.Venue = *f.Fixture.Venue.Name
		if f.Fixture.Venue.City != nil && *f.Fixture.Venue.City != "" {
			m.Venue += ", " + *f.Fixture.Venue.City
		}
	}
	_, home := featuredTeams[m.Home.ID]
	_, away := featuredTeams[m.Away.ID]
	m.Featured = home || away
	return m
}

// footballSnapshot is the shown leagues' matches as of one refresh. It is also what is saved to disk.
type footballSnapshot struct {
	FetchedAt time.Time `json:"fetchedAt"`
	// The UTC days covered, as YYYY-MM-DD.
	From    string          `json:"from"`
	To      string          `json:"to"`
	Matches []footballMatch `json:"matches"`
}

// footballService caches the matches for an hour, in memory and in a file so restarting the backend
// does not spend the day's request allowance again.
type footballService struct {
	client    *footballClient
	cachePath string
	now       func() time.Time

	mu        sync.Mutex
	snapshot  *footballSnapshot
	lastError time.Time
}

func newFootballService(client *footballClient, cachePath string) *footballService {
	s := &footballService{client: client, cachePath: cachePath, now: time.Now}
	if cachePath != "" {
		if data, err := os.ReadFile(cachePath); err == nil {
			var snapshot footballSnapshot
			if err := json.Unmarshal(data, &snapshot); err == nil {
				s.snapshot = &snapshot
			}
		}
	}
	return s
}

// defaultFootballCachePath is a file in the user's cache directory, or "" when there is none.
func defaultFootballCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "smart-dashboard", "football.json")
}

// matches returns the cached snapshot, refreshing it when it is an hour old or from an earlier day.
// When a refresh fails, the previous snapshot is served if there is one.
func (s *footballService) matches(ctx context.Context) (footballSnapshot, error) {
	if s == nil || s.client == nil || s.client.key == "" {
		return footballSnapshot{}, errFootballNotConfigured
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	today := now.Format(dateLayout)
	fresh := s.snapshot != nil && s.snapshot.From == today && now.Sub(s.snapshot.FetchedAt) < footballTTL
	backingOff := now.Sub(s.lastError) < footballRetryAfter
	if fresh || (backingOff && s.snapshot != nil) {
		return *s.snapshot, nil
	}
	if backingOff {
		return footballSnapshot{}, errors.New("api-football failed recently; waiting before trying again")
	}

	// The refresh serves every visitor, so one closing their tab must not cancel it (and start a back-off).
	snapshot, err := s.fetch(context.WithoutCancel(ctx), now)
	if err != nil {
		s.lastError = now
		if s.snapshot != nil {
			log.Printf("refresh football matches, serving the cached ones: %v", err)
			return *s.snapshot, nil
		}
		return footballSnapshot{}, err
	}
	s.snapshot = &snapshot
	s.save(snapshot)
	return snapshot, nil
}

func (s *footballService) fetch(ctx context.Context, now time.Time) (footballSnapshot, error) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	snapshot := footballSnapshot{
		FetchedAt: now,
		From:      start.Format(dateLayout),
		To:        start.AddDate(0, 0, footballDays-1).Format(dateLayout),
		Matches:   []footballMatch{},
	}
	for i := range footballDays {
		fixtures, err := s.client.fixturesOn(ctx, start.AddDate(0, 0, i))
		if err != nil {
			return footballSnapshot{}, err
		}
		for _, f := range fixtures {
			if isShownLeague(f.League.ID) {
				snapshot.Matches = append(snapshot.Matches, newFootballMatch(f))
			}
		}
	}
	slices.SortStableFunc(snapshot.Matches, func(a, b footballMatch) int { return a.Kickoff.Compare(b.Kickoff) })
	return snapshot, nil
}

func (s *footballService) save(snapshot footballSnapshot) {
	if s.cachePath == "" {
		return
	}
	data, err := json.Marshal(snapshot)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(s.cachePath), 0o755)
	}
	if err == nil {
		err = os.WriteFile(s.cachePath, data, 0o644)
	}
	if err != nil {
		log.Printf("save football matches: %v", err)
	}
}
