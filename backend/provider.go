package main

import (
	"context"
	"errors"
	"time"
)

// Source says where a rate came from, so the frontend can attribute it.
type Source struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// errNoRate means the provider answered but has no rate for the requested day or currency.
var errNoRate = errors.New("no rate published for that day")

// rateProvider supplies exchange rates for the pairs it supports. A rate is how many units of quote
// one unit of base buys. Every error other than errNoRate means the provider could not be reached.
type rateProvider interface {
	source() Source
	supports(base, quote string) bool
	// bounds returns the first and last day the provider has rates for.
	bounds() (earliest, latest time.Time)
	rate(ctx context.Context, base, quote string, day time.Time) (float64, error)
	// history returns the rates from from to to inclusive, oldest first. Days without a rate are
	// left out rather than filled in.
	history(ctx context.Context, base, quote string, from, to time.Time) ([]RatePoint, error)
}

var mockSource = Source{ID: "mock", Name: "Mocked rates"}

// mockProvider serves the generated rateStore series. It supports every pair the store knows.
type mockProvider struct {
	store *rateStore
}

func (p *mockProvider) source() Source { return mockSource }

func (p *mockProvider) supports(base, quote string) bool {
	return p.store.isKnown(base) && p.store.isKnown(quote)
}

func (p *mockProvider) bounds() (time.Time, time.Time) { return p.store.earliest(), p.store.latest() }

func (p *mockProvider) rate(_ context.Context, base, quote string, day time.Time) (float64, error) {
	return p.store.rate(base, quote, day), nil
}

func (p *mockProvider) history(_ context.Context, base, quote string, from, to time.Time) ([]RatePoint, error) {
	return p.store.history(base, quote, from, to), nil
}
