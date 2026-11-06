package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	nbrbBaseURL = "https://api.nbrb.by/exrates"
	// The API is not always reachable; better to show nothing quickly than to hang the page.
	nbrbTimeout = 5 * time.Second
	// The currency list changes a few times a year at most.
	nbrbCurrenciesTTL = 24 * time.Hour
	// After the currency list fails to load, wait this long before asking again.
	nbrbRetryAfter = 30 * time.Second
	// The dynamics endpoint returns at most this many days per request.
	nbrbMaxDynamicsDays = 365

	nbrbDaily = 0

	// How the rates endpoint reads its cur_id argument (parammode).
	nbrbByInternalID = 0
	nbrbByLetterCode = 2

	byn = "BYN"
)

var nbrbSource = Source{ID: "nbrb", Name: "National Bank of the Republic of Belarus"}

// BYN replaced BYR at 10,000:1 on 2016-07-01; the API quotes earlier days in old rubles.
var nbrbEarliest = time.Date(2016, time.July, 1, 0, 0, 0, 0, time.UTC)

// The National Bank sets rates per calendar day in Minsk, which does not observe daylight saving.
var minsk = time.FixedZone("Europe/Minsk", 3*60*60)

// nbrbDate parses the API's zone-less timestamps ("2025-10-10T00:00:00") as UTC days.
type nbrbDate struct{ time.Time }

func (d *nbrbDate) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		d.Time = time.Time{}
		return nil
	}
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

// nbrbCurrency is the API's Currency class. A currency gets a new Cur_ID whenever its code, scale or
// name changes; Cur_ParentID ties those records together.
type nbrbCurrency struct {
	ID           int      `json:"Cur_ID"`
	ParentID     int      `json:"Cur_ParentID"`
	Code         string   `json:"Cur_Code"`
	Abbreviation string   `json:"Cur_Abbreviation"`
	Name         string   `json:"Cur_Name"`
	NameBel      string   `json:"Cur_Name_Bel"`
	NameEng      string   `json:"Cur_Name_Eng"`
	QuotName     string   `json:"Cur_QuotName"`
	QuotNameBel  string   `json:"Cur_QuotName_Bel"`
	QuotNameEng  string   `json:"Cur_QuotName_Eng"`
	NameMulti    string   `json:"Cur_NameMulti"`
	NameBelMulti string   `json:"Cur_Name_BelMulti"`
	NameEngMulti string   `json:"Cur_Name_EngMulti"`
	Scale        int      `json:"Cur_Scale"`
	Periodicity  int      `json:"Cur_Periodicity"`
	DateStart    nbrbDate `json:"Cur_DateStart"`
	DateEnd      nbrbDate `json:"Cur_DateEnd"`
}

// nbrbRate is the API's Rate class: OfficialRate rubles buy Scale units of the currency.
type nbrbRate struct {
	ID           int      `json:"Cur_ID"`
	Date         nbrbDate `json:"Date"`
	Code         string   `json:"Cur_Code"`
	Abbreviation string   `json:"Cur_Abbreviation"`
	Scale        int      `json:"Cur_Scale"`
	Name         string   `json:"Cur_Name"`
	OfficialRate float64  `json:"Cur_OfficialRate"`
}

// nbrbRateShort is the API's RateShort class, returned by the dynamics endpoint.
type nbrbRateShort struct {
	ID           int      `json:"Cur_ID"`
	Date         nbrbDate `json:"Date"`
	OfficialRate float64  `json:"Cur_OfficialRate"`
}

// nbrbClient is a thin wrapper over the National Bank of the Republic of Belarus exchange rate API.
type nbrbClient struct {
	baseURL string
	http    *http.Client
}

func newNBRBClient(baseURL string) *nbrbClient {
	return &nbrbClient{baseURL: baseURL, http: &http.Client{Timeout: nbrbTimeout}}
}

// currencies returns every currency the National Bank has ever set a ruble rate for.
func (c *nbrbClient) currencies(ctx context.Context) ([]nbrbCurrency, error) {
	var list []nbrbCurrency
	return list, c.get(ctx, "/currencies", nil, &list)
}

// currency returns one currency by its internal Cur_ID.
func (c *nbrbClient) currency(ctx context.Context, id int) (nbrbCurrency, error) {
	var currency nbrbCurrency
	return currency, c.get(ctx, "/currencies/"+strconv.Itoa(id), nil, &currency)
}

// rates returns the official rates of every currency with the given periodicity on a day.
func (c *nbrbClient) rates(ctx context.Context, day time.Time, periodicity int) ([]nbrbRate, error) {
	query := url.Values{"ondate": {day.Format(dateLayout)}, "periodicity": {strconv.Itoa(periodicity)}}
	var list []nbrbRate
	return list, c.get(ctx, "/rates", query, &list)
}

// rate returns one currency's official rate on a day. id is read according to paramMode: an
// internal Cur_ID, or an ISO 4217 code as it was on that day.
func (c *nbrbClient) rate(ctx context.Context, id string, paramMode int, day time.Time) (nbrbRate, error) {
	query := url.Values{"parammode": {strconv.Itoa(paramMode)}, "ondate": {day.Format(dateLayout)}}
	var rate nbrbRate
	return rate, c.get(ctx, "/rates/"+url.PathEscape(id), query, &rate)
}

// dynamics returns a currency's daily rates from from to to inclusive, at most 365 days apart.
func (c *nbrbClient) dynamics(ctx context.Context, id int, from, to time.Time) ([]nbrbRateShort, error) {
	query := url.Values{"startdate": {from.Format(dateLayout)}, "enddate": {to.Format(dateLayout)}}
	var list []nbrbRateShort
	return list, c.get(ctx, "/rates/dynamics/"+strconv.Itoa(id), query, &list)
}

func (c *nbrbClient) get(ctx context.Context, path string, query url.Values, out any) error {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("nbrb %s: %w", path, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errNoRate
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("nbrb %s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("nbrb %s: decode: %w", path, err)
	}
	return nil
}

// cachedDay is one fetched day of a currency. ok is false when the API had no rate for it, so the
// day is not asked for again.
type cachedDay struct {
	official float64
	ok       bool
}

// nbrbProvider serves pairs of BYN and another currency from the National Bank's official rates.
// Published rates never change, so every day fetched is kept for the life of the process.
type nbrbProvider struct {
	client *nbrbClient
	now    func() time.Time

	// listMu is held while the currency list loads, so concurrent requests share one fetch.
	listMu       sync.Mutex
	list         []nbrbCurrency
	listLoadedAt time.Time
	listFailedAt time.Time

	mu   sync.Mutex
	days map[int]map[time.Time]cachedDay // Cur_ID → day → rate
}

func newNBRBProvider(client *nbrbClient) *nbrbProvider {
	return &nbrbProvider{client: client, now: time.Now, days: make(map[int]map[time.Time]cachedDay)}
}

func (p *nbrbProvider) source() Source { return nbrbSource }

func (p *nbrbProvider) supports(base, quote string) bool {
	return (base == byn) != (quote == byn)
}

func (p *nbrbProvider) bounds() (time.Time, time.Time) { return nbrbEarliest, p.today() }

func (p *nbrbProvider) today() time.Time {
	now := p.now().In(minsk)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func (p *nbrbProvider) rate(ctx context.Context, base, quote string, day time.Time) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, nbrbTimeout)
	defer cancel()
	code, inverse := foreignSide(base, quote)

	list, err := p.currencies(ctx)
	if err != nil {
		// Without the list there is no Cur_ID to cache under, but the rate can still be asked for by
		// its ISO code.
		rate, err := p.client.rate(ctx, code, nbrbByLetterCode, day)
		if err != nil {
			return 0, err
		}
		p.store(rate.ID, day, rate.OfficialRate)
		return toPairRate(rate.OfficialRate, rate.Scale, inverse), nil
	}

	currency, ok := findCurrency(list, code, day)
	if !ok {
		return 0, errNoRate
	}
	if cached, hit := p.cached(currency.ID, day); hit {
		if !cached.ok {
			return 0, errNoRate
		}
		return toPairRate(cached.official, currency.Scale, inverse), nil
	}

	rate, err := p.client.rate(ctx, strconv.Itoa(currency.ID), nbrbByInternalID, day)
	if err != nil {
		return 0, err
	}
	p.store(currency.ID, day, rate.OfficialRate)
	return toPairRate(rate.OfficialRate, rate.Scale, inverse), nil
}

func (p *nbrbProvider) history(ctx context.Context, base, quote string, from, to time.Time) ([]RatePoint, error) {
	ctx, cancel := context.WithTimeout(ctx, nbrbTimeout)
	defer cancel()
	code, inverse := foreignSide(base, quote)

	list, err := p.currencies(ctx)
	if err != nil {
		return nil, err
	}

	var points []RatePoint
	// A long period can span several records of the same currency, e.g. after a change of scale.
	for _, span := range currencySpans(list, code, from, to) {
		if err := p.fill(ctx, span.currency.ID, span.from, span.to); err != nil {
			return nil, err
		}

		p.mu.Lock()
		cache := p.days[span.currency.ID]
		for day := span.from; !day.After(span.to); day = day.AddDate(0, 0, 1) {
			if cached := cache[day]; cached.ok {
				points = append(points, RatePoint{
					Date: day.Format(dateLayout),
					Rate: toPairRate(cached.official, span.currency.Scale, inverse),
				})
			}
		}
		p.mu.Unlock()
	}

	if len(points) == 0 {
		return nil, errNoRate
	}
	return points, nil
}

// fill fetches whichever days from from to to are not cached yet, in as few requests as it can.
func (p *nbrbProvider) fill(ctx context.Context, id int, from, to time.Time) error {
	first, last, missing := p.missingDays(id, from, to)
	if !missing {
		return nil
	}

	for start := first; !start.After(last); start = start.AddDate(0, 0, nbrbMaxDynamicsDays) {
		end := minTime(start.AddDate(0, 0, nbrbMaxDynamicsDays-1), last)
		rows, err := p.client.dynamics(ctx, id, start, end)
		if err != nil {
			return err
		}

		today := p.today()
		p.mu.Lock()
		cache := p.dayCache(id)
		// Remember past days the API had nothing for; today's rate may simply not be set yet.
		for day := start; !day.After(end) && day.Before(today); day = day.AddDate(0, 0, 1) {
			if _, ok := cache[day]; !ok {
				cache[day] = cachedDay{}
			}
		}
		for _, row := range rows {
			cache[row.Date.Time] = cachedDay{official: row.OfficialRate, ok: true}
		}
		p.mu.Unlock()
	}

	return nil
}

func (p *nbrbProvider) missingDays(id int, from, to time.Time) (first, last time.Time, missing bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	cache := p.days[id]
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		if _, ok := cache[day]; !ok {
			if !missing {
				first, missing = day, true
			}
			last = day
		}
	}
	return first, last, missing
}

func (p *nbrbProvider) cached(id int, day time.Time) (cachedDay, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cached, ok := p.days[id][day]
	return cached, ok
}

func (p *nbrbProvider) store(id int, day time.Time, official float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dayCache(id)[day] = cachedDay{official: official, ok: true}
}

// dayCache returns the cache for one Cur_ID, creating it. The caller holds mu.
func (p *nbrbProvider) dayCache(id int) map[time.Time]cachedDay {
	cache, ok := p.days[id]
	if !ok {
		cache = make(map[time.Time]cachedDay)
		p.days[id] = cache
	}
	return cache
}

// currencies returns the cached currency list, reloading it once a day. A stale list is served
// when a reload fails, and after a failed load the API is left alone for a while.
func (p *nbrbProvider) currencies(ctx context.Context) ([]nbrbCurrency, error) {
	p.listMu.Lock()
	defer p.listMu.Unlock()

	now := p.now()
	if p.list != nil && now.Sub(p.listLoadedAt) < nbrbCurrenciesTTL {
		return p.list, nil
	}
	if now.Sub(p.listFailedAt) < nbrbRetryAfter {
		if p.list != nil {
			return p.list, nil
		}
		return nil, fmt.Errorf("nbrb currencies: failed %s ago", now.Sub(p.listFailedAt).Round(time.Second))
	}

	// Load the list even if the request that asked for it goes away: other requests are waiting on it,
	// and a cancelled load would otherwise count as a failure.
	loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), nbrbTimeout)
	defer cancel()
	list, err := p.client.currencies(loadCtx)
	if err != nil {
		p.listFailedAt = now
		if p.list != nil {
			log.Printf("reload NBRB currencies, serving the cached list: %v", err)
			return p.list, nil
		}
		return nil, err
	}

	p.list, p.listLoadedAt = list, now
	return list, nil
}

type currencySpan struct {
	currency nbrbCurrency
	from, to time.Time
}

// currencySpans splits from–to into the parts covered by each daily-rate record of a currency.
func currencySpans(list []nbrbCurrency, code string, from, to time.Time) []currencySpan {
	var spans []currencySpan
	for _, currency := range list {
		if currency.Abbreviation != code || currency.Periodicity != nbrbDaily {
			continue
		}
		start, end := maxTime(from, currency.DateStart.Time), minTime(to, currency.DateEnd.Time)
		if !start.After(end) {
			spans = append(spans, currencySpan{currency: currency, from: start, to: end})
		}
	}

	// Records come in Cur_ID order, which is not necessarily date order.
	sort.Slice(spans, func(i, j int) bool { return spans[i].from.Before(spans[j].from) })
	return spans
}

// findCurrency returns the daily-rate record of a currency that was in effect on a day.
func findCurrency(list []nbrbCurrency, code string, day time.Time) (nbrbCurrency, bool) {
	spans := currencySpans(list, code, day, day)
	if len(spans) == 0 {
		return nbrbCurrency{}, false
	}
	return spans[0].currency, true
}

// foreignSide returns the non-BYN currency of a pair, and whether BYN is the base, which inverts the
// National Bank's quotation of rubles per foreign unit.
func foreignSide(base, quote string) (code string, inverse bool) {
	if base == byn {
		return quote, true
	}
	return base, false
}

func toPairRate(official float64, scale int, inverse bool) float64 {
	perUnit := official / float64(max(scale, 1))
	if inverse {
		return roundSignificant(1/perUnit, 6)
	}
	return roundSignificant(perUnit, 6)
}
